package service_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/geometry"
)

//
// --- HELPERS ---
//

//
// -- Map builder --
//

// buildResolverMap creates a deterministic 7x7 map.
// Inner 5x5 area is Floor. Border tiles are Wall.
// All tiles share RoomId=1 so CanMoveTo never panics on out-of-room checks.
// A ClosedDoor is placed at doorPos=(3,3).
//
// Layout (. = Floor, # = Wall, % = ClosedDoor):
//
//	#######
//	#.....#
//	#.....#
//	#..%..#
//	#.....#
//	#.....#
//	#######
func buildResolverMap() *model.Map {
	const (
		width  = 7
		height = 7
	)

	doorPos := geometry.Point{X: 3, Y: 3}

	grid := make([][]model.Cell, height)
	for y := range grid {
		grid[y] = make([]model.Cell, width)
		for x := range grid[y] {
			tileType := model.Floor
			if y == 0 || y == height-1 || x == 0 || x == width-1 {
				tileType = model.Wall
			}
			if x == doorPos.X && y == doorPos.Y {
				tileType = model.ClosedDoor
			}
			// All tiles share RoomId=1 so GetRoomByPoint always returns a
			// non-nil room, preventing panics inside CanMoveTo.
			grid[y][x] = model.Cell{Type: tileType, RoomId: 1}
		}
	}

	blueprint := &model.MapBlueprint{
		Width:    width,
		Height:   height,
		TileGrid: grid,
		Rooms: []*model.Room{
			{
				Id:     1,
				Pos:    geometry.Point{X: 0, Y: 0},
				Width:  width,
				Height: height,
				Center: geometry.Point{X: width / 2, Y: height / 2},
			},
		},
		Doors: map[geometry.Point]*model.DoorMetadata{
			doorPos: {Pos: doorPos},
		},
		EntranceRoomId: 1,
		ExitRoomId:     1,
		ExitPoint:      model.NewInvalidPoint(),
	}

	return model.NewMapFromBlueprint(blueprint)
}

func setupResolverEnv() (*model.Playthrough, *service.MoveResolver, *model.Actor) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildResolverMap()

	resolver := service.NewMoveResolverService()

	mover := model.NewDefaultPlayer(1, geometry.Point{X: 2, Y: 2}, 9)
	play.Players[mover.Id] = mover
	play.Map.SetActor(mover.Pos, int64(mover.Id))

	return play, resolver, mover
}

//
// --- ATTACK HANDLER ---
//

//
// -- Normal cases --
//

func TestResolveAttackHandlerMonsterOnTargetReturnsIntentAttack(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	monster := model.NewDefaultZombie(2, geometry.Point{X: 3, Y: 2})
	play.Monsters[monster.Id] = monster
	play.Map.SetActor(monster.Pos, int64(monster.Id))

	vector := geometry.Point{X: 1, Y: 0}
	intent := resolver.Resolve(play, mover, vector)

	if intent == nil {
		t.Fatalf("Expected IntentAttack, got nil")
	}
	if intent.IntentType != model.IntentAttack {
		t.Errorf("Expected IntentAttack, got %v", intent.IntentType)
	}
	if intent.Defender != monster.Id {
		t.Errorf("Expected Defender=%d, got %d", monster.Id, intent.Defender)
	}
	if intent.Actor != mover.Id {
		t.Errorf("Expected Actor=%d, got %d", mover.Id, intent.Actor)
	}
}

func TestResolveAttackHandlerPlayerOnTargetReturnsIntentAttack(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	ally := model.NewDefaultPlayer(3, geometry.Point{X: 3, Y: 2}, 9)
	play.Players[ally.Id] = ally
	play.Map.SetActor(ally.Pos, int64(ally.Id))

	vector := geometry.Point{X: 1, Y: 0}
	intent := resolver.Resolve(play, mover, vector)

	if intent == nil {
		t.Fatalf("Expected IntentAttack against player on target tile, got nil")
	}
	if intent.IntentType != model.IntentAttack {
		t.Errorf("Expected IntentAttack, got %v", intent.IntentType)
	}
	if intent.Defender != ally.Id {
		t.Errorf("Expected Defender=%d, got %d", ally.Id, intent.Defender)
	}
}

//
// -- Edge cases --
//

