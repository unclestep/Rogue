package service

import (
	"math"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

// Director/Alien two-tier AI thresholds.
const (
	rlTriggerRadius    = 8  // Chebyshev cells to player → activate RL
	rlTriggerScent     = 0.25 // ChScent value in crop window → activate RL
	rlFrustrationLimit = 15   // turns in RL mode without engagement → hand back to Director
)

// PursuerBehavior drives the Pursuer monster via a Two-Tier Director/Alien
// pattern:
//
//   - Macro level (Director): when the monster hasn't sensed the player for
//     rlFrustrationLimit turns it falls back to a hardcoded Dijkstra wander.
//   - Micro level (RL): as soon as the player enters rlTriggerRadius cells or
//     the scent gradient exceeds rlTriggerScent, the ONNX policy takes over.
type PursuerBehavior struct {
	raycaster *Raycaster
	policy    Policy // RL policy (ONNX / scripted / fallback-as-configured)
	fallback  Policy // Director-mode fallback — always Dijkstra wander/chase
	memory    map[model.ActorId]*PursuerMemory
}

func NewPursuerBehavior(raycaster *Raycaster, pathfinder *Pathfinder, policy Policy) *PursuerBehavior {
	return &PursuerBehavior{
		raycaster: raycaster,
		policy:    policy,
		fallback:  NewFallbackPolicy(pathfinder),
		memory:    make(map[model.ActorId]*PursuerMemory),
	}
}

// MemorySize exposes the number of Pursuers currently tracked. Useful for
// monitoring the lazy-prune strategy and for tests.
func (pb *PursuerBehavior) MemorySize() int { return len(pb.memory) }

func (pb *PursuerBehavior) Update(ctx *model.SessionContext, actor *model.Actor) (model.BehaviorType, *MonsterDecision) {
	pb.pruneDeadMemory(ctx)

	mem, ok := pb.memory[actor.Id]
	if !ok {
		mem = NewPursuerMemory()
		pb.memory[actor.Id] = mem
	}
	pb.updateMemory(ctx, actor, mem)

	obs := BuildObservation(ctx, actor, mem)

	// Director: check whether RL engagement conditions are met.
	if !mem.RLActive && pb.rlTrigger(ctx, actor) {
		mem.RLActive = true
		mem.FrustrationCount = 0
	}

	var action int
	if mem.RLActive {
		action = pb.policy.Predict(ctx, actor, obs)
		if pb.seesAnyPlayer(ctx, actor) {
			mem.FrustrationCount = 0
		} else {
			mem.FrustrationCount++
		}
		if mem.FrustrationCount >= rlFrustrationLimit {
			mem.RLActive = false
		}
	} else {
		action = pb.fallback.Predict(ctx, actor, obs)
		mem.FrustrationCount = 0
	}

	return model.BehaviorPursuer, &MonsterDecision{MoveVector: ActionToVector(action)}
}

// pruneDeadMemory drops memory for monsters that have been killed or removed
// from the active roster. Runs every Update call but is cheap (Monsters is
// small). Matches the lazy cleanup strategy described in PR 2.
func (pb *PursuerBehavior) pruneDeadMemory(ctx *model.SessionContext) {
	for id := range pb.memory {
		if _, alive := ctx.Playthrough.Monsters[id]; !alive {
			delete(pb.memory, id)
		}
	}
}

func (pb *PursuerBehavior) updateMemory(ctx *model.SessionContext, actor *model.Actor, mem *PursuerMemory) {
	m := ctx.Playthrough.Map
	if m == nil {
		return
	}

	mem.Trail = append(mem.Trail, actor.Pos)
	if len(mem.Trail) > PursuerTrailCapacity {
		mem.Trail = mem.Trail[len(mem.Trail)-PursuerTrailCapacity:]
	}

	player, _, have := nearestAlivePlayer(ctx.Playthrough, actor.Pos)
	if have && hasLOS(m, actor.Pos, player.Pos) {
		mem.LastSeen = player.Pos
		mem.TurnsSinceLOS = 0
		return
	}
	if mem.TurnsSinceLOS < PursuerMemoryHorizon {
		mem.TurnsSinceLOS++
	}
}

// rlTrigger returns true when engagement conditions are met:
//   - any alive player is within rlTriggerRadius Chebyshev cells, or
//   - the max scent value in the 11×11 observation window exceeds rlTriggerScent.
func (pb *PursuerBehavior) rlTrigger(ctx *model.SessionContext, actor *model.Actor) bool {
	play := ctx.Playthrough
	m := play.Map
	if m == nil {
		return false
	}
	for _, p := range play.Players {
		if p.Vitals[model.VitalHP] <= 0 || p.Pos == model.NewInvalidPoint() {
			continue
		}
		dx := actor.Pos.X - p.Pos.X
		if dx < 0 {
			dx = -dx
		}
		dy := actor.Pos.Y - p.Pos.Y
		if dy < 0 {
			dy = -dy
		}
		if max(dx, dy) <= rlTriggerRadius {
			return true
		}
	}
	sm := ctx.ScentMaps[model.ScentMapChase]
	if sm == nil {
		return false
	}
	for dy := -ObservationHalfCrop; dy <= ObservationHalfCrop; dy++ {
		for dx := -ObservationHalfCrop; dx <= ObservationHalfCrop; dx++ {
			p := geometry.Point{X: actor.Pos.X + dx, Y: actor.Pos.Y + dy}
			if !m.InBounds(p) {
				continue
			}
			dist := sm.Scent[p.Y][p.X]
			if dist == math.MaxInt {
				continue
			}
			v := 1.0 - float64(dist)/ScentNormFactor
			if v > rlTriggerScent {
				return true
			}
		}
	}
	return false
}

// seesAnyPlayer returns true if the actor has LOS to at least one alive player.
func (pb *PursuerBehavior) seesAnyPlayer(ctx *model.SessionContext, actor *model.Actor) bool {
	m := ctx.Playthrough.Map
	if m == nil {
		return false
	}
	for _, p := range ctx.Playthrough.Players {
		if p.Vitals[model.VitalHP] <= 0 || p.Pos == model.NewInvalidPoint() {
			continue
		}
		if hasLOS(m, actor.Pos, p.Pos) {
			return true
		}
	}
	return false
}
