// package service_test
//
// import (
// 	"math/rand"
// 	"testing"
// 	"time"
//
// 	"github.com/unclestep/Rogue/internal/domain/model"
// 	"github.com/unclestep/Rogue/internal/domain/service"
// 	"github.com/unclestep/Rogue/pkg/geometry"
// )
//
// func createMoveEnv() (*model.GameSession, *service.Move, *model.Actor) {
// 	var seed int64 = 2
// 	rng := rand.New(rand.NewSource(seed))
//
// 	session := model.NewGameSession()
// 	session.Map.GenerateTopology(1, 1, rng)
//
// 	center := session.Map.Rooms[0].Center
// 	mover := model.NewDefaultPlayer(1, center)
// 	session.Monsters[mover.Id] = mover
// 	session.Map.SetActor(center, 1)
//
// 	moveService := service.NewMoveService(session, time.Now().UnixNano())
// 	moveService.SetSeed(seed)
//
// 	return session, moveService, mover
// }
//
// func TestMovePatterns(t *testing.T) {
// 	session, moveService, mover := createMoveEnv()
// 	m := session.Map
//
// 	t.Run("Default Move One Possible Move", func(t *testing.T) {
// 		target := mover.Pos.Add(geometry.Point{X: 1, Y: 0})
// 		scentMap := m.GenerateScentMap([]geometry.Point{target})
//
// 		nextPos := moveService.DefaultMove(mover, scentMap, m)
//
// 		if nextPos != target {
// 			t.Errorf("Expected nextPos: %v, got %v\n", target, nextPos)
// 		}
// 	})
//
// 	t.Run("Default Move Two Possible Moves", func(t *testing.T) {
// 		target := mover.Pos.Add(geometry.Point{X: 1, Y: 1})
// 		expected1 := mover.Pos.Add(geometry.Point{X: 1, Y: 0})
// 		expected2 := mover.Pos.Add(geometry.Point{X: 0, Y: 1})
//
// 		scentMap := m.GenerateScentMap([]geometry.Point{target})
//
// 		nextPos := moveService.DefaultMove(mover, scentMap, m)
//
// 		if nextPos != expected1 && nextPos != expected2 {
// 			t.Errorf("Expected nextPos: %v or %v, got %v\n", expected1, expected2, nextPos)
// 		}
// 	})
//
// 	t.Run("Diagonal Move", func(t *testing.T) {
// 		target := mover.Pos.Add(geometry.Point{X: 1, Y: 1})
// 		scentMap := m.GenerateScentMap([]geometry.Point{target})
//
// 		nextPos := moveService.DiagonalMove(mover, scentMap, m)
//
// 		if nextPos != target {
// 			t.Errorf("DiagonalMove should verify diagonal step from %v directly to %v, got %v", mover.Pos, target, nextPos)
// 		}
// 	})
//
// 	t.Run("Diagonal Move Cannot Move Directly", func(t *testing.T) {
// 		target := mover.Pos.Add(geometry.Point{X: 1, Y: 0})
// 		expected1 := mover.Pos.Add(geometry.Point{X: 1, Y: 1})
// 		expected2 := mover.Pos.Add(geometry.Point{X: 1, Y: -1})
//
// 		scentMap := m.GenerateScentMap([]geometry.Point{target})
//
// 		nextPos := moveService.DiagonalMove(mover, scentMap, m)
//
// 		if nextPos != expected1 && nextPos != expected2 {
// 			t.Errorf("Expected nextPos: %v or %v, got %v\n", expected1, expected2, nextPos)
// 		}
// 	})
//
// 	t.Run("Teleport Move: Jump over obstacles", func(t *testing.T) {
// 		mover.DerivedAttrs[model.AttrHostility] = 5
//
// 		target := mover.Pos.Add(geometry.Point{X: 3, Y: 0})
//
// 		scentMap := m.GenerateScentMap([]geometry.Point{target})
//
// 		nextPos := moveService.TeleportMove(mover, scentMap, m)
//
// 		if nextPos != target {
// 			t.Errorf("TeleportMove should jump to %v, got %v", target, nextPos)
// 		}
// 	})
// }
//
// func TestExecuteMove(t *testing.T) {
// 	session, moveService, mover := createMoveEnv()
//
// 	m := session.Map
//
// 	target := mover.Pos.Add(geometry.Point{X: 1, Y: 0})
// 	scentMap := m.GenerateScentMap([]geometry.Point{target})
//
// 	t.Run("Mechanics: Stamina Cost and PosChange", func(t *testing.T) {
// 		events := moveService.ExecuteMove(mover, scentMap, m)
//
// 		event, ok := events[0].(*service.MoveEvent)
// 		if !ok {
// 			t.Fatalf("Expected first event to be *MoveEvent, got %T\n", events[0])
// 		}
//
// 		if event.Outcome != service.MoveOutcomeSuccess {
// 			t.Errorf("Expected Success, got %v", event.Outcome)
// 		}
//
// 		expectedVec := geometry.Point{X: 1, Y: 0}
// 		if event.Mover.PosChange != expectedVec {
// 			t.Errorf("Expected PosChange %v, got %v", expectedVec, event.Mover.PosChange)
// 		}
//
// 		cost := mover.DerivedAttrs[model.AttrMoveStaminaCost]
// 		if event.Mover.VitalsChange[model.VitalStamina] != -cost {
// 			t.Errorf("Expected stamina change %d, got %d", -cost, event.Mover.VitalsChange[model.VitalStamina])
// 		}
// 	})
//
// 	t.Run("Mechanics: Not Enough Stamina", func(t *testing.T) {
// 		mover.Vitals[model.VitalStamina] = 0
//
// 		events := moveService.ExecuteMove(mover, scentMap, m)
//
// 		event, ok := events[0].(*service.MoveEvent)
// 		if !ok {
// 			t.Fatalf("Expected first event to be *MoveEvent, got %T\n", events[0])
// 		}
//
// 		if event.Outcome != service.MoveOutcomeNoStamina {
// 			t.Errorf("Expected MoveOutcomeNoStamina, got %v", event.Outcome)
// 		}
// 	})
//
// 	t.Run("Mechanics: Status Prevent Move", func(t *testing.T) {
// 		mover.Vitals[model.VitalStamina] = 200
// 		mover.Statuses[model.StatusSleep] = 1
//
// 		events := moveService.ExecuteMove(mover, scentMap, m)
//
// 		event, ok := events[0].(*service.MoveEvent)
// 		if !ok {
// 			t.Fatalf("Expected first event to be *MoveEvent, got %T\n", events[0])
// 		}
//
// 		if event.Outcome != service.MoveOutcomeCantMove {
// 			t.Errorf("Expected MoveOutcomeCantMove (Sleep), got %v", event.Outcome)
// 		}
// 	})
//
// 	t.Run("Triggers: TriggerOnMove", func(t *testing.T) {
// 		mover.Statuses[model.StatusSleep] = 0
// 		mover.Vitals[model.VitalStamina] = 200
//
// 		healOnMove := &model.Reaction{
// 			Trigger: model.TriggerOnMove,
// 			Target:  model.TargetSource,
// 			Chance:  model.Guaranteed,
// 			VitalsChange: map[model.VitalType]model.Change{
// 				model.VitalHP: {
// 					Holder: model.TargetSource,
// 					Vital:  model.VitalHP,
// 					Amount: 5,
// 				},
// 			},
// 		}
// 		mover.Traits[model.TriggerOnMove] = []*model.Reaction{healOnMove}
//
// 		events := moveService.ExecuteMove(mover, scentMap, m)
//
// 		event, ok := events[0].(*service.MoveEvent)
// 		if !ok {
// 			t.Fatalf("Expected first event to be *MoveEvent, got %T\n", events[0])
// 		}
//
// 		if event.Outcome != service.MoveOutcomeSuccess {
// 			t.Errorf("Expected Success")
// 		}
//
// 		if event.Mover.VitalsChange[model.VitalHP] != 5 {
// 			t.Errorf("Expected TriggerOnMove to add +5 HP, got %d", event.Mover.VitalsChange[model.VitalHP])
// 		}
// 	})
// }
//
// func TestMoveEventPerform(t *testing.T) {
// 	session, _, mover := createMoveEnv()
// 	rng := rand.New(rand.NewSource(1))
//
// 	t.Run("Perform updates Map and Actor", func(t *testing.T) {
// 		startPos := mover.Pos
// 		moveVec := geometry.Point{X: 1, Y: 0}
// 		targetPos := startPos.Add(moveVec)
//
// 		event := service.NewMoveEvent(mover)
// 		event.Outcome = service.MoveOutcomeSuccess
// 		event.Mover.PosChange = moveVec
//
// 		event.Perform(session, rng)
//
// 		if mover.Pos != targetPos {
// 			t.Errorf("Actor struct Pos not updated. Got %v, want %v", mover.Pos, targetPos)
// 		}
//
// 		if session.Map.IsActor(startPos) {
// 			t.Error("Old position on Map should be free")
// 		}
// 		if !session.Map.IsActor(targetPos) {
// 			t.Error("New position on Map should be occupied")
// 		}
//
// 		id, _ := session.Map.GetActorID(targetPos)
// 		if model.ActorId(id) != mover.Id {
// 			t.Errorf("Map has wrong ActorID at new pos. Got %d, want %d", id, mover.Id)
// 		}
// 	})
// }
