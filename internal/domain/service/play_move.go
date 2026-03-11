package service

import (
	"log"
	"math/rand"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

//
//
// --- MOVEMENT SERVICE ---
//
//

type Move struct {
	Session  *model.GameSession
	Moves    *MoveRegistry
	Resolver *Resolver
	seed     int64
	rng      *rand.Rand
}

//
// -- CONSTRUCTOR --
//

func NewMoveService(session *model.GameSession, seed int64) *Move {
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
// -- MOVE REGISTRY --
//

type MoveRegistry struct {
	movements map[model.MovePatternType]func(*model.Actor, [][]int, WalkableGrid) geometry.Point
	findPoint map[model.MovePatternType]func(*model.Actor, int, WalkableGrid, bool) (geometry.Point, bool)
}

func NewMoveRegistry() *MoveRegistry {
	return &MoveRegistry{
		movements: make(map[model.MovePatternType]func(*model.Actor, [][]int, WalkableGrid) geometry.Point),
		findPoint: make(map[model.MovePatternType]func(*model.Actor, int, WalkableGrid, bool) (geometry.Point, bool)),
	}
}

func (m *Move) RegisterAll() {
	m.Moves.movements[model.MovePatternDefault] = m.DefaultMove
	m.Moves.movements[model.MovePatternTeleport] = m.TeleportMove
	m.Moves.movements[model.MovePatternDiagonal] = m.DiagonalMove

	m.Moves.findPoint[model.MovePatternDefault] = m.FindPointCardinal
	m.Moves.findPoint[model.MovePatternTeleport] = m.FindPointMoore
	m.Moves.findPoint[model.MovePatternDiagonal] = m.FindPointDiagonal
}

func (mr *MoveRegistry) GetMoveFunc(movement model.MovePatternType) func(*model.Actor, [][]int, WalkableGrid) geometry.Point {
	if moveFunc, exists := mr.movements[movement]; exists {
		return moveFunc
	}
	log.Fatalf("[ERROR] Such movement as %v is not registered\n", movement)
	return nil
}

func (mr *MoveRegistry) GetFindPointFunc(movement model.MovePatternType) func(*model.Actor, int, WalkableGrid, bool) (geometry.Point, bool) {
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
	Mover   *model.ActorImpact
	Outcome MoveOutcome
}

type MoveOutcome int

const (
	MoveOutcomeSuccess MoveOutcome = iota
	MoveOutcomeNoStamina
	MoveOutcomeCantMove
	MoveOutcomeNoSpace
)

func NewMoveEvent(actor *model.Actor) *MoveEvent {
	return &MoveEvent{
		Mover: model.NewActorImpact(actor),
	}
}

func (event *MoveEvent) WasPerformed() bool {
	return event.Outcome == MoveOutcomeSuccess
}

func (event *MoveEvent) Perform(gs *model.GameSession, rng *rand.Rand) {
	mover := event.Mover.Actor
	oldPos := mover.Pos

	event.Mover.Apply(rng)

	newPos := mover.Pos
	gs.Map.Move(oldPos, newPos)
}

//
// -- MAIN MOVE FUNCTION --
//

func (m *Move) ExecuteMove(mover *model.Actor, scentMap [][]int, grid WalkableGrid) []Event {
	event := NewMoveEvent(mover)

	moveFunc := m.Moves.GetMoveFunc(mover.MovePattern)
	nextPos := moveFunc(mover, scentMap, grid)
	event.Mover.PosChange = nextPos.Sub(mover.Pos)

	return m.Resolver.ResolveMove(event)
}

//
// -- MOVE PATTERNS --
//

func (m *Move) DefaultMove(mover *model.Actor, scentMap [][]int, grid WalkableGrid) geometry.Point {
	ind := 0
	availablePoints := m.FindAllMinCardinal(mover, 1, scentMap, grid)

	if len(availablePoints) == 0 {
		return mover.Pos
	}

	ind = m.rng.Intn(len(availablePoints))

	return availablePoints[ind]
}

func (m *Move) FindAllMinCardinal(mover *model.Actor, searchLen int, scentMap [][]int, grid WalkableGrid) []geometry.Point {
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

		if actorId, _ := grid.GetActorID(p); m.Session.IsMonster(model.ActorId(actorId)) && mover.Kind != model.ActorPlayer {
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

		if actorId, _ := grid.GetActorID(p); m.Session.IsMonster(model.ActorId(actorId)) && mover.Kind != model.ActorPlayer {
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

func (m *Move) FindPointCardinal(mover *model.Actor, searchLen int, grid WalkableGrid, findEmpty bool) (geometry.Point, bool) {
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

	return model.NewInvalidPoint(), false
}

func (m *Move) TeleportMove(mover *model.Actor, scentMap [][]int, grid WalkableGrid) geometry.Point {
	ind := 0
	rad := mover.DerivedAttrs[model.AttrHostility]
	availablePoints := m.FindAllMinMoore(mover, rad, scentMap, grid)

	if len(availablePoints) == 0 {
		return mover.Pos
	}

	ind = m.rng.Intn(len(availablePoints))

	return availablePoints[ind]
}

func (m *Move) FindAllMinMoore(mover *model.Actor, rad int, scentMap [][]int, grid WalkableGrid) []geometry.Point {
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

			if actorId, _ := grid.GetActorID(p); m.Session.IsMonster(model.ActorId(actorId)) && mover.Kind != model.ActorPlayer {
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

func (m *Move) FindPointMoore(mover *model.Actor, rad int, grid WalkableGrid, findEmpty bool) (geometry.Point, bool) {
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

	return model.NewInvalidPoint(), false
}

func (m *Move) DiagonalMove(mover *model.Actor, scentMap [][]int, grid WalkableGrid) geometry.Point {
	ind := 0
	availablePoints := m.FindAllMinDiagonal(mover, 1, scentMap, grid)

	if len(availablePoints) == 0 {
		return mover.Pos
	}

	ind = m.rng.Intn(len(availablePoints))

	return availablePoints[ind]
}

func (m *Move) FindAllMinDiagonal(mover *model.Actor, searchLen int, scentMap [][]int, grid WalkableGrid) []geometry.Point {
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

			if actorId, _ := grid.GetActorID(p); m.Session.IsMonster(model.ActorId(actorId)) && mover.Kind != model.ActorPlayer {
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

func (m *Move) FindPointDiagonal(mover *model.Actor, searchLen int, grid WalkableGrid, findEmpty bool) (geometry.Point, bool) {
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

	return model.NewInvalidPoint(), false
}