func TestResolveAttackHandlerNoActorOnTargetDoesNotReturnIntentAttack(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	// No actor placed at (3,2).
	vector := geometry.Point{X: 1, Y: 0}
	intent := resolver.Resolve(play, mover, vector)

	if intent != nil && intent.IntentType == model.IntentAttack {
		t.Errorf("Expected AttackHandler to skip when target tile is empty, got IntentAttack")
	}
}

func TestResolveAttackHandlerZeroVectorTargetsSelf(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	// Vector (0,0) points to the actor's own tile.
	// GetActorID returns mover's ID (> 0), so AttackHandler fires.
	intent := resolver.Resolve(play, mover, geometry.Point{X: 0, Y: 0})

	if intent == nil {
		t.Fatalf("Expected IntentAttack with self as Defender when vector is (0,0), got nil")
	}
	if intent.IntentType != model.IntentAttack {
		t.Errorf("Expected IntentAttack, got %v", intent.IntentType)
	}
	if intent.Defender != mover.Id {
		t.Errorf("Expected self-target Defender=%d, got %d", mover.Id, intent.Defender)
	}
}

func TestResolveMoveHandlerTileWithItemIsWalkable(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	// Items do not block actor movement: CanMoveTo checks actorGrid, not
	// itemGrid. A tile that has only an item should still resolve to IntentMove.
	item := model.NewDefaultFood(10, geometry.Point{X: 3, Y: 2})
	play.Items[item.Id] = item
	play.Map.SetItem(item.Pos, int64(item.Id))

	vector := geometry.Point{X: 1, Y: 0}
	intent := resolver.Resolve(play, mover, vector)

	if intent == nil {
		t.Fatalf("Expected IntentMove when target tile holds only an item, got nil")
	}
	if intent.IntentType != model.IntentMove {
		t.Errorf("Expected IntentMove, got %v", intent.IntentType)
	}
}

//
// --- DOOR BUMP HANDLER ---
//

//
// -- Normal cases --
//

func TestResolveDoorBumpHandlerClosedDoorReturnsIntentInteract(t *testing.T) {
	// Mover at (2,3), door at (3,3). Map door is at (3,3) by default.
	play, resolver, mover := setupResolverEnv()

	mover.Pos = geometry.Point{X: 2, Y: 3}
	play.Map.RemoveActor(geometry.Point{X: 2, Y: 2})
	play.Map.SetActor(mover.Pos, int64(mover.Id))

	vector := geometry.Point{X: 1, Y: 0}
	intent := resolver.Resolve(play, mover, vector)

	if intent == nil {
		t.Fatalf("Expected IntentInteract for closed door, got nil")
	}
	if intent.IntentType != model.IntentInteract {
		t.Errorf("Expected IntentInteract, got %v", intent.IntentType)
	}
	if intent.Actor != mover.Id {
		t.Errorf("Expected Actor=%d, got %d", mover.Id, intent.Actor)
	}
	if intent.Vector != vector {
		t.Errorf("Expected Vector=%v, got %v", vector, intent.Vector)
	}
}

//
// -- Edge cases --
//

func TestResolveDoorBumpHandlerOpenDoorIsNotInteracted(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	// Open the door so it becomes walkable.
	doorPos := geometry.Point{X: 3, Y: 3}
	play.Map.OpenDoor(doorPos)

	mover.Pos = geometry.Point{X: 2, Y: 3}
	play.Map.RemoveActor(geometry.Point{X: 2, Y: 2})
	play.Map.SetActor(mover.Pos, int64(mover.Id))

	vector := geometry.Point{X: 1, Y: 0}
	intent := resolver.Resolve(play, mover, vector)

	if intent != nil && intent.IntentType == model.IntentInteract {
		t.Errorf("Expected DoorBumpHandler to skip open door, got IntentInteract")
	}
}

func TestResolveDoorBumpHandlerPlainFloorTileIsNotInteracted(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	// Target (3,2) is plain Floor with no actor, no door.
	vector := geometry.Point{X: 1, Y: 0}
	intent := resolver.Resolve(play, mover, vector)

	if intent != nil && intent.IntentType == model.IntentInteract {
		t.Errorf("Expected DoorBumpHandler to skip plain floor tile, got IntentInteract")
	}
}

//
// --- MOVE HANDLER ---
//

//
// -- Normal cases --
//

