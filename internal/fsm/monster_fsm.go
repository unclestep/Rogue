package fsm

import (
	"github.com/unclestep/Rogue/internal/domain/entity"
	"github.com/unclestep/Rogue/pkg/geometry"
	"log"
)

type MonsterLogic struct {
	Behaviors map[entity.ActorStateType]MonsterBehavior
	Session   *entity.GameSession
	Ctx       *MonsterBehaviorContext
}

func NewMonsterLogic(session *entity.GameSession, seed int64) *MonsterLogic {
	ai := &MonsterLogic{
		Behaviors: make(map[entity.ActorStateType]MonsterBehavior),
		Session:   session,
		Ctx:       NewBehaviorContext(session, seed),
	}
	ai.RegisterAllBehaviors()

	return ai
}

func (m *MonsterLogic) RegisterAllBehaviors() {
	m.Behaviors[entity.AIStateWander] = NewWanderBehavior(m.genWanderScentMapToCenters(), m.genWanderScentMapToEdges())
	m.Behaviors[entity.AIStateChase] = NewChaseBehavior(m.genScentMapToPlayer())
}

func (m *MonsterLogic) genWanderScentMapToCenters() [][]int {
	gameMap := m.Session.Map

	centers := make([]geometry.Point, 0)

	for _, room := range gameMap.Rooms {
		centers = append(centers, room.Center)
	}

	return gameMap.GenerateScentMap(centers)
}

func (m *MonsterLogic) genWanderScentMapToEdges() [][]int {
	gameMap := m.Session.Map

	edgeCells := make([]geometry.Point, 0)

	for _, room := range gameMap.Rooms {
		yMin, yMax := room.Pos.Y, room.Pos.Y+room.Rows-1
		xMin, xMax := room.Pos.X, room.Pos.X+room.Cols-1

		for x := xMin; x <= xMax; x++ {
			upper := geometry.Point{X: x, Y: yMin}
			bottom := geometry.Point{X: x, Y: yMax}
			edgeCells = append(edgeCells, upper, bottom)
		}

		for y := yMin + 1; y < yMax; y++ {
			left := geometry.Point{X: xMin, Y: y}
			right := geometry.Point{X: xMax, Y: y}
			edgeCells = append(edgeCells, left, right)
		}
	}

	return gameMap.GenerateScentMap(edgeCells)
}

func (m *MonsterLogic) genScentMapToPlayer() [][]int {
	gameMap := m.Session.Map
	playersCells := make([]geometry.Point, 0, len(m.Session.Players))

	for _, player := range m.Session.Players {
		playersCells = append(playersCells, player.Pos)
	}

	return gameMap.GenerateScentMap(playersCells)
}

func (m *MonsterLogic) ProccessTurn(actor *entity.Actor) {
	if actor == nil {
		return
	}

	for (actor.CanMove() || actor.CanAttack()) && (actor.HasStaminaForHit() || actor.HasStaminaForMove()) {
		if !m.canMakeMove(actor) {
			return
		}

		behavior, exists := m.Behaviors[actor.CurState]
		if !exists {
			log.Fatalf("State %v does not exist", actor.CurState)
		}

		newState, events := behavior.Update(actor, m.Ctx)
		if newState != actor.CurState {
			actor.CurState = newState
		}

		// Immediately apply all generated events
		for _, event := range events {
			event.Perform(m.Session, m.Ctx.Rng)
		}
	}
}

func (m *MonsterLogic) canMakeMove(actor *entity.Actor) bool {
	findPoint := m.Ctx.Move.Moves.GetFindPointFunc(actor.MovePattern)
	searchLen := 1

	// Ghost move distance is determined by its hostility value
	if actor.MovePattern == entity.TeleportMovePattern {
		searchLen = actor.DerivedAttrs[entity.Hostility]
	}

	// If the actor is surrounded by other actor but can only move
	_, found := findPoint(actor, searchLen, m.Session.Map, true) // Find empty point
	if actor.CanMove() && !actor.CanAttack() && !found {
		return false
	}

	// If there are no other actors near the actor but he can only attack
	_, found = findPoint(actor, searchLen, m.Session.Map, false) // Find occupied point
	if !actor.CanMove() && actor.CanAttack() && found {
		return false
	}

	// Standard case
	if !actor.CanMove() && !actor.CanAttack() {
		return false
	}

	return true
}
