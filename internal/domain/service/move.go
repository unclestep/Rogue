package service

import (
	"log"
	"math/rand"

	"github.com/unclestep/Rogue/internal/domain/entity"
	"github.com/unclestep/Rogue/pkg/geometry"
)

//
//
// --- MOVEMENT SERVICE ---
//
//

type Move struct {
	session  *entity.GameSession
	moves    *MoveRegistry
	resolver *Resolver
	seed     int64
	rng      *rand.Rand
}

//
// -- CONSTRUCTOR --
//

func NewMoveService(session *entity.GameSession, seed int64) *Move {
	m := &Move{
		session:  session,
		moves:    NewMoveRegistry(),
		resolver: NewResolverService(session, seed),
		seed:     seed,
		rng:      rand.New(rand.NewSource(seed)),
	}
	m.RegisterAll()

	return m
}

//
// -- SETTERS --
//

func (m *Move) SetSeed(seed int64) {
	m.seed = seed
	m.rng = rand.New(rand.NewSource(m.seed))
}

//
// -- INTERFACE --
//

type WalkableGrid interface {
	IsWalkable(p geometry.Point) bool
	GetActorID(p geometry.Point) (int, bool)
}

//
// -- MOVE REGISTRY --
//

type MoveRegistry struct {
	movements map[entity.MovePatternType]func(*entity.Actor, [][]int, WalkableGrid) geometry.Point
}

func NewMoveRegistry() *MoveRegistry {
	return &MoveRegistry{
		movements: make(map[entity.MovePatternType]func(*entity.Actor, [][]int, WalkableGrid) geometry.Point),
	}
}

func (m *Move) RegisterAll() {
	m.moves.movements[entity.DefaultMovePattern] = m.DefaultMove
	m.moves.movements[entity.TeleportMovePattern] = m.TeleportMove
	m.moves.movements[entity.DiagonalMovePattern] = m.DiagonalMove
}

func (mr *MoveRegistry) Register(movement entity.MovePatternType, moveFunc func(*entity.Actor, [][]int, WalkableGrid) geometry.Point) {
	mr.movements[movement] = moveFunc
}

func (mr *MoveRegistry) GetMoveFunc(movement entity.MovePatternType) func(*entity.Actor, [][]int, WalkableGrid) geometry.Point {
	if moveFunc, exists := mr.movements[movement]; exists {
		return moveFunc
	}
	log.Fatalf("[ERROR] Such movement as %v is not registered\n", movement)
	return nil
}

//
// -- MOVE EVENT --
//

type MoveEvent struct {
	Mover   *entity.ActorImpact
	Outcome MoveOutcome
}

type MoveOutcome int

const (
	MoveOutcomeSuccess MoveOutcome = iota
	MoveOutcomeNoStamina
	MoveOutcomeCantMove
	MoveOutcomeNoSpace
)

func NewMoveEvent(actor *entity.Actor) *MoveEvent {
	return &MoveEvent{
		Mover: entity.NewActorImpact(actor),
	}
}

func (event *MoveEvent) WasPerformed() bool {
	return event.Outcome == MoveOutcomeSuccess
}

func (event *MoveEvent) Perform(gs *entity.GameSession, rng *rand.Rand) {
	mover := event.Mover.Actor
	oldPos := mover.Pos

	event.Mover.Apply(rng)

	newPos := mover.Pos
	gs.Map.Move(oldPos, newPos)
}

//
// -- MAIN MOVE FUNCTION --
//

func (m *Move) ExecuteMove(mover *entity.Actor, scentMap [][]int, grid WalkableGrid) []Event {
	event := NewMoveEvent(mover)

	moveFunc := m.moves.GetMoveFunc(mover.MovePattern)
	nextPos := moveFunc(mover, scentMap, grid)
	event.Mover.PosChange = nextPos.Sub(mover.Pos)

	return m.resolver.ResolveMove(event)
}

//
// -- MOVE PATTERNS --
//

func (m *Move) DefaultMove(mover *entity.Actor, scentMap [][]int, grid WalkableGrid) geometry.Point {
	ind := 0
	availablePoints := m.FindAllMinCardinals(mover, 1, scentMap, grid)

	if len(availablePoints) == 0 {
		return mover.Pos
	}

	ind = m.rng.Intn(len(availablePoints))

	return availablePoints[ind]
}