func TestResolveMoveHandlerWalkableFreeFloorReturnsIntentMove(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	// (3,2) is a plain Floor with no actor.
	vector := geometry.Point{X: 1, Y: 0}
	intent := resolver.Resolve(play, mover, vector)

	if intent == nil {
		t.Fatalf("Expected IntentMove for free walkable floor, got nil")
	}
	if intent.IntentType != model.IntentMove {
		t.Errorf("Expected IntentMove, got %v", intent.IntentType)
	}
	if intent.Actor != mover.Id {
		t.Errorf("Expected Actor=%d, got %d", mover.Id, intent.Actor)
	}
	if intent.Vector != vector {
		t.Errorf("Expected Vector=%v, got %v", vector, intent.Vector)
	}
}

//
// -- Edge cases --
//

func TestResolveMoveHandlerOccupiedFloorReturnsNil(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	// Place another actor on (3,2) — CanMoveTo returns false.
	blocker := model.NewDefaultZombie(5, geometry.Point{X: 3, Y: 2})
	play.Monsters[blocker.Id] = blocker
	play.Map.SetActor(blocker.Pos, int64(blocker.Id))

	// AttackHandler fires because actor is on the target tile.
	// This test is documented here to clarify that MoveHandler is bypassed,
	// not that MoveHandler independently returns nil.
	vector := geometry.Point{X: 1, Y: 0}
	intent := resolver.Resolve(play, mover, vector)

	if intent == nil {
		t.Fatalf("Expected intent for occupied tile (AttackHandler fires), got nil")
	}
	if intent.IntentType == model.IntentMove {
		t.Errorf("Expected MoveHandler to be bypassed on occupied tile, got IntentMove")
	}
}

func TestResolveMoveHandlerWallTileReturnsNil(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	// Mover at (1,1). Moving up (-Y) points to (1,0) which is a Wall.
	mover.Pos = geometry.Point{X: 1, Y: 1}
	play.Map.RemoveActor(geometry.Point{X: 2, Y: 2})
	play.Map.SetActor(mover.Pos, int64(mover.Id))

	vector := geometry.Point{X: 0, Y: -1}
	intent := resolver.Resolve(play, mover, vector)

	if intent != nil {
		t.Errorf("Expected nil for wall tile, got %v", intent.IntentType)
	}
}

//
// --- RESOLVER DISPATCH AND PRIORITY ---
//

//
// -- Priority chain --
//

func TestResolvePriorityAttackBeforeMove(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	// Monster placed on walkable floor at (3,2): both AttackHandler and
	// MoveHandler could claim this tile. AttackHandler must win.
	monster := model.NewDefaultZombie(2, geometry.Point{X: 3, Y: 2})
	play.Monsters[monster.Id] = monster
	play.Map.SetActor(monster.Pos, int64(monster.Id))

	vector := geometry.Point{X: 1, Y: 0}
	intent := resolver.Resolve(play, mover, vector)

	if intent == nil {
		t.Fatalf("Expected intent, got nil")
	}
	if intent.IntentType != model.IntentAttack {
		t.Errorf("Expected AttackHandler to take priority over MoveHandler, got %v", intent.IntentType)
	}
}

func TestResolvePriorityDoorBeforeMove(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	// Move mover adjacent to the closed door.
	mover.Pos = geometry.Point{X: 2, Y: 3}
	play.Map.RemoveActor(geometry.Point{X: 2, Y: 2})
	play.Map.SetActor(mover.Pos, int64(mover.Id))

	// Target (3,3) is a ClosedDoor: DoorBumpHandler must win over MoveHandler.
	vector := geometry.Point{X: 1, Y: 0}
	intent := resolver.Resolve(play, mover, vector)

	if intent == nil {
		t.Fatalf("Expected intent, got nil")
	}
	if intent.IntentType != model.IntentInteract {
		t.Errorf("Expected DoorBumpHandler to take priority over MoveHandler, got %v", intent.IntentType)
	}
}

//
// -- Nil result --
//

func TestResolveAllHandlersBlockedReturnsNil(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	// Target (1,0) is a Wall. No actor, not a door, not walkable.
	mover.Pos = geometry.Point{X: 1, Y: 1}
	play.Map.RemoveActor(geometry.Point{X: 2, Y: 2})
	play.Map.SetActor(mover.Pos, int64(mover.Id))

	vector := geometry.Point{X: 0, Y: -1}
	intent := resolver.Resolve(play, mover, vector)

	if intent != nil {
		t.Errorf("Expected nil when all handlers are blocked, got %v", intent.IntentType)
	}
}

