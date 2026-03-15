package entity_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func setupTestMap() *model.Map {
	grid := make([][]model.Cell, 10)
	for i := range grid {
		grid[i] = make([]model.Cell, 10)
		for j := range grid[i] {
			grid[i][j] = model.Cell{Type: model.Wall}
		}
	}

	for i := 1; i <= 3; i++ {
		for j := 1; j <= 3; j++ {
			grid[i][j] = model.Cell{Type: model.Floor, RoomId: 1}
		}
	}

	for i := 6; i <= 8; i++ {
		for j := 6; j <= 8; j++ {
			grid[i][j] = model.Cell{Type: model.Floor, RoomId: 2}
		}
	}

	grid[4][4] = model.Cell{Type: model.ClosedDoor}

	blueprint := &model.MapBlueprint{
		Width:    10,
		Height:   10,
		TileGrid: grid,
		Rooms: []*model.Room{
			{Id: 1, Pos: geometry.Point{X: 1, Y: 1}, Width: 3, Height: 3},
			{Id: 2, Pos: geometry.Point{X: 6, Y: 6}, Width: 3, Height: 3},
		},
		Doors: map[geometry.Point]*model.DoorMetadata{
			{X: 4, Y: 4}: {Pos: geometry.Point{X: 4, Y: 4}, Locked: true, Keyhole: 1},
		},
		EntranceRoomId: 1,
		ExitRoomId:     2,
		ExitPoint:      geometry.Point{X: 8, Y: 8},
	}

	return model.NewMapFromBlueprint(blueprint)
}

func TestMapInitializationAndGetters(t *testing.T) {
	m := setupTestMap()

	t.Run("Dimensions", func(t *testing.T) {
		dim := m.GetDimensions()
		if dim.X != 10 || dim.Y != 10 {
			t.Errorf("Expected 10x10 dimensions, got %dx%d", dim.X, dim.Y)
		}
	})

	t.Run("Rooms", func(t *testing.T) {
		if m.GetRoomCount() != 2 {
			t.Errorf("Expected 2 rooms, got %d", m.GetRoomCount())
		}
		if len(m.GetRooms()) != 2 {
			t.Errorf("GetRooms returned incorrect number of rooms")
		}

		entRoom := m.GetEntranceRoom()
		if entRoom == nil || entRoom.Id != 1 {
			t.Error("Incorrect entrance room")
		}
		// Entrance room shouldn't have free item points (assumed in Hydrate)
		if entRoom.GetItemCapacity() != 0 {
			t.Errorf("Entrance room capacity should be 0, got %d", entRoom.GetItemCapacity())
		}

		extRoom := m.GetExitRoom()
		if extRoom == nil || extRoom.Id != 2 {
			t.Error("Incorrect exit room")
		}
		// Area 3x3 = 9. Minus ExitPoint(8,8) = 8
		if extRoom.GetItemCapacity() != 8 {
			t.Errorf("Expected capacity 8 for exit room, got %d", extRoom.GetItemCapacity())
		}
	})

	t.Run("Room Retrievers", func(t *testing.T) {
		room, ok := m.GetRoomByID(1)
		if !ok || room.Id != 1 {
			t.Error("Failed to retrieve room by ID")
		}

		room, ok = m.GetRoomByPoint(geometry.Point{X: 2, Y: 2})
		if !ok || room.Id != 1 {
			t.Error("Failed to retrieve room by coordinates inside the room")
		}

		_, ok = m.GetRoomByPoint(geometry.Point{X: 0, Y: 0})
		if ok {
			t.Error("Searching for a room in a wall should return false")
		}
	})
}

