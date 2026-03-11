// package service
//
// import (
// 	"log"
// 	"math/rand"
// 	"slices"
//
// 	"github.com/unclestep/Rogue/internal/domain/model"
// 	"github.com/unclestep/Rogue/internal/domain/service"
// 	"github.com/unclestep/Rogue/pkg/algorithm"
// 	"github.com/unclestep/Rogue/pkg/geometry"
// )
//
// type MonsterLogic struct {
// 	Behaviors         map[model.ActorStateType]MonsterBehavior
// 	Monsters          map[model.ActorId]MonsterBehavior
// 	MonstersSlice     []model.ActorId
// 	Session           *model.GameSession
// 	Ctx               *MonsterBehaviorContext
// 	CachedRoomCenters *ScentMap
// 	CachedRoomEdges   *ScentMap
// 	CachedPlayers     *ScentMap
// }
//
// type ScentMap struct {
// 	Scent               [][]int
// 	TicksFromLastUpdate int
// }
//
// func (m *MonsterLogic) Cleanup() {
// 	for i := len(m.MonstersSlice) - 1; i >= 0; i-- {
// 		id := m.MonstersSlice[i]
// 		monster, exists := m.Session.Monsters[id]
// 		if !exists || monster.Vitals[model.VitalHP] <= 0 {
// 			delete(m.Monsters, id)
// 			m.MonstersSlice = algorithm.RemoveOrderly(m.MonstersSlice, i)
// 		}
// 	}
// }
//
// func (m *MonsterLogic) Rebuild() {
// 	m.Reset()
// 	for _, monster := range m.Session.Monsters {
// 		m.Monsters[monster.Id] = NewWanderBehavior(m.CachedRoomCenters, m.CachedRoomEdges)
// 		m.MonstersSlice = append(m.MonstersSlice, monster.Id)
// 	}
// 	slices.Sort(m.MonstersSlice)
//
// 	m.CachedRoomCenters = &ScentMap{Scent: m.genWanderScentMapToCenters()}
// 	m.CachedRoomEdges = &ScentMap{Scent: m.genWanderScentMapToEdges()}
// 	m.CachedPlayers = &ScentMap{Scent: m.genScentMapToPlayers()}
// }
//
// func (m *MonsterLogic) Reset() {
// 	m.Monsters = make(map[model.ActorId]MonsterBehavior, len(m.Session.Monsters))
// 	m.MonstersSlice = make([]model.ActorId, 0, len(m.Session.Monsters))
// 	m.CachedRoomCenters = nil
// 	m.CachedRoomEdges = nil
// 	m.CachedPlayers = nil
// }
//
// func (m *MonsterLogic) UpdateScent(rng *rand.Rand) {
// 	chance := m.CachedPlayers.TicksFromLastUpdate * 33
// 	if rng.Intn(100) < chance { // Randomly update player's scent
// 		m.CachedPlayers.Scent = m.genScentMapToPlayers()
// 		m.CachedPlayers.TicksFromLastUpdate = 0
// 	} else {
// 		m.CachedPlayers.TicksFromLastUpdate++
// 	}
// }
//
// func NewMonsterLogic(session *model.GameSession, seed int64) *MonsterLogic {
// 	m := &MonsterLogic{
// 		Behaviors:     make(map[model.ActorStateType]MonsterBehavior),
// 		Monsters:      make(map[model.ActorId]MonsterBehavior, len(session.Monsters)),
// 		MonstersSlice: make([]model.ActorId, 0, len(session.Monsters)),
// 		Session:       session,
// 		Ctx:           NewBehaviorContext(session, seed),
// 	}
//
// 	m.CachedRoomCenters = &ScentMap{Scent: m.genWanderScentMapToCenters()}
// 	m.CachedRoomEdges = &ScentMap{Scent: m.genWanderScentMapToEdges()}
// 	m.CachedPlayers = &ScentMap{Scent: m.genScentMapToPlayers()}
//
// 	m.RegisterAll()
//
// 	for _, monster := range m.Session.Monsters {
// 		m.Monsters[monster.Id] = NewWanderBehavior(m.CachedRoomCenters, m.CachedRoomEdges)
// 		m.MonstersSlice = append(m.MonstersSlice, monster.Id)
// 	}
// 	slices.Sort(m.MonstersSlice)
//
// 	return m
// }
//
// func (m *MonsterLogic) RegisterAll() {
// 	m.Behaviors[model.ActorStateWander] = NewWanderBehavior(m.CachedRoomCenters, m.CachedRoomEdges)
// 	m.Behaviors[model.ActorStateChase] = NewChaseBehavior(m.CachedPlayers)
// }
//
// func (m *MonsterLogic) genWanderScentMapToCenters() [][]int {
// 	gameMap := m.Session.Map
//
// 	centers := make([]geometry.Point, 0)
//
// 	for _, room := range gameMap.Rooms {
// 		centers = append(centers, room.Center)
// 	}
//
// 	return gameMap.GenerateScentMap(centers)
// }
//
// func (m *MonsterLogic) genWanderScentMapToEdges() [][]int {
// 	gameMap := m.Session.Map
//
// 	edgeCells := make([]geometry.Point, 0)
//
// 	for _, room := range gameMap.Rooms {
// 		yMin, yMax := room.Pos.Y, room.Pos.Y+room.Rows-1
// 		xMin, xMax := room.Pos.X, room.Pos.X+room.Cols-1
//
// 		for x := xMin; x <= xMax; x++ {
// 			upper := geometry.Point{X: x, Y: yMin}
// 			bottom := geometry.Point{X: x, Y: yMax}
// 			edgeCells = append(edgeCells, upper, bottom)
// 		}
//
// 		for y := yMin + 1; y < yMax; y++ {
// 			left := geometry.Point{X: xMin, Y: y}
// 			right := geometry.Point{X: xMax, Y: y}
// 			edgeCells = append(edgeCells, left, right)
// 		}
// 	}
//
// 	return gameMap.GenerateScentMap(edgeCells)
// }
//
// func (m *MonsterLogic) genScentMapToPlayers() [][]int {
// 	gameMap := m.Session.Map
// 	playersCells := make([]geometry.Point, 0, len(m.Session.Players))
//
// 	for _, player := range m.Session.Players {
// 		playersCells = append(playersCells, player.Pos)
// 	}
//
// 	return gameMap.GenerateScentMap(playersCells)
// }
//
// func (m *MonsterLogic) ProcessAllMonstersTurn() []service.Event {
// 	events := make([]service.Event, 0, 2*len(m.Monsters))
// 	for i := len(m.MonstersSlice) - 1; i >= 0; i-- {
// 		id := m.MonstersSlice[i]
// 		monster, exists := m.Session.Monsters[id]
// 		if !exists {
// 			delete(m.Monsters, id)
// 			m.MonstersSlice = algorithm.RemoveOrderly(m.MonstersSlice, i)
// 			continue
// 		}
// 		events = append(events, m.ProcessMonsterTurn(monster)...)
// 	}
//
// 	return events
// }
//
// func (m *MonsterLogic) ProcessMonsterTurn(monster *model.Actor) []service.Event {
// 	if monster == nil {
// 		return nil
// 	}
//
// 	totalEvents := make([]service.Event, 0, 2)
//
// 	attempt := 0
// 	for (monster.CanMove() || monster.CanAttack()) && (monster.HasStaminaForHit() || monster.HasStaminaForMove()) {
// 		if attempt > 100 { // Futureproof value of infinte loop protection
// 			break
// 		}
// 		if !m.CanMakeMove(monster) {
// 			return totalEvents
// 		}
//
// 		behavior, exists := m.Monsters[monster.Id]
// 		if !exists {
// 			behavior = m.Behaviors[model.ActorStateWander].Clone()
// 			m.Monsters[monster.Id] = behavior
// 		}
//
// 		newState, events := behavior.Update(monster, m.Ctx)
// 		totalEvents = append(totalEvents, events...)
// 		if newState != monster.CurState {
// 			monster.CurState = newState
// 			newBehavior, exists := m.Behaviors[newState]
// 			if !exists {
// 				log.Fatalf("State %v is not registered\n", newState)
// 			}
// 			m.Monsters[monster.Id] = newBehavior.Clone()
// 		}
//
// 		for _, event := range events {
// 			event.Perform(m.Session, m.Ctx.Rng)
// 		}
//
// 		attempt++
// 	}
//
// 	return totalEvents
// }
//
// func (m *MonsterLogic) CanMakeMove(actor *model.Actor) bool {
// 	findPoint := m.Ctx.Move.Moves.GetFindPointFunc(actor.MovePattern)
// 	searchLen := 1
//
// 	// Ghost move distance is determined by its hostility value
// 	if actor.MovePattern == model.MovePatternTeleport {
// 		searchLen = actor.DerivedAttrs[model.AttrHostility]
// 	}
//
// 	// If the actor is surrounded by other actor but can only move
// 	_, found := findPoint(actor, searchLen, m.Session.Map, true) // Find empty point
// 	if actor.CanMove() && !actor.CanAttack() && !found {
// 		return false
// 	}
//
// 	// If there are no other actors near the actor, but he can only attack
// 	_, found = findPoint(actor, searchLen, m.Session.Map, false) // Find occupied point
// 	if !actor.CanMove() && actor.CanAttack() && !found {
// 		return false
// 	}
//
// 	// Standard case
// 	if !actor.CanMove() && !actor.CanAttack() {
// 		return false
// 	}
//
// 	return true
// }
