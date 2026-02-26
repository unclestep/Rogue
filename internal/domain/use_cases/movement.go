package usecases

import (
	"github.com/unclestep/Rogue/internal/domain/entity"
	"github.com/unclestep/Rogue/internal/pkg/geometry"
	"log"
	"math/rand"
	"time"
)

//
//
// --- MOVEMENT SERVICE ---
//
//

type MovementService struct {
	session      *entity.GameSession
	moveRegistry *MoveRegistry
	seed         int64
	rng          *rand.Rand
}

//
// -- CONSTRUCTOR --
//

func NewMoveService(session *entity.GameSession) *MovementService {
	ms := &MovementService{
		session:      session,
		moveRegistry: NewMoveRegistry(),
		seed:         time.Now().UnixNano(),
	}

	ms.rng = rand.New(rand.NewSource(ms.seed))
	ms.RegisterAll()

	return ms
}

//
// -- SETTERS --
//

func (ms *MovementService) SetSeed(seed int64) {
	ms.seed = seed
	ms.rng = rand.New(rand.NewSource(ms.seed))
}

//
// -- INTERFACE --
//

type WalkableGrid interface {
	IsWalkable(p geometry.Point) bool
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

func (ms *MovementService) RegisterAll() {
	ms.moveRegistry.movements[entity.DefaultMovePattern] = ms.DefaultMove
	ms.moveRegistry.movements[entity.TeleportMovePattern] = ms.TeleportMove
	ms.moveRegistry.movements[entity.DiagonalMovePattern] = ms.DiagonalMove
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
	if !event.WasPerformed() {
		return
	}

	mover := event.Mover.Actor
	oldPos := mover.Pos

	event.Mover.Apply(rng)

	newPos := mover.Pos
	gs.Map.Move(oldPos, newPos)
}

//
// -- MAIN MOVE FUNCTION --
//

func (ms *MovementService) ExecuteMove(mover *entity.Actor, scentMap [][]int, grid WalkableGrid) *MoveEvent {
	event := NewMoveEvent(mover)

	if !mover.CanMove() {
		event.Outcome = MoveOutcomeCantMove
		return event
	}

	if !mover.HasStaminaForMove() {
		event.Outcome = MoveOutcomeNoStamina
		return event
	}

	moveFunc := ms.moveRegistry.GetMoveFunc(mover.MovePattern)
	nextPos := moveFunc(mover, scentMap, grid)

	event.Mover.VitalsChange[entity.Stamina] -= mover.DerivedAttrs[entity.MoveStaminaCost]
	event.Mover.PosChange = nextPos.Sub(mover.Pos)

	if !event.Mover.PosChange.Equal(geometry.Point{X: 0, Y: 0}) {
		entity.ResolveReactions(entity.TriggerOnMove, event.Mover, event.Mover, ms.rng)
	}

	return event
}

//
// -- MOVE PATTERNS --
//

func (ms *MovementService) DefaultMove(mover *entity.Actor, scentMap [][]int, grid WalkableGrid) geometry.Point {
	ind := 0
	availablePoints := ms.FindAllMinCardinals(mover.Pos, 1, scentMap, grid)

	if len(availablePoints) == 0 {
		return mover.Pos
	}

	ind = ms.rng.Intn(len(availablePoints))

	return availablePoints[ind]
}

func (ms *MovementService) FindAllMinCardinals(center geometry.Point, searchLen int, scentMap [][]int, grid WalkableGrid) []geometry.Point {
	mins := make([]geometry.Point, 0)
	minScent := scentMap[center.Y][center.X]

	yMin, yMax := max(0, center.Y-searchLen), min(len(scentMap)-1, center.Y+searchLen)
	xMin, xMax := max(0, center.X-searchLen), min(len(scentMap[0])-1, center.X+searchLen)

	for y := yMin; y <= yMax; y++ {
		p := geometry.Point{Y: y, X: center.X}
		pScent := scentMap[y][center.X]

		if !grid.IsWalkable(p) {
			continue
		}

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
		pScent := scentMap[center.Y][x]

		if !grid.IsWalkable(p) {
			continue
		}

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

func (ms *MovementService) TeleportMove(mover *entity.Actor, scentMap [][]int, grid WalkableGrid) geometry.Point {
	ind := 0
	rad := mover.DerivedAttrs[entity.Hostility]
	availablePoints := ms.FindAllMinMoore(mover.Pos, rad, scentMap, grid)

	if len(availablePoints) == 0 {
		return mover.Pos
	}

	ind = ms.rng.Intn(len(availablePoints))

	return availablePoints[ind]

}

func (ms *MovementService) FindAllMinMoore(center geometry.Point, rad int, scentMap [][]int, grid WalkableGrid) []geometry.Point {
	mins := make([]geometry.Point, 0)
	minScent := scentMap[center.Y][center.X]

	yMin, yMax := max(0, center.Y-rad), min(len(scentMap)-1, center.Y+rad)
	xMin, xMax := max(0, center.X-rad), min(len(scentMap[0])-1, center.X+rad)

	for y := yMin; y <= yMax; y++ {
		for x := xMin; x <= xMax; x++ {
			p := geometry.Point{Y: y, X: x}
			pScent := scentMap[y][x]

			if !grid.IsWalkable(p) {
				continue
			}

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

func (ms *MovementService) DiagonalMove(mover *entity.Actor, scentMap [][]int, grid WalkableGrid) geometry.Point {
	ind := 0
	availablePoints := ms.FindAllMinDiagonal(mover.Pos, 1, scentMap, grid)

	if len(availablePoints) == 0 {
		return mover.Pos
	}

	ind = ms.rng.Intn(len(availablePoints))

	return availablePoints[ind]
}

func (ms *MovementService) FindAllMinDiagonal(center geometry.Point, searchLen int, scentMap [][]int, grid WalkableGrid) []geometry.Point {
	mins := make([]geometry.Point, 0)
	minScent := scentMap[center.Y][center.X]
	i := 0

	yMin, yMax := max(0, center.Y-searchLen), min(len(scentMap)-1, center.Y+searchLen)
	xMin, xMax := max(0, center.X-searchLen), min(len(scentMap[0])-1, center.X+searchLen)

	for x := xMin; x <= xMax; x++ {
		upperAndBottom := make([]geometry.Point, 0, 2)
		bottom := geometry.Point{Y: yMin + i, X: x}
		upper := geometry.Point{Y: yMax - i, X: x}

		upperAndBottom = append(upperAndBottom, bottom)
		if upper != bottom {
			upperAndBottom = append(upperAndBottom, upper)
		}

		for _, p := range upperAndBottom {
			pScent := scentMap[p.Y][p.X]

			if !grid.IsWalkable(p) {
				continue
			}

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
