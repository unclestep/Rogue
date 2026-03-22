package service_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func createMoveEnv() (*model.SessionContext, *service.Movement, *model.Actor) {
	play := model.NewPlaythrough("", 0, 1)
	ctx := model.NewSessionContext(play)
	topGen := service.NewTopologyGenerator()
	topGen.Gen(ctx, 80, 24, 3, 3)

	impactResolver := service.NewImpactResolverService()
	pickup := service.NewPickupService()
	movement := service.NewMovementService(impactResolver, pickup)

	mover := model.NewDefaultPlayer(1, ctx.Playthrough.Map.GetEntranceRoom().Center, 9)
	play.Players[mover.Id] = mover
	play.Map.SetActor(mover.Pos, int64(mover.Id))

	return ctx, movement, mover
}

func TestExecuteMove(t *testing.T) {
	ctx, moveService, mover := createMoveEnv()
	vector := geometry.Point{X: 1, Y: 0}

	t.Run("Mechanics: Stamina Cost and PosChange", func(t *testing.T) {
		events := moveService.ExecuteMove(ctx, mover, vector)

		event, ok := events[0].(*service.MoveEvent)
		if !ok {
			t.Fatalf("Expected first event to be *MoveEvent, got %T\n", events[0])
		}

		if event.Outcome != service.MoveOutcomeSuccess {
			t.Errorf("Expected Success, got %v", event.Outcome)
		}

		expectedVec := geometry.Point{X: 1, Y: 0}
		if event.Mover.PosChange != expectedVec {
			t.Errorf("Expected PosChange %v, got %v", expectedVec, event.Mover.PosChange)
		}

		cost := mover.DerivedAttrs[model.AttrMoveStaminaCost]
		if event.Mover.VitalsChange[model.VitalStamina] != -cost {
			t.Errorf("Expected stamina change %d, got %d", -cost, event.Mover.VitalsChange[model.VitalStamina])
		}
	})

	t.Run("Mechanics: Not Enough Stamina", func(t *testing.T) {
		mover.Vitals[model.VitalStamina] = 0

		events := moveService.ExecuteMove(ctx, mover, vector)

		event, ok := events[0].(*service.MoveEvent)
		if !ok {
			t.Fatalf("Expected first event to be *MoveEvent, got %T\n", events[0])
		}

		if event.Outcome != service.MoveOutcomeNoStamina {
			t.Errorf("Expected MoveOutcomeNoStamina, got %v", event.Outcome)
		}
	})

	t.Run("Mechanics: Status Prevent Move", func(t *testing.T) {
		mover.Vitals[model.VitalStamina] = 200
		mover.Statuses[model.StatusSleep] = 1

		events := moveService.ExecuteMove(ctx, mover, vector)

		event, ok := events[0].(*service.MoveEvent)
		if !ok {
			t.Fatalf("Expected first event to be *MoveEvent, got %T\n", events[0])
		}

		if event.Outcome != service.MoveOutcomeCantMove {
			t.Errorf("Expected MoveOutcomeCantMove (Sleep), got %v", event.Outcome)
		}
	})

	t.Run("Triggers: TriggerOnMove", func(t *testing.T) {
		mover.Statuses[model.StatusSleep] = 0
		mover.Vitals[model.VitalStamina] = 200

		healOnMove := &model.Reaction{
			Trigger: model.TriggerOnMove,
			Target:  model.TargetSource,
			Chance:  model.Guaranteed,
			VitalsChange: map[model.VitalType]model.Change{
				model.VitalHP: {
					Holder: model.TargetSource,
					Vital:  model.VitalHP,
					Amount: 5,
				},
			},
		}
		mover.Traits[model.TriggerOnMove] = []*model.Reaction{healOnMove}

		events := moveService.ExecuteMove(ctx, mover, vector)

		event, ok := events[0].(*service.MoveEvent)
		if !ok {
			t.Fatalf("Expected first event to be *MoveEvent, got %T\n", events[0])
		}

		if event.Outcome != service.MoveOutcomeSuccess {
			t.Errorf("Expected Success")
		}

		if event.Mover.VitalsChange[model.VitalHP] != 5 {
			t.Errorf("Expected TriggerOnMove to add +5 HP, got %d", event.Mover.VitalsChange[model.VitalHP])
		}
	})

	t.Run("Item pickup", func(t *testing.T) {
		item := model.NewDefaultFood(2, mover.Pos.Add(geometry.Point{X: 1, Y: 0}))
		ctx.Playthrough.AddItem(item)

		events := moveService.ExecuteMove(ctx, mover, vector)

		move, ok := events[0].(*service.MoveEvent)
		if !ok {
			t.Fatalf("Expected first event to be *MoveEvent, got %T\n", events[0])
		}

		if move.Outcome != service.MoveOutcomeSuccess {
			t.Errorf("Expected Success move, got %v", move.Outcome)
		}

		expectedVec := geometry.Point{X: 1, Y: 0}
		if move.Mover.PosChange != expectedVec {
			t.Errorf("Expected PosChange %v, got %v", expectedVec, move.Mover.PosChange)
		}

		pickup, ok := events[1].(*service.ItemPickupEvent)
		if !ok {
			t.Fatalf("Expected second event to be *ItemPickupEvent, got %T\n", events[1])
		}

		if pickup.Outcome != service.PickupOutcomeSuccess {
			t.Errorf("Expected Success pickup, got %v", pickup.Outcome)
		}

		if pickup.PickupItem != item {
			t.Errorf("Expected pickup item %v, got %v", item, pickup.PickupItem)
		}
	})
}