func TestMapPredicates(t *testing.T) {
	m := setupTestMap()

	t.Run("InBounds", func(t *testing.T) {
		if !m.InBounds(geometry.Point{X: 5, Y: 5}) {
			t.Error("Point 5,5 should be within map bounds")
		}
		if m.InBounds(geometry.Point{X: -1, Y: 5}) || m.InBounds(geometry.Point{X: 10, Y: 10}) {
			t.Error("Point outside of bounds detected incorrectly")
		}
	})

	t.Run("Tiles Walkability", func(t *testing.T) {
		floor := geometry.Point{X: 2, Y: 2}
		wall := geometry.Point{X: 0, Y: 0}
		closedDoor := geometry.Point{X: 4, Y: 4}

		if !m.IsWalkable(floor) {
			t.Error("Floor should be walkable")
		}
		if m.IsWalkable(wall) {
			t.Error("Wall should not be walkable")
		}
		if m.IsWalkable(closedDoor) {
			t.Error("Closed door should not be walkable")
		}
	})

	t.Run("Doors Predicates", func(t *testing.T) {
		door := geometry.Point{X: 4, Y: 4}
		if !m.IsDoor(door) {
			t.Error("Point should be recognized as a door")
		}
		if !m.IsClosedDoor(door) {
			t.Error("Door should be closed by default")
		}
		if m.IsOpenDoor(door) {
			t.Error("Door should not be open")
		}
	})
}

func TestMapSettersAndMovement(t *testing.T) {
	m := setupTestMap()
	floorPoint := geometry.Point{X: 6, Y: 6} // Point in ExitRoom
	wallPoint := geometry.Point{X: 0, Y: 0}

	t.Run("Actors", func(t *testing.T) {
		if m.SetActor(wallPoint, 1) {
			t.Error("Actor should not be placed on a wall")
		}

		if !m.SetActor(floorPoint, 99) {
			t.Error("Actor should be successfully placed on the floor")
		}
		if !m.IsActor(floorPoint) {
			t.Error("IsActor should return true")
		}

		id, ok := m.GetActorID(floorPoint)
		if !ok || id != 99 {
			t.Error("Incorrect actor ID retrieved")
		}

		if m.CanMoveTo(floorPoint) {
			t.Error("CanMoveTo should return false for an occupied tile")
		}

		m.RemoveActor(floorPoint)
		if m.IsActor(floorPoint) {
			t.Error("Actor should have been removed")
		}
	})

	t.Run("Items", func(t *testing.T) {
		if m.SetItem(wallPoint, 1) {
			t.Error("Item should not be placed on a wall")
		}

		m.SetItem(floorPoint, 42)
		if !m.IsItem(floorPoint) {
			t.Error("Item not found after placement")
		}

		id, ok := m.GetItemID(floorPoint)
		if !ok || id != 42 {
			t.Error("Incorrect item ID retrieved")
		}

		m.RemoveItem(floorPoint)
		if m.IsItem(floorPoint) {
			t.Error("Item should have been removed")
		}
	})

	t.Run("Movement", func(t *testing.T) {
		src := geometry.Point{X: 6, Y: 6}
		dst := geometry.Point{X: 6, Y: 7}

		// Try to move from an empty cell
		if m.Move(src, dst) {
			t.Error("Move should not work from an empty cell")
		}

		m.SetActor(src, 10)

		// Try to move into a wall
		if m.Move(src, wallPoint) {
			t.Error("Actor should not move into a wall")
		}

		// Successful move
		if !m.Move(src, dst) {
			t.Error("Successful move should return true")
		}
		if m.IsActor(src) {
			t.Error("Actor was not removed from the source position")
		}
		if !m.IsActor(dst) {
			t.Error("Actor did not appear at the destination position")
		}

		// Try to move into an occupied cell
		m.SetActor(src, 11) // Set new one
		if m.Move(src, dst) {
			t.Error("Cannot move into an occupied cell")
		}
	})
}

func TestMapRandomPoints(t *testing.T) {
	m := setupTestMap()
	rng := rand.New(rand.NewSource(1))
	room := m.GetExitRoom()

	initialCap := room.GetItemCapacity()

	pt, ok := m.TakeRandomItemPoint(room, rng)
	if !ok {
		t.Fatal("Should have received a free point")
	}

	if room.GetItemCapacity() != initialCap-1 {
		t.Error("Room capacity should have decreased by 1")
	}

	// Empty the room completely
	for i := 0; i < initialCap-1; i++ {
		_, ok := m.TakeRandomItemPoint(room, rng)
		if !ok {
			t.Error("Expected to receive a free point, but they ran out early")
		}
	}

	// Verify no spots left
	_, ok = m.TakeRandomItemPoint(room, rng)
	if ok {
		t.Error("Room should be empty to prevent issuing new points")
	}

	// Return one back (simulating item removal)
	m.SetItem(pt, 0)
	if room.GetItemCapacity() != 1 {
		t.Error("Capacity should have been restored after SetItem(pt, 0)")
	}
}

