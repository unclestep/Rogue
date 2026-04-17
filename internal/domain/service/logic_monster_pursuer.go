package service

import (
	"github.com/unclestep/Rogue/internal/domain/model"
)

// PursuerBehavior drives the Pursuer monster. It observes the world through a
// LOS-limited view of the player, keeps per-monster memory of the last known
// position, and delegates the final action to a Policy (ONNX / scripted / fallback).
//
// Unlike WanderBehavior and ChaseBehavior it does not use a scent gradient
// directly — the scent map is provided to the FallbackPolicy, which is used
// when no trained model is loaded.
type PursuerBehavior struct {
	raycaster *Raycaster
	policy    Policy
	memory    map[model.ActorId]*PursuerMemory
}

func NewPursuerBehavior(raycaster *Raycaster, policy Policy) *PursuerBehavior {
	return &PursuerBehavior{
		raycaster: raycaster,
		policy:    policy,
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
	action := pb.policy.Predict(ctx, actor, obs)

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
