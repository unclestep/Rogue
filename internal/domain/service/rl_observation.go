package service

import (
	"math"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

// Observation layout.
//
// The vector is laid out as [channels... | scalars]:
//
//	channels: ObservationChannels × ObservationCropSize × ObservationCropSize
//	          flattened in CHW row-major order
//	          (i.e. obs[c*H*W + y*W + x])
//	scalars : ObservationScalars floats appended after the channel block
//
// Keep this layout in lock-step with the Python Gymnasium env
// (rl/pursuer_env.py). Any reordering or resizing breaks parity.
const (
	ObservationCropSize   = 11
	ObservationHalfCrop   = ObservationCropSize / 2
	ObservationChannels   = 9
	ObservationScalars    = 12
	ObservationGridFloats = ObservationChannels * ObservationCropSize * ObservationCropSize // 9*121=1089
	ObservationSize       = ObservationGridFloats + ObservationScalars                     // 1101

	ChWalkable     = 0
	ChClosedDoor   = 1
	ChOtherMonster = 2
	ChSelfTrail    = 3
	ChPlayerVisNow = 4
	ChPlayerMemory = 5
	ChPlayerCone   = 6
	ChScent        = 7 // Dijkstra chase-scent gradient: hot near player, fades with distance
	ChAgeLastSeen  = 8 // Gaussian-spread decay around last-known player position

	// ScentNormFactor is the BFS-distance scale for ChScent normalisation.
	ScentNormFactor = 30.0
	// AgeLastSeenSigma is the Gaussian spread (cells) for ChAgeLastSeen.
	AgeLastSeenSigma = 2.5

	// PursuerTrailCapacity bounds the self-trail channel window.
	// Oldest entry has weight 0, newest has weight 1.
	PursuerTrailCapacity = 20

	// PursuerMemoryHorizon bounds the memory decay channel. Past this many
	// turns without LOS the chPlayerMemory channel reads 0.
	PursuerMemoryHorizon = 20
)

// PursuerMemory is per-Pursuer state preserved across turns. The map that
// owns it lives inside PursuerBehavior; PursuerMemory itself is a leaf value
// struct. Exported so the observation layout can be verified from tests and
// from the Python Gymnasium env.
type PursuerMemory struct {
	LastSeen      geometry.Point   // NewInvalidPoint when the player has never been spotted
	TurnsSinceLOS int              // Capped at PursuerMemoryHorizon for stability
	Trail         []geometry.Point

	// Director/Alien two-tier state (not part of the ONNX observation).
	FrustrationCount int  // turns in RL mode without LOS or hot scent
	RLActive         bool // true = ONNX policy controls; false = macro/Director
}

func NewPursuerMemory() *PursuerMemory {
	return &PursuerMemory{
		LastSeen:      model.NewInvalidPoint(),
		TurnsSinceLOS: PursuerMemoryHorizon,
		Trail:         make([]geometry.Point, 0, PursuerTrailCapacity),
	}
}

// BuildObservation assembles a flat float32 tensor for the Pursuer policy.
// All spatial features are relative to the Pursuer's current position. Scalars
// are normalised into [-1, 1] where reasonable. The tensor always has length
// ObservationSize; cells outside the map are left at 0 (treated as walls).
// Pure function — all state is read from ctx and mem; nothing is mutated.
func BuildObservation(ctx *model.SessionContext, actor *model.Actor, mem *PursuerMemory) []float32 {
	obs := make([]float32, ObservationSize)
	play := ctx.Playthrough
	m := play.Map
	if m == nil {
		return obs
	}

	player, playerAngle, haveTarget := nearestAlivePlayer(play, actor.Pos)

	playerCone := playerConeArea(play, player, haveTarget)
	seesPlayer := haveTarget && hasLOS(m, actor.Pos, player.Pos)

	writeChannels(obs, m, actor, mem, play, player, playerCone, haveTarget, seesPlayer)
	writeChScent(obs, ctx.ScentMaps[model.ScentMapChase], actor.Pos, m)
	writeChAgeLastSeen(obs, mem, actor.Pos)
	writeScalars(obs, m, actor, mem, player, playerAngle, haveTarget, seesPlayer, play)
	return obs
}

func writeChannels(
	obs []float32,
	m *model.Map,
	actor *model.Actor,
	mem *PursuerMemory,
	play *model.Playthrough,
	player *model.Actor,
	playerCone *model.VisibleArea,
	haveTarget bool,
	seesPlayer bool,
) {
	origin := actor.Pos
	for dy := -ObservationHalfCrop; dy <= ObservationHalfCrop; dy++ {
		for dx := -ObservationHalfCrop; dx <= ObservationHalfCrop; dx++ {
			p := geometry.Point{X: origin.X + dx, Y: origin.Y + dy}
			x := dx + ObservationHalfCrop
			y := dy + ObservationHalfCrop

			if !m.InBounds(p) {
				continue
			}

			if m.IsWalkable(p) {
				setChannel(obs, ChWalkable, x, y, 1.0)
			}
			if m.IsClosedDoor(p) {
				setChannel(obs, ChClosedDoor, x, y, 1.0)
			}
			if id, _ := m.GetActorID(p); id != 0 && model.ActorId(id) != actor.Id {
				if _, isMonster := play.Monsters[model.ActorId(id)]; isMonster {
					setChannel(obs, ChOtherMonster, x, y, 1.0)
				}
			}
			if haveTarget && p == player.Pos && seesPlayer {
				setChannel(obs, ChPlayerVisNow, x, y, 1.0)
			}
			if playerCone != nil && playerCone.Area[p.Y][p.X] == model.Visible {
				setChannel(obs, ChPlayerCone, x, y, 1.0)
			}
		}
	}

	// ChSelfTrail — fresh positions dominate; oldest has weight ~1/cap.
	for i, tp := range mem.Trail {
		age := len(mem.Trail) - 1 - i
		weight := 1.0 - float32(age)/float32(PursuerTrailCapacity)
		if weight <= 0 {
			continue
		}
		dx := tp.X - origin.X
		dy := tp.Y - origin.Y
		if dx < -ObservationHalfCrop || dx > ObservationHalfCrop ||
			dy < -ObservationHalfCrop || dy > ObservationHalfCrop {
			continue
		}
		setChannel(obs, ChSelfTrail, dx+ObservationHalfCrop, dy+ObservationHalfCrop, weight)
	}

	// ChPlayerMemory — single spike at LastSeen, amplitude decays with TurnsSinceLOS.
	if mem.LastSeen != model.NewInvalidPoint() {
		dx := mem.LastSeen.X - origin.X
		dy := mem.LastSeen.Y - origin.Y
		if dx >= -ObservationHalfCrop && dx <= ObservationHalfCrop &&
			dy >= -ObservationHalfCrop && dy <= ObservationHalfCrop {
			weight := 1.0 - float32(mem.TurnsSinceLOS)/float32(PursuerMemoryHorizon)
			if weight > 0 {
				setChannel(obs, ChPlayerMemory, dx+ObservationHalfCrop, dy+ObservationHalfCrop, weight)
			}
		}
	}
}

// writeChScent writes the normalised Dijkstra chase-scent gradient into ChScent.
// Hot (≈1.0) near the player, fading to 0 at ScentNormFactor BFS steps away.
// Unreachable cells are left at 0.
func writeChScent(obs []float32, scentMap *model.ScentMap, origin geometry.Point, m *model.Map) {
	if scentMap == nil {
		return
	}
	for dy := -ObservationHalfCrop; dy <= ObservationHalfCrop; dy++ {
		for dx := -ObservationHalfCrop; dx <= ObservationHalfCrop; dx++ {
			p := geometry.Point{X: origin.X + dx, Y: origin.Y + dy}
			if !m.InBounds(p) {
				continue
			}
			dist := scentMap.Scent[p.Y][p.X]
			if dist == math.MaxInt {
				continue // unreachable cell
			}
			v := 1.0 - float32(dist)/ScentNormFactor
			if v <= 0 {
				continue
			}
			setChannel(obs, ChScent, dx+ObservationHalfCrop, dy+ObservationHalfCrop, v)
		}
	}
}

// writeChAgeLastSeen writes a Gaussian-spread decay centred on the last known
// player position into ChAgeLastSeen. Amplitude decays linearly with
// TurnsSinceLOS; spatial spread uses σ=AgeLastSeenSigma cells. This gives the
// network a directional gradient even when the player is out of the 11×11 crop.
func writeChAgeLastSeen(obs []float32, mem *PursuerMemory, origin geometry.Point) {
	if mem.LastSeen == model.NewInvalidPoint() {
		return
	}
	amplitude := 1.0 - float32(mem.TurnsSinceLOS)/float32(PursuerMemoryHorizon)
	if amplitude <= 0 {
		return
	}
	const sigma = AgeLastSeenSigma
	const twoSigmaSq = 2.0 * sigma * sigma
	for dy := -ObservationHalfCrop; dy <= ObservationHalfCrop; dy++ {
		for dx := -ObservationHalfCrop; dx <= ObservationHalfCrop; dx++ {
			wx := origin.X + dx
			wy := origin.Y + dy
			ddx := float64(wx - mem.LastSeen.X)
			ddy := float64(wy - mem.LastSeen.Y)
			d2 := ddx*ddx + ddy*ddy
			v := amplitude * float32(math.Exp(-d2/twoSigmaSq))
			if v < 0.01 {
				continue
			}
			setChannel(obs, ChAgeLastSeen, dx+ObservationHalfCrop, dy+ObservationHalfCrop, v)
		}
	}
}

func writeScalars(
	obs []float32,
	m *model.Map,
	actor *model.Actor,
	mem *PursuerMemory,
	player *model.Actor,
	playerAngle float64,
	haveTarget bool,
	seesPlayer bool,
	play *model.Playthrough,
) {
	s := obs[ObservationGridFloats:]

	maxHP := float32(actor.DerivedAttrs[model.AttrMaxHP])
	if maxHP > 0 {
		s[0] = float32(actor.Vitals[model.VitalHP]) / maxHP
	}
	maxStamina := float32(actor.DerivedAttrs[model.AttrMaxStamina])
	if maxStamina > 0 {
		s[1] = float32(actor.Vitals[model.VitalStamina]) / maxStamina
	}

	s[2] = float32(mem.TurnsSinceLOS) / float32(PursuerMemoryHorizon)
	if s[2] > 1.0 {
		s[2] = 1.0
	}

	w, h := float32(m.Width), float32(m.Height)
	if mem.LastSeen != model.NewInvalidPoint() {
		dx := mem.LastSeen.X - actor.Pos.X
		dy := mem.LastSeen.Y - actor.Pos.Y
		dist := float32(math.Hypot(float64(dx), float64(dy)))
		s[3] = dist / float32(math.Hypot(float64(w), float64(h)))
		s[4] = float32(dx) / w
		s[5] = float32(dy) / h
	}

	nearest := nearestOtherPursuer(play, actor)
	if nearest != nil {
		s[6] = float32(nearest.Pos.X-actor.Pos.X) / w
		s[7] = float32(nearest.Pos.Y-actor.Pos.Y) / h
	}

	if haveTarget {
		if isInPlayerCone(actor.Pos, player.Pos, playerAngle) {
			s[8] = 1.0
		}
		s[9] = float32(player.Pos.X-actor.Pos.X) / w
		s[10] = float32(player.Pos.Y-actor.Pos.Y) / h

		coneDirX := float32(math.Cos(playerAngle))
		coneDirY := float32(math.Sin(playerAngle))
		dx := float32(actor.Pos.X - player.Pos.X)
		dy := float32(actor.Pos.Y - player.Pos.Y)
		n := float32(math.Hypot(float64(dx), float64(dy)))
		if n > 0 {
			s[11] = (dx*coneDirX + dy*coneDirY) / n
		}
	}

	_ = seesPlayer // reserved — seesPlayer already encoded in chPlayerVisNow
}

func setChannel(obs []float32, channel, x, y int, v float32) {
	if x < 0 || x >= ObservationCropSize || y < 0 || y >= ObservationCropSize {
		return
	}
	obs[channel*ObservationCropSize*ObservationCropSize+y*ObservationCropSize+x] = v
}

// ObservationCell reads obs[channel, x, y] from a flattened tensor produced
// by BuildObservation. Out-of-range coordinates return 0 instead of panicking.
// Exported so tests and offline debug tools can inspect the layout without
// re-deriving the stride arithmetic.
func ObservationCell(obs []float32, channel, x, y int) float32 {
	if x < 0 || x >= ObservationCropSize || y < 0 || y >= ObservationCropSize {
		return 0
	}
	return obs[channel*ObservationCropSize*ObservationCropSize+y*ObservationCropSize+x]
}

func nearestAlivePlayer(play *model.Playthrough, from geometry.Point) (*model.Actor, float64, bool) {
	var best *model.Actor
	bestDist := math.Inf(1)
	for _, p := range play.Players {
		if p.Vitals[model.VitalHP] <= 0 || p.Pos == model.NewInvalidPoint() {
			continue
		}
		d := from.EuclideanDistance(p.Pos)
		if d < bestDist {
			bestDist = d
			best = p
		}
	}
	if best == nil {
		return nil, 0, false
	}
	angle := play.PlayerAimAngles[best.Id]
	return best, angle, true
}

func nearestOtherPursuer(play *model.Playthrough, self *model.Actor) *model.Actor {
	var best *model.Actor
	bestDist := math.Inf(1)
	for _, other := range play.Monsters {
		if other.Id == self.Id || other.Kind != model.ActorPursuer {
			continue
		}
		d := self.Pos.EuclideanDistance(other.Pos)
		if d < bestDist {
			bestDist = d
			best = other
		}
	}
	return best
}

// playerConeArea returns the player's current flashlight visibility area if
// one has been computed for this turn, else nil. Callers must treat nil as
// "no cone info" (all cells dark).
func playerConeArea(play *model.Playthrough, player *model.Actor, haveTarget bool) *model.VisibleArea {
	if !haveTarget || play.PlayersFoW == nil {
		return nil
	}
	return play.PlayersFoW[player.Id]
}

func isInPlayerCone(target, playerPos geometry.Point, playerAngle float64) bool {
	dx := float64(target.X - playerPos.X)
	dy := float64(target.Y - playerPos.Y)
	dist := math.Hypot(dx, dy)
	if dist < 1e-9 || dist > FlashlightRange {
		return target == playerPos
	}
	// cos(angle between vectors) = (a·b) / (|a||b|)
	coneDirX := math.Cos(playerAngle)
	coneDirY := math.Sin(playerAngle)
	cosTheta := (dx*coneDirX + dy*coneDirY) / dist
	cosHalfFOV := math.Cos(FlashlightHalfFOV * math.Pi / 180.0)
	return cosTheta >= cosHalfFOV
}

// hasLOS runs a Bresenham traversal between two grid cells and returns true
// iff no light-blocking tile sits strictly between them. Walls, Empty and
// ClosedDoor block; OpenDoors, floors and corridors do not (unlike the
// flashlight, which treats open doors as opaque to give the player a visual
// cue — monsters are not under that constraint).
func hasLOS(m *model.Map, from, to geometry.Point) bool {
	if from == to {
		return true
	}
	dx := abs(to.X - from.X)
	dy := abs(to.Y - from.Y)
	sx := 1
	if from.X > to.X {
		sx = -1
	}
	sy := 1
	if from.Y > to.Y {
		sy = -1
	}
	err := dx - dy
	x, y := from.X, from.Y
	for {
		if x == to.X && y == to.Y {
			return true
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
		if x == to.X && y == to.Y {
			return true
		}
		p := geometry.Point{X: x, Y: y}
		if !m.InBounds(p) {
			return false
		}
		t, _ := m.GetTileType(p)
		if t == model.Wall || t == model.Empty || t == model.ClosedDoor {
			return false
		}
	}
}