func TestMoveEventPerform(t *testing.T) {
	ctx, _, mover := createMoveEnv()
	startPos := mover.Pos

	t.Run("Perform updates Map and Actor", func(t *testing.T) {
		vector := geometry.Point{X: 1, Y: 0}
		targetPos := startPos.Add(vector)

		event := service.NewMoveEvent(mover, service.NewImpactResolverService())
		event.Outcome = service.MoveOutcomeSuccess
		event.Mover.PosChange = vector

		event.Perform(ctx)

		if mover.Pos != targetPos {
			t.Errorf("Actor position is incorrect. Got %v, want %v", mover.Pos, targetPos)
		}

		if ctx.Playthrough.Map.IsActor(startPos) {
			t.Error("Old position on Map should be free")
		}
		if !ctx.Playthrough.Map.IsActor(targetPos) {
			t.Error("New position on Map should be occupied")
		}

		id, _ := ctx.Playthrough.Map.GetActorID(targetPos)
		if model.ActorId(id) != mover.Id {
			t.Errorf("Map has wrong ActorID at new pos. Got %d, want %d", id, mover.Id)
		}
	})

	t.Run("Players can exit", func(t *testing.T) {
		mover.Pos = startPos
		ctx.Playthrough.Map.SetActor(mover.Pos, int64(mover.Id))
		targetPos := ctx.Playthrough.Map.ExitPoint
		vector := targetPos.Sub(startPos)

		event := service.NewMoveEvent(mover, service.NewImpactResolverService())
		event.Outcome = service.MoveOutcomeSuccess
		event.Mover.PosChange = vector

		event.Perform(ctx)

		if ctx.Playthrough.Map.IsActor(startPos) {
			t.Error("Old position on Map should be free")
		}
		if ctx.Playthrough.Map.IsActor(targetPos) {
			t.Error("New position on Map should not be occupied - Exit")
		}

		if mover.Pos != model.NewInvalidPoint() {
			t.Errorf("Player should be hidden after reaching ExitPoint. Got %v, want %v", mover.Pos, model.NewInvalidPoint())
		}

		if _, exists := ctx.Playthrough.Players[mover.Id]; !exists {
			t.Errorf("Player should exist in Playthrough map after reaching ExitPoint")
		}
	})

	t.Run("Monsters can't exit", func(t *testing.T) {
		mover.Pos = startPos
		ctx.Playthrough.Map.SetActor(mover.Pos, int64(mover.Id))
		mover.Kind = model.ActorMimic
		delete(ctx.Playthrough.Players, mover.Id)
		ctx.Playthrough.Monsters[mover.Id] = mover

		targetPos := ctx.Playthrough.Map.ExitPoint
		vector := targetPos.Sub(startPos)

		event := service.NewMoveEvent(mover, service.NewImpactResolverService())
		event.Outcome = service.MoveOutcomeSuccess
		event.Mover.PosChange = vector

		event.Perform(ctx)

		if mover.Pos != targetPos {
			t.Errorf("Actor position incorrect. Got %v, want %v", mover.Pos, targetPos)
		}

		if ctx.Playthrough.Map.IsActor(startPos) {
			t.Error("Old position on Map should be free")
		}
		if !ctx.Playthrough.Map.IsActor(targetPos) {
			t.Errorf("Exit on Map should be occupied by monster. Got: %v, expected: %v", mover.Pos, targetPos)
		}

		if mover.Pos == model.NewInvalidPoint() {
			t.Errorf("Monster should NOT be hidden after reaching ExitPoint. Got %v, want %v", mover.Pos, targetPos)
		}

		if _, exists := ctx.Playthrough.Monsters[mover.Id]; !exists {
			t.Errorf("Monster should exist in Playthrough map after reaching ExitPoint")
		}
	})
}