//
// --- ADDITIONAL COVERAGE ---
//

//
// -- Out-of-bounds vector --
//

func TestResolveOutOfBoundsVectorDoesNotPanic(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Resolve panicked on out-of-bounds vector: %v. CanMoveTo dereferences a nil room for points outside the map.", r)
		}
	}()

	// Mover at (2,2). Vector (100,100) puts target at (102,102) — outside the
	// 7x7 map. Expected result is nil; actual result is a panic.
	vector := geometry.Point{X: 100, Y: 100}
	intent := resolver.Resolve(play, mover, vector)

	if intent != nil {
		t.Errorf("Expected nil for out-of-bounds target, got %v", intent.IntentType)
	}
}

//
// -- Locked door --
//

// DoorBumpHandler checks only tile type (ClosedDoor), not the lock state.
// A locked door must produce IntentInteract the same as an unlocked one.
// The actual key check is the responsibility of the interact service.
func TestResolveDoorBumpHandlerLockedDoorReturnsIntentInteract(t *testing.T) {
	play, resolver, mover := setupResolverEnv()

	doorPos := geometry.Point{X: 3, Y: 3}
	play.Map.LockDoor(doorPos, 1)

	mover.Pos = geometry.Point{X: 2, Y: 3}
	play.Map.RemoveActor(geometry.Point{X: 2, Y: 2})
	play.Map.SetActor(mover.Pos, int64(mover.Id))

	vector := geometry.Point{X: 1, Y: 0}
	intent := resolver.Resolve(play, mover, vector)

	if intent == nil {
		t.Fatalf("Expected IntentInteract for locked door, got nil")
	}
	if intent.IntentType != model.IntentInteract {
		t.Errorf("Expected IntentInteract for locked door, got %v", intent.IntentType)
	}
}

//
// -- Corridor tile --
//

// Corridor tiles are walkable (IsWalkable returns true for Corridor).
// MoveHandler must return IntentMove for a free corridor tile.
// This test builds a dedicated map that contains a corridor tile with a
// valid RoomId so CanMoveTo does not panic.
func TestResolveMoveHandlerCorridorTileIsWalkable(t *testing.T) {
	const (
		width  = 5
		height = 5
	)

	// All tiles belong to room 1. Inner positions are Floor except (3,2)
	// which is set to Corridor to simulate a room-connected corridor cell.
	grid := make([][]model.Cell, height)
	for y := range grid {
		grid[y] = make([]model.Cell, width)
		for x := range grid[y] {
			tileType := model.Floor
			if y == 0 || y == height-1 || x == 0 || x == width-1 {
				tileType = model.Wall
			}
			grid[y][x] = model.Cell{Type: tileType, RoomId: 1}
		}
	}
	// Corridor tile adjacent to mover's position (2,2).
	grid[2][3] = model.Cell{Type: model.Corridor, RoomId: 1}

	blueprint := &model.MapBlueprint{
		Width:    width,
		Height:   height,
		TileGrid: grid,
		Rooms: []*model.Room{
			{
				Id:     1,
				Pos:    geometry.Point{X: 0, Y: 0},
				Width:  width,
				Height: height,
				Center: geometry.Point{X: width / 2, Y: height / 2},
			},
		},
		Doors:          map[geometry.Point]*model.DoorMetadata{},
		EntranceRoomId: 1,
		ExitRoomId:     1,
		ExitPoint:      model.NewInvalidPoint(),
	}

	play := model.NewPlaythrough("", 0, 0)
	play.Map = model.NewMapFromBlueprint(blueprint)

	mover := model.NewDefaultPlayer(1, geometry.Point{X: 2, Y: 2}, 9)
	play.Players[mover.Id] = mover
	play.Map.SetActor(mover.Pos, int64(mover.Id))

	resolver := service.NewMoveResolverService()
	vector := geometry.Point{X: 1, Y: 0}
	intent := resolver.Resolve(play, mover, vector)

	if intent == nil {
		t.Fatalf("Expected IntentMove for corridor tile, got nil")
	}
	if intent.IntentType != model.IntentMove {
		t.Errorf("Expected IntentMove for corridor tile, got %v", intent.IntentType)
	}
}
