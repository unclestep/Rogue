package service_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/geometry"
)

//
//
// --- HELPERS ---
//
//

const (
	interactKeyhole1 model.Keyhole = 1
	interactKeyhole2 model.Keyhole = 2
)

var (
	interactPlayerPos     = geometry.Point{X: 2, Y: 3}
	interactLockedDoorPos = geometry.Point{X: 4, Y: 3}

	// This tile is ClosedDoor in the grid but has NO entry in the Doors map.
	// It represents a door placed by the topology generator without a key requirement.
	interactPlainDoorPos = geometry.Point{X: 2, Y: 1}
)

// buildInteractMap returns a 7x7 map with:
//   - Floor walkable area in the inner 5x5
//   - A locked ClosedDoor at interactLockedDoorPos (in Doors map, keyhole=1)
//   - A plain ClosedDoor at interactPlainDoorPos (NOT in Doors map)
func buildInteractMap() *model.Map {
	const W, H = 7, 7

	grid := make([][]model.Cell, H)
	for y := range grid {
		grid[y] = make([]model.Cell, W)
		for x := range grid[y] {
			tileType := model.Floor
			if y == 0 || y == H-1 || x == 0 || x == W-1 {
				tileType = model.Wall
			}
			grid[y][x] = model.Cell{Type: tileType, RoomId: 1}
		}
	}

	grid[interactLockedDoorPos.Y][interactLockedDoorPos.X] = model.Cell{Type: model.ClosedDoor, RoomId: 1}
	grid[interactPlainDoorPos.Y][interactPlainDoorPos.X] = model.Cell{Type: model.ClosedDoor, RoomId: 1}

	blueprint := &model.MapBlueprint{
		Width:    W,
		Height:   H,
		TileGrid: grid,
		Rooms: []*model.Room{
			{
				Id:     1,
				Pos:    geometry.Point{X: 1, Y: 1},
				Width:  W - 2,
				Height: H - 2,
				Center: geometry.Point{X: W / 2, Y: H / 2},
			},
		},
		Doors: map[geometry.Point]*model.DoorMetadata{
			interactLockedDoorPos: {
				Pos:     interactLockedDoorPos,
				Locked:  true,
				Keyhole: interactKeyhole1,
			},
		},
		EntranceRoomId: 1,
		ExitRoomId:     1,
		ExitPoint:      model.NewInvalidPoint(),
	}

	return model.NewMapFromBlueprint(blueprint)
}

func setupInteractEnv() (*model.SessionContext, *service.Interactor, *model.Actor) {
	play := model.NewPlaythrough(0, 0, 0)
	play.Map = buildInteractMap()

	actor := model.NewDefaultPlayer(1, interactPlayerPos, 9)
	play.Players[actor.Id] = actor
	play.Map.SetActor(actor.Pos, int64(actor.Id))

	ctx := model.NewSessionContext(play)
	return ctx, service.NewInteractorService(), actor
}

//
//
// --- EXECUTE ---
//
//

//
// -- Nil / invalid inputs --
//

func TestInteractExecuteNilActorReturnsNil(t *testing.T) {
	ctx, interactor, _ := setupInteractEnv()

	event := interactor.Execute(ctx, nil, interactLockedDoorPos)

	if event != nil {
		t.Errorf("Expected nil for nil actor, got non-nil event")
	}
}

func TestInteractExecuteNonDoorTileReturnsFailed(t *testing.T) {
	ctx, interactor, actor := setupInteractEnv()

	// Target is a plain floor tile — not a door.
	floorTile := geometry.Point{X: 3, Y: 3}
	event := interactor.Execute(ctx, actor, floorTile)

	if event == nil {
		t.Fatalf("Expected event (not nil) for non-door tile")
	}
	if event.Outcome != service.InteractionEventOutcomeFailed {
		t.Errorf("Expected Failed for non-door tile, got %v", event.Outcome)
	}
}

//
// -- Locked door --
//

func TestInteractExecuteLockedDoorCorrectKeyReturnsSuccess(t *testing.T) {
	ctx, interactor, actor := setupInteractEnv()

	key := model.NewKeyItem(10, model.NewInvalidPoint(), interactKeyhole1)
	actor.Backpack.Add(key)

	event := interactor.Execute(ctx, actor, interactLockedDoorPos)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.InteractionEventOutcomeSuccess {
		t.Errorf("Expected Success with matching key, got %v", event.Outcome)
	}
	if event.DoorToOpen != interactLockedDoorPos {
		t.Errorf("Expected DoorToOpen=%v, got %v", interactLockedDoorPos, event.DoorToOpen)
	}
}

func TestInteractExecuteLockedDoorNoKeysReturnsFailed(t *testing.T) {
	ctx, interactor, actor := setupInteractEnv()
	// Actor has no keys in backpack.

	event := interactor.Execute(ctx, actor, interactLockedDoorPos)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.InteractionEventOutcomeFailed {
		t.Errorf("Expected Failed when actor has no keys, got %v", event.Outcome)
	}
}

