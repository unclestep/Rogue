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
	Session  *entity.GameSession
	Moves    *MoveRegistry
	Resolver *Resolver
	seed     int64
	rng      *rand.Rand
}

//
// -- CONSTRUCTOR --
//

func NewMoveService(session *entity.GameSession, seed int64) *Move {
	m := &Move{
		Session:  session,
		Moves:    NewMoveRegistry(),
		Resolver: NewResolverService(session, seed),
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
	m.Resolver.SetSeed(seed)
}

//
// -- INTERFACE --
//

type WalkableGrid interface {
	IsWalkable(p geometry.Point) bool
	GetActorID(p geometry.Point) (int, bool)
	GetDimensions() geometry.Point
}

//
// -- MOVE REGISTRY --
//

type MoveRegistry struct {
	movements map[entity.MovePatternType]func(*entity.Actor, [][]int, WalkableGrid) geometry.Point
	findPoint map[entity.MovePatternType]func(*entity.Actor, int, WalkableGrid, bool) (geometry.Point, bool)
}

func NewMoveRegistry() *MoveRegistry {
	return &MoveRegistry{
		movements: make(map[entity.MovePatternType]func(*entity.Actor, [][]int, WalkableGrid) geometry.Point),
		findPoint: make(map[entity.MovePatternType]func(*entity.Actor, int, WalkableGrid, bool) (geometry.Point, bool)),
	}
}

func (m *Move) RegisterAll() {
	m.Moves.movements[entity.DefaultMovePattern] = m.DefaultMove
	m.Moves.movements[entity.TeleportMovePattern] = m.TeleportMove
	m.Moves.movements[entity.DiagonalMovePattern] = m.DiagonalMove

	m.Moves.findPoint[entity.DefaultMovePattern] = m.FindPointCardinal
	m.Moves.findPoint[entity.TeleportMovePattern] = m.FindPointMoore
	m.Moves.findPoint[entity.DiagonalMovePattern] = m.FindPointDiagonal
}

func (mr *MoveRegistry) GetMoveFunc(movement entity.MovePatternType) func(*entity.Actor, [][]int, WalkableGrid) geometry.Point {
	if moveFunc, exists := mr.movements[movement]; exists {
		return moveFunc
	}
	log.Fatalf("[ERROR] Such movement as %v is not registered\n", movement)
	return nil
}

func (mr *MoveRegistry) GetFindPointFunc(movement entity.MovePatternType) func(*entity.Actor, int, WalkableGrid, bool) (geometry.Point, bool) {
	if findFreePointFunc, exists := mr.findPoint[movement]; exists {
		return findFreePointFunc
	}
	log.Fatalf("[ERROR] Such find free point function as %v is not registered\n", movement)
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

	moveFunc := m.Moves.GetMoveFunc(mover.MovePattern)
	nextPos := moveFunc(mover, scentMap, grid)
	event.Mover.PosChange = nextPos.Sub(mover.Pos)

	return m.Resolver.ResolveMove(event)
}

//
// -- MOVE PATTERNS --
//

func (m *Move) DefaultMove(mover *entity.Actor, scentMap [][]int, grid WalkableGrid) geometry.Point {
	ind := 0
	availablePoints := m.FindAllMinCardinal(mover, 1, scentMap, grid)

	if len(availablePoints) == 0 {
		return mover.Pos
	}

	ind = m.rng.Intn(len(availablePoints))

	return availablePoints[ind]
}

func (m *Move) FindAllMinCardinal(mover *entity.Actor, searchLen int, scentMap [][]int, grid WalkableGrid) []geometry.Point {
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

		if actorId, _ := grid.GetActorID(p); m.Session.IsMonster(entity.ActorId(actorId)) && mover.Kind != entity.PlayerType {
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

		if actorId, _ := grid.GetActorID(p); m.Session.IsMonster(entity.ActorId(actorId)) && mover.Kind != entity.PlayerType {
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

func (m *Move) FindPointCardinal(mover *entity.Actor, searchLen int, grid WalkableGrid, findEmpty bool) (geometry.Point, bool) {
	start := mover.Pos
	height, width := grid.GetDimensions().Y, grid.GetDimensions().X
	yMin, yMax := max(0, start.Y-searchLen), min(height-1, start.Y+searchLen)
	xMin, xMax := max(0, start.X-searchLen), min(width-1, start.X+searchLen)

	for y := yMin; y <= yMax; y++ {
		p := geometry.Point{X: start.X, Y: y}
		if p == start {
			continue
		}

		actorId, _ := grid.GetActorID(p)
		mustBePositive := actorId == 0
		if !findEmpty {
			mustBePositive = actorId != 0
		}
		if grid.IsWalkable(p) && mustBePositive {
			return p, true
		}
	}

	for x := xMin; x <= xMax; x++ {
		p := geometry.Point{X: x, Y: start.Y}
		if p == start {
			continue
		}

		actorId, _ := grid.GetActorID(p)
		mustBePositive := actorId == 0
		if !findEmpty {
			mustBePositive = actorId != 0
		}
		if grid.IsWalkable(p) && mustBePositive {
			return p, true
		}
	}

	return entity.NewInvalidPoint(), false
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

			if actorId, _ := grid.GetActorID(p); m.Session.IsMonster(entity.ActorId(actorId)) && mover.Kind != entity.PlayerType {
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

func (m *Move) FindPointMoore(mover *entity.Actor, rad int, grid WalkableGrid, findEmpty bool) (geometry.Point, bool) {
	start := mover.Pos
	height, width := grid.GetDimensions().Y, grid.GetDimensions().X
	yMin, yMax := max(0, start.Y-rad), min(height-1, start.Y+rad)
	xMin, xMax := max(0, start.X-rad), min(width-1, start.X+rad)

	for y := yMin; y <= yMax; y++ {
		for x := xMin; x <= xMax; x++ {
			p := geometry.Point{X: x, Y: y}
			if p == start {
				continue
			}

			actorId, _ := grid.GetActorID(p)
			mustBePositive := actorId == 0
			if !findEmpty {
				mustBePositive = actorId != 0
			}
			if grid.IsWalkable(p) && mustBePositive {
				return p, true
			}
		}
	}

	return entity.NewInvalidPoint(), false
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

			if actorId, _ := grid.GetActorID(p); m.Session.IsMonster(entity.ActorId(actorId)) && mover.Kind != entity.PlayerType {
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

func (m *Move) FindPointDiagonal(mover *entity.Actor, searchLen int, grid WalkableGrid, findEmpty bool) (geometry.Point, bool) {
	start := mover.Pos
	i := 0
	yMin, yMax := start.Y-searchLen, start.Y+searchLen
	xMin, xMax := start.X-searchLen, start.X+searchLen

	upperAndBottom := make([]geometry.Point, 0, 2)

	for x := xMin; x <= xMax; x++ {
		upperAndBottom = upperAndBottom[:0]

		upper := geometry.Point{Y: yMin + i, X: x}
		bottom := geometry.Point{Y: yMax - i, X: x}

		upperAndBottom = append(upperAndBottom, bottom, upper)

		for _, p := range upperAndBottom {
			if p == start {
				continue
			}

			actorId, _ := grid.GetActorID(p)
			mustBePositive := actorId == 0
			if !findEmpty {
				mustBePositive = actorId != 0
			}
			if grid.IsWalkable(p) && mustBePositive {
				return p, true
			}
		}

		i++
	}

	return entity.NewInvalidPoint(), false
}