func (m *Move) FindAllMinCardinals(mover *entity.Actor, searchLen int, scentMap [][]int, grid WalkableGrid) []geometry.Point {
	center := mover.Pos
	mins := make([]geometry.Point, 0)
	minScent := scentMap[center.Y][center.X]

	yMin, yMax := max(0, center.Y-searchLen), min(len(scentMap)-1, center.Y+searchLen)
	xMin, xMax := max(0, center.X-searchLen), min(len(scentMap[0])-1, center.X+searchLen)

	for y := yMin; y <= yMax; y++ {
		p := geometry.Point{Y: y, X: center.X}
		if p == center {
			continue
		}

		if !grid.IsWalkable(p) {
			continue
		}

		if actorId, _ := grid.GetActorID(p); m.session.IsMonsterExists(entity.ActorId(actorId)) && mover.Kind != entity.PlayerType {
			continue
		}

		pScent := scentMap[y][center.X]

		if minScent > pScent {
			mins = mins[:0]
			mins = append(mins, p)
			minScent = pScent
		} else if minScent == pScent {
			mins = append(mins, p)
		}
	}

	for x := xMin; x <= xMax; x++ {
		p := geometry.Point{Y: center.Y, X: x}
		if p == center {
			continue
		}

		if !grid.IsWalkable(p) {
			continue
		}

		if actorId, _ := grid.GetActorID(p); m.session.IsMonsterExists(entity.ActorId(actorId)) && mover.Kind != entity.PlayerType {
			continue
		}

		pScent := scentMap[center.Y][x]

		if minScent > pScent {
			mins = mins[:0]
			mins = append(mins, p)
			minScent = pScent
		} else if minScent == pScent {
			mins = append(mins, p)
		}
	}

	return mins
}

func (m *Move) TeleportMove(mover *entity.Actor, scentMap [][]int, grid WalkableGrid) geometry.Point {
	ind := 0
	rad := mover.DerivedAttrs[entity.Hostility]
	availablePoints := m.FindAllMinMoore(mover, rad, scentMap, grid)

	if len(availablePoints) == 0 {
		return mover.Pos
	}

	ind = m.rng.Intn(len(availablePoints))

	return availablePoints[ind]
}

func (m *Move) FindAllMinMoore(mover *entity.Actor, rad int, scentMap [][]int, grid WalkableGrid) []geometry.Point {
	center := mover.Pos
	mins := make([]geometry.Point, 0)
	minScent := scentMap[center.Y][center.X]

	yMin, yMax := max(0, center.Y-rad), min(len(scentMap)-1, center.Y+rad)
	xMin, xMax := max(0, center.X-rad), min(len(scentMap[0])-1, center.X+rad)

	for y := yMin; y <= yMax; y++ {
		for x := xMin; x <= xMax; x++ {
			p := geometry.Point{Y: y, X: x}
			if p == center {
				continue
			}

			if !grid.IsWalkable(p) {
				continue
			}

			if actorId, _ := grid.GetActorID(p); m.session.IsMonsterExists(entity.ActorId(actorId)) && mover.Kind != entity.PlayerType {
				continue
			}

			pScent := scentMap[y][x]

			if minScent > pScent {
				mins = mins[:0]
				mins = append(mins, p)
				minScent = pScent
			} else if minScent == pScent {
				mins = append(mins, p)
			}
		}
	}

	return mins
}

func (m *Move) DiagonalMove(mover *entity.Actor, scentMap [][]int, grid WalkableGrid) geometry.Point {
	ind := 0
	availablePoints := m.FindAllMinDiagonal(mover, 1, scentMap, grid)

	if len(availablePoints) == 0 {
		return mover.Pos
	}

	ind = m.rng.Intn(len(availablePoints))

	return availablePoints[ind]
}

func (m *Move) FindAllMinDiagonal(mover *entity.Actor, searchLen int, scentMap [][]int, grid WalkableGrid) []geometry.Point {
	center := mover.Pos
	mins := make([]geometry.Point, 0)
	minScent := scentMap[center.Y][center.X]
	i := 0

	yMin, yMax := center.Y-searchLen, center.Y+searchLen
	xMin, xMax := center.X-searchLen, center.X+searchLen

	upperAndBottom := make([]geometry.Point, 0, 2)

	for x := xMin; x <= xMax; x++ {
		upperAndBottom = upperAndBottom[:0]

		upper := geometry.Point{Y: yMin + i, X: x}
		bottom := geometry.Point{Y: yMax - i, X: x}

		upperAndBottom = append(upperAndBottom, bottom, upper)

		for _, p := range upperAndBottom {
			if p == center {
				continue
			}

			if !grid.IsWalkable(p) {
				continue
			}

			if actorId, _ := grid.GetActorID(p); m.session.IsMonsterExists(entity.ActorId(actorId)) && mover.Kind != entity.PlayerType {
				continue
			}

			pScent := scentMap[p.Y][p.X]

			if minScent > pScent {
				mins = mins[:0]
				mins = append(mins, p)
				minScent = pScent
			} else if minScent == pScent {
				mins = append(mins, p)
			}
		}

		i++
	}

	return mins
}