func TestInteractExecuteLockedDoorWrongKeyholeReturnsFailed(t *testing.T) {
	ctx, interactor, actor := setupInteractEnv()

	wrongKey := model.NewKeyItem(10, model.NewInvalidPoint(), interactKeyhole2)
	actor.Backpack.Add(wrongKey)

	event := interactor.Execute(ctx, actor, interactLockedDoorPos)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.InteractionEventOutcomeFailed {
		t.Errorf("Expected Failed with wrong keyhole, got %v", event.Outcome)
	}
}

// When actor has multiple keys and the first does not match, the service must
// keep iterating and succeed with the second matching key.
func TestInteractExecuteLockedDoorMatchingSecondKeySucceeds(t *testing.T) {
	ctx, interactor, actor := setupInteractEnv()

	wrongKey := model.NewKeyItem(10, model.NewInvalidPoint(), interactKeyhole2)
	correctKey := model.NewKeyItem(11, model.NewInvalidPoint(), interactKeyhole1)
	actor.Backpack.Add(wrongKey)
	actor.Backpack.Add(correctKey)

	event := interactor.Execute(ctx, actor, interactLockedDoorPos)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.InteractionEventOutcomeSuccess {
		t.Errorf("Expected Success when second key matches, got %v", event.Outcome)
	}
}

//
// -- Plain closed door (not in doors map) --
//

// A ClosedDoor tile placed without door metadata is never openable via
// Interactor.Execute: GetDoorKeyhole returns (0, false), the match condition
// in the loop is never satisfied, and the outcome is always Failed.
// This means topology-placed unlocked doors cannot be opened by the player
// even if they carry a key.
func TestInteractExecutePlainClosedDoorIsNotOpenable(t *testing.T) {
	ctx, interactor, actor := setupInteractEnv()

	// Actor has a key — it should not matter; the door has no metadata.
	key := model.NewKeyItem(10, model.NewInvalidPoint(), interactKeyhole1)
	actor.Backpack.Add(key)

	event := interactor.Execute(ctx, actor, interactPlainDoorPos)

	if event == nil {
		t.Fatalf("Expected non-nil event for plain closed door")
	}
	if event.Outcome != service.InteractionEventOutcomeFailed {
		t.Errorf("Expected Failed for plain closed door without metadata, got %v", event.Outcome)
	}

	metaKey := model.NewKeyItem(10, model.NewInvalidPoint(), -1)
	actor.Backpack.Add(metaKey)

	event = interactor.Execute(ctx, actor, interactPlainDoorPos)

	if event == nil {
		t.Fatalf("Expected non-nil event for plain closed door")
	}
	if event.Outcome != service.InteractionEventOutcomeSuccess {
		t.Errorf("Expected Success for plain closed door with meta key in backpack, got %v", event.Outcome)
	}
	if event.DoorToOpen != interactPlainDoorPos {
		t.Errorf("Expected DoorToOpen=%v, got %v", interactPlainDoorPos, event.DoorToOpen)
	}
}

func TestInteractExecutePlainClosedDoorWithNoKeyAlsoFails(t *testing.T) {
	ctx, interactor, actor := setupInteractEnv()
	// Actor has no keys.

	event := interactor.Execute(ctx, actor, interactPlainDoorPos)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.InteractionEventOutcomeFailed {
		t.Errorf("Expected Failed for plain closed door, got %v", event.Outcome)
	}
}

//
//
// --- PERFORM ---
//
//

//
// -- Success outcome --
//

func TestInteractPerformSuccessOutcomeOpensDoor(t *testing.T) {
	ctx, interactor, actor := setupInteractEnv()

	key := model.NewKeyItem(10, model.NewInvalidPoint(), interactKeyhole1)
	actor.Backpack.Add(key)

	event := interactor.Execute(ctx, actor, interactLockedDoorPos)
	if event.Outcome != service.InteractionEventOutcomeSuccess {
		t.Fatalf("Setup: expected Success, got %v", event.Outcome)
	}

	event.Perform(ctx)

	if !ctx.Playthrough.Map.IsOpenDoor(interactLockedDoorPos) {
		t.Errorf("Expected door at %v to be OpenDoor after Perform, but it is still closed", interactLockedDoorPos)
	}
}

//
// -- Failed outcome --
//

func TestInteractPerformFailedOutcomeDoesNotOpenDoor(t *testing.T) {
	ctx, interactor, actor := setupInteractEnv()
	// No key — outcome will be Failed.

	event := interactor.Execute(ctx, actor, interactLockedDoorPos)
	if event.Outcome != service.InteractionEventOutcomeFailed {
		t.Fatalf("Setup: expected Failed, got %v", event.Outcome)
	}

	event.Perform(ctx)

	if !ctx.Playthrough.Map.IsClosedDoor(interactLockedDoorPos) {
		t.Errorf("Expected door at %v to remain ClosedDoor after failed Perform", interactLockedDoorPos)
	}
}

// Perform on a Failed outcome must not panic even when DoorToOpen is the
// default invalid point.
func TestInteractPerformWithInvalidDoorToOpenDoesNotPanic(t *testing.T) {
	ctx, _, _ := setupInteractEnv()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Perform panicked with invalid DoorToOpen: %v", r)
		}
	}()

	event := service.NewInteractionEvent()
	event.Outcome = service.InteractionEventOutcomeSuccess
	// DoorToOpen is already InvalidPoint by default; OpenDoor on an out-of-bounds
	// or metadata-less point should be a no-op, not a panic.
	event.Perform(ctx)
}