func TestMapDoors(t *testing.T) {
	m := setupTestMap()
	doorPoint := geometry.Point{X: 4, Y: 4}

	// Try to open with incorrect key
	if m.TryOpenDoor(doorPoint, 999) {
		t.Error("Door should not open with an incorrect key")
	}
	if m.IsOpenDoor(doorPoint) {
		t.Error("Door should still be closed")
	}

	// Try to open with correct key (blueprint has Keyhole: 1)
	if !m.TryOpenDoor(doorPoint, 1) {
		t.Error("Door should open with the correct key")
	}
	if !m.IsOpenDoor(doorPoint) {
		t.Error("Tile type should have changed to OpenDoor")
	}

	// Door with no lock (Keyhole: 0) or already open should return true
	if !m.TryOpenDoor(doorPoint, 0) {
		t.Error("An already open door should return true on any attempt to open")
	}

	// Locking
	m.LockDoor(doorPoint)
	if !m.IsClosedDoor(doorPoint) {
		t.Error("Door should be locked again")
	}
}

func TestMapDungeonClearing(t *testing.T) {
	m := setupTestMap()
	p := geometry.Point{X: 6, Y: 6}
	m.SetActor(p, 1)
	m.SetItem(p, 1)

	// Test clearing
	m.ClearActors()
	if m.IsActor(p) {
		t.Error("Actors should be cleared")
	}

	m.ClearItems()
	if m.IsItem(p) {
		t.Error("Items should be cleared")
	}

	// After ClearTopology, everything should be broken/reset
	m.ClearTopology()
	if m.GetRoomCount() != 0 {
		t.Error("Room list should be empty")
	}
	if m.EntranceRoomId != model.InvalidRoomId {
		t.Error("Entrance room ID should have been reset")
	}
}

func TestMapScentAndLoot(t *testing.T) {
	m := setupTestMap()

	t.Run("GenerateScentMap", func(t *testing.T) {
		target := geometry.Point{X: 2, Y: 2} // Entrance room center
		scentMap := m.GenerateScentMap([]geometry.Point{target})

		if scentMap[target.Y][target.X] != 0 {
			t.Error("Scent at the target point should be 0")
		}
		if scentMap[0][0] != math.MaxInt {
			t.Error("Scent in walls should remain MaxInt")
		}
		// Since we know there is a floor (2,3) around floor (2,2), distance should be 1
		if scentMap[3][2] != 1 {
			t.Errorf("Scent on an adjacent free tile should be 1, got %d", scentMap[3][2])
		}
	})

	t.Run("FindEmptyPoint", func(t *testing.T) {
		center := geometry.Point{X: 7, Y: 7} // Exit room

		// Block the point itself
		m.SetItem(center, 99)

		// With radius 0 nothing should be found, as the point is occupied
		_, ok := m.FindEmptyPoint(center, 0)
		if ok {
			t.Error("Should not find a point with rad=0 if it is occupied")
		}

		// With radius 1 it should find an adjacent cell
		pt, ok := m.FindEmptyPoint(center, 1)
		if !ok {
			t.Error("Should find an empty adjacent cell within radius 1")
		}
		if pt == center {
			t.Error("Found cell should not coincide with the blocked center")
		}

		// Fill the entire room so that rad=-1 (entire room) doesn't find a spot
		for i := 6; i <= 8; i++ {
			for j := 6; j <= 8; j++ {
				m.SetItem(geometry.Point{X: j, Y: i}, 99)
			}
		}

		_, ok = m.FindEmptyPoint(center, -1)
		if ok {
			t.Error("Should not find points if the entire room is filled")
		}

		// Request with invalid input (in a wall)
		_, ok = m.FindEmptyPoint(geometry.Point{X: 0, Y: 0}, 2)
		if ok {
			t.Error("Call for non-walkable point should fail with false")
		}
	})
}
