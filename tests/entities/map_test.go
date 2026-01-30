package entities

import (
	"io"
	"log"
	"maps"
	"testing"
	"time"

	"github.com/Nikolay-Yakunin/gouge/internal/domain/entity"
	"github.com/Nikolay-Yakunin/gouge/internal/pkg/geometry"
)

//
//
// --- LEVEL GENERATION LOGIC ---
//
//

//
// -- TOPOLOGY GENERATION TESTS --
//

// Grid tests

func TestGenerateTopologyNormalGrid(t *testing.T) {
	m := entity.NewDefaultMap()
	generated := m.GenerateTopology(3, 3)
	if !generated {
		t.Error("Expected true")
	}
}

func TestGenerateTopologyTooSmallGrid(t *testing.T) {
	m := entity.NewCustomMap(10, 10)
	generated := m.GenerateTopology(3, 3)
	if generated {
		t.Error("Expected false")
	}
}

func TestGenerateTopologyZeroGrid(t *testing.T) {
	m := entity.NewCustomMap(10, 10)
	generated := m.GenerateTopology(0, 0)
	if generated {
		t.Error("Expected false")
	}
}

func TestGenerateTopologyNegativeGrid(t *testing.T) {
	m := entity.NewCustomMap(10, 10)
	generated := m.GenerateTopology(-1, -1)
	if generated {
		t.Error("Expected false")
	}
}

// Accessibility from entrance to exit test
func TestGenerateTopologyConnectivity(t *testing.T) {
	m := entity.NewDefaultMap()

	for range 100 {
		seed := time.Now().UnixNano()
		m.SetSeed(seed)

		m.ClearTopology()
		generated := m.GenerateTopology(3, 3)

		entrance := m.GetEntrancePoint()
		exit := m.GetExitPoint()

		if !generated {
			t.Errorf("Seed %v: expected generated = true, get: %v\n", seed, generated)
			break
		}

		if !m.IsWalkable(entrance) {
			t.Errorf("Seed %v: entrance is not walkable!\n", seed)
			break
		}

		if !m.IsWalkable(exit) {
			t.Errorf("Seed %v: exit is not walkable!\n", seed)
			break
		}

		for id := range m.GetRoomsCount() {
			room, _ := m.GetRoomByID(id)
			pos := room.GetPos()
			x, y := pos.X, pos.Y
			height, width := room.GetHW()

			if !m.InBounds(pos) || !m.InBounds(geometry.Point{X: x + width - 1, Y: y + height - 1}) {
				t.Errorf("Seed %v: room #%d is out of bounds", seed, id)
			}
		}

		if _, ok := m.FindPath(entrance, exit); !ok {
			t.Errorf("Seed %v: map is not fully connected\n", seed)
			t.Errorf("\n%v", m)
			break
		}
	}
}

//
// -- OBJECT GENERATION TESTS --
//

func TestGenerateItems(t *testing.T) {
	m := entity.NewCustomMap(80, 24)
	testGenerateObjects(
		t,
		m,
		m.GenerateItems,
		func(r *entity.Room) []geometry.Point {
			return r.GetEmptyItemPoints()
		},
		m.ClearItems)
}

func TestGenerateActors(t *testing.T) {
	m := entity.NewCustomMap(80, 24)
	testGenerateObjects(t,
		m,
		m.GenerateActors,
		func(r *entity.Room) []geometry.Point {
			return r.GetEmptyActorPoints()
		},
		m.ClearActors)
}

// Default object generation tests
// Non-repeating generation tests
// Non-natural number of objects to generate tests
// Generation more object than util cells tests
// Clearing tests
func testGenerateObjects(t *testing.T,
	m *entity.Map,
	genObject func(n int) map[int]geometry.Point,
	getEmptyPoints func(r *entity.Room) []geometry.Point,
	clearObjects func(),
) {
	for range 100 {
		seed := time.Now().UnixNano()
		m.SetSeed(seed)

		m.ClearTopology()
		clearObjects()

		// Correct clearing checks
		if m.GetEntranceRoom() != nil {
			t.Errorf("Seed %v: expected nil, got %v", seed, m.GetEntranceRoom())
		}

		if m.GetExitRoom() != nil {
			t.Errorf("Seed %v: expected nil, got %v", seed, m.GetExitRoom())
		}

		for i := 0; i < 24; i++ {
			for j := 0; j < 80; j++ {
				p := geometry.Point{X: j, Y: i}

				cellTileGrid, tok := m.GetTileType(p)
				cellItemGrid, iok := m.GetItemID(p)
				cellActorGrid, aok := m.GetActorID(p)

				if !tok {
					t.Errorf("Seed %v: point %v is out of bounds in tileGrid", seed, p)
				}

				if !iok {
					t.Errorf("Seed %v: point %v is out of bounds in itemGrid", seed, p)
				}

				if !aok {
					t.Errorf("Seed %v: point %v is out of bounds in actorGrid", seed, p)
				}

				if cellTileGrid != 0 {
					t.Errorf("Seed %v: expected 0, got %v", seed, cellTileGrid)
				}

				if cellItemGrid != 0 {
					t.Errorf("Seed %v: expected 0, got %v", seed, cellItemGrid)
				}
				if cellActorGrid != 0 {
					t.Errorf("Seed %v: expected 0, got %v", seed, cellActorGrid)
				}
			}
		}

		m.GenerateTopology(3, 3)

		availablePoints := make([]geometry.Point, 0, 80*24)
		for i := 0; i < m.GetRoomsCount(); i++ {
			room, _ := m.GetRoomByID(i)
			if room == m.GetEntranceRoom() {
				continue
			}

			availablePoints = append(availablePoints, getEmptyPoints(room)...)
		}

		entrance := m.GetEntrancePoint()
		exit := m.GetExitPoint()

		// First generation (all cells are available to store firstItems)
		genObjects := genObject(3)

		if len(genObjects) != 3 {
			t.Errorf("Seed: %v\nExpected 3 items, got %d", seed, len(genObjects))
		}

		// Second generation (previous firstItems should stay, new ones shouldn't intersect old ones)
		maps.Copy(genObjects, genObject(3))
		if len(genObjects) != 6 {
			t.Errorf("Seed: %v\nExpected 6 items after 2nd gen, got %d", seed, len(genObjects))
		}

		// Third generation (check negative n)
		original := log.Writer() // Temporarily shutting down the logger
		log.SetOutput(io.Discard)

		noGen1 := genObject(-1)
		if noGen1 != nil {
			t.Errorf("Seed: %v\nThere should be no items in the third generation", seed)
		}

		// Fourth generation (check zero n)
		noGen2 := genObject(0)
		if noGen2 != nil {
			t.Errorf("Seed: %v\nThere should be no items in the fourth generation", seed)
		}

		log.SetOutput(original) // Turning on the logger back

		// Fifth generation
		// Generating more items than available cells in the map
		// Should generate maximum possible number of items
		// Extras shouldn't appear
		maps.Copy(genObjects, genObject(80*24))
		if len(genObjects) != len(availablePoints) {
			t.Errorf("Seed: %v\nExpected %d items after 5th gen, got %d", seed, len(availablePoints), len(genObjects))
		}

		for _, i := range genObjects {
			if !m.IsWalkable(i) {
				t.Errorf("Seed: %v\nItem %v should be walkable", seed, i)
			}

			if _, ok := m.FindPath(entrance, i); !ok {
				t.Errorf("Seed: %v\nItem %v should be accessible from entrance %v", seed, i, entrance)
			}

			if _, ok := m.FindPath(exit, i); !ok {
				t.Errorf("Seed: %v\nItem %v should be accessible from exit %v", seed, i, exit)
			}
		}

		genPointsSet := getValues(genObjects)
		availablePointsSet := sliceToSet(availablePoints)

		if diff := mapDiff(genPointsSet, availablePointsSet); len(diff) > 0 {
			t.Errorf("Seed %v\nGenerated points are not equal initially available points\nDiff:%v\n", seed, diff)

		}
	}
}

func TestGenerateLevel(t *testing.T) {
	m := entity.NewDefaultMap()
	targetItems := 5
	targetActors := 3

	for range 100 {
		seed := time.Now().UnixNano()
		m.SetSeed(seed)

		m.GenerateLevel(targetItems, targetActors)

		//  Verify topology exists
		if m.GetRoomsCount() == 0 {
			t.Errorf("Seed %v: no rooms generated", seed)
		}
		if m.GetEntranceRoom() == nil || m.GetExitRoom() == nil {
			t.Errorf("Seed %v: entrance or exit not defined", seed)
		}

		// Verify objects counts
		itemCount, actorCount := 0, 0
		width, height := 80, 24 // Default map size

		for y := range height {
			for x := range width {
				p := geometry.Point{X: x, Y: y}
				if m.IsItem(p) {
					itemCount++
				}
				if m.IsActor(p) {
					actorCount++
				}
			}
		}

		if itemCount != targetItems {
			t.Errorf("Seed %v: expected %d items, got %d", seed, targetItems, itemCount)
		}
		if actorCount != targetActors {
			t.Errorf("Seed %v: expected %d actors, got %d", seed, targetActors, actorCount)
		}

		// Verify connectivity
		if _, ok := m.FindPath(m.GetEntrancePoint(), m.GetExitPoint()); !ok {
			t.Errorf("Seed %v: level is not connected", seed)
		}
	}
}

//
//
// --- LOOT MANAGEMENT TESTS ---
//
//

func TestLootSystem(t *testing.T) {
	m := entity.NewDefaultMap()

	for i := 0; i < 100; i++ {
		seed := time.Now().UnixNano()
		m.SetSeed(seed + int64(i))

		m.ClearLevel()
		m.GenerateTopology(3, 3)

		// Ensure we have a valid room and point
		var room *entity.Room
		for i := 0; i < m.GetRoomsCount(); i++ {
			if r, _ := m.GetRoomByID(i); len(r.GetEmptyItemPoints()) > 1 && r != m.GetEntranceRoom() {
				room = r
			}
		}

		if room == nil {
			t.Fatalf("Seed: %v\nCan't find room with at least 2 free points", seed)
			return
		}

		center := room.GetCenter()

		t.Run("SpawnLootSuccess", func(t *testing.T) {
			// Attempt to spawn loot near center
			id, pos, ok := m.SpawnLoot(center, 5)

			if !ok {
				t.Errorf("Seed: %v\nFailed to spawn loot in valid room", seed)
			}

			if pos != center {
				t.Errorf("Seed: %v\nSpawnLoot: expected point %v, got %v", seed, center, pos)
			}

			if id == 0 {
				t.Errorf("Seed: %v\nExpected valid ID > 0", seed)
			}

			if !m.IsItem(pos) {
				t.Errorf("Seed: %v\nItem grid not updated at %v", seed, pos)
			}

			if !m.IsWalkable(pos) {
				t.Errorf("Seed: %v\nLoot spawned on non-walkable tile %v", seed, pos)
			}
		})

		t.Run("SpawnLootInvalidCenter", func(t *testing.T) {
			// Use logger discard to suppress expected error output
			original := log.Writer()
			log.SetOutput(io.Discard)
			defer log.SetOutput(original)

			wall := geometry.Point{X: 0, Y: 0}
			if _, _, ok := m.SpawnLoot(wall, -1); ok {
				t.Errorf("Seed: %v\nShould not spawn loot from non-walkable center", seed)
			}
		})

		t.Run("FindLootPointLogic", func(t *testing.T) {
			// Should spawn loot not in the given point
			m.SetItem(center, 1)
			p, found := m.FindLootPoint(center, 10)
			if !found {
				t.Errorf("Seed: %v\nShould find point in empty room", seed)
			}

			if p == center {
				t.Errorf("Seed: %v\nPoint %v should be different from %v", seed, p, center)
			}
		})

		t.Run("ImpossibleToSpawn", func(t *testing.T) {
			for _, p := range room.GetEmptyItemPoints() {
				m.SetItem(p, 1)
			}

			if len(room.GetEmptyItemPoints()) > 0 {
				t.Errorf("Seed: %v\nShould be no available points in the room, got %v available points", seed, len(room.GetEmptyItemPoints()))
			}

			p, found := m.FindLootPoint(center, 10)
			if found {
				t.Errorf("Seed: %v\nShould not find point in full room with rad=10, but found point %v", seed, p)
				t.Errorf("\nPlayer position: %v", m.GetEntrancePoint())
			}

			p, found = m.FindLootPoint(center, -1)
			if found {
				t.Errorf("Seed: %v\nShould not find point in full room with rad=-1, but found point %v", seed, p)
			}
		})
	}

}

//
//
// --- PATHFINDING INTERFACE TESTS ---
//
//

func TestPathfindingInterface(t *testing.T) {
	m := entity.NewDefaultMap()
	m.GenerateTopology(2, 2)

	start := m.GetEntrancePoint()
	end := m.GetExitPoint()

	t.Run("FindPathBasic", func(t *testing.T) {
		path, ok := m.FindPath(start, end)

		if !ok {
			t.Errorf("Path should exist on generated map")
		}
		if len(path) == 0 {
			t.Errorf("Path should not be empty")
		}

		// Verify path continuity
		for i := 0; i < len(path)-1; i++ {
			curr, next := path[i], path[i+1]
			dist := m.CalcHeuristic(curr, next)
			if dist != 1.0 {
				t.Errorf("Path gap between %v and %v", curr, next)
			}
		}
	})

	t.Run("GetNeighbors", func(t *testing.T) {
		neighbors := m.GetNeighbors(start)
		if len(neighbors) == 0 {
			t.Error("Walkable point should have neighbors")
		}
		for _, n := range neighbors {
			if !m.InBounds(n) {
				t.Errorf("Neighbor %v out of bounds", n)
			}
		}
	})

	t.Run("PathEdgeCases", func(t *testing.T) {
		// Path to self
		path, ok := m.FindPath(start, start)
		if !ok || len(path) != 1 {
			t.Error("Path to self should contain 1 point")
		}

		// Path to wall
		wall := geometry.Point{X: 0, Y: 0} // Assumed wall
		if _, ok := m.FindPath(start, wall); ok {
			t.Error("Should not find path to wall")
		}
	})
}

//
//
// --- GETTERS TESTS ---
//
//

func TestMapBounds(t *testing.T) {
	width, height := 80, 24
	m := entity.NewCustomMap(width, height)

	testCases := []struct {
		name     string
		point    geometry.Point
		expected bool
	}{
		{"TopLeft", geometry.Point{X: 0, Y: 0}, true},
		{"BottomRight", geometry.Point{X: width - 1, Y: height - 1}, true},
		{"NegativeX", geometry.Point{X: -1, Y: 10}, false},
		{"OutOfBoundsY", geometry.Point{X: 10, Y: height}, false},
	}

	for _, tc := range testCases {
		if res := m.InBounds(tc.point); res != tc.expected {
			t.Errorf("%s: InBounds(%v) = %v, want %v", tc.name, tc.point, res, tc.expected)
		}
	}
}

func TestRoomGetters(t *testing.T) {
	m := entity.NewDefaultMap()
	m.GenerateTopology(2, 2)

	t.Run("GetRoomByPoint", func(t *testing.T) {
		room, ok := m.GetRoomByID(0)
		if !ok {
			t.Errorf("Room with ID 0 should exist")
		}

		foundRoom, foundOk := m.GetRoomByPoint(room.GetCenter())
		if !foundOk || foundRoom.GetID() != room.GetID() {
			t.Errorf("Failed to find room by its center point")
		}
	})

	t.Run("NonExistentRoom", func(t *testing.T) {
		_, ok := m.GetRoomByID(999)
		if ok {
			t.Error("Should not find room with invalid ID")
		}
	})
}

//
//
// --- SETTERS TESTS ---
//
//

func TestMapEntities(t *testing.T) {
	m := entity.NewCustomMap(10, 10)
	m.GenerateTopology(1, 1)

	room := m.GetEntranceRoom()
	center := room.GetCenter()

	t.Run("SetAndGetActor", func(t *testing.T) {
		actorID := 42
		m.SetActor(center, actorID)
		id, ok := m.GetActorID(center)
		if !ok || id != actorID {
			t.Errorf("Expected actor ID %d, got %d (ok: %v)", actorID, id, ok)
		}
	})

	t.Run("SetAndGetItem", func(t *testing.T) {
		itemID := 7
		m.SetItem(center, itemID)
		id, ok := m.GetItemID(center)
		if !ok || id != itemID {
			t.Errorf("Expected item ID %d, got %d (ok: %v)", itemID, id, ok)
		}
	})

	t.Run("SetOnNonWalkableTile", func(t *testing.T) {
		wallPos := geometry.Point{X: 0, Y: 0}
		m.SetActor(wallPos, 99)
		id, _ := m.GetActorID(wallPos)
		if id != 0 {
			t.Errorf("Should not be able to set actor on non-walkable tile")
		}
	})
}

//
//
// --- MUTATORS TESTS ---
//
//

func TestIDGenerationSequence(t *testing.T) {
	m := entity.NewDefaultMap()

	// Check sequentiality
	id1 := m.GenID()
	id2 := m.GenID()
	id3 := m.GenID()

	if id1 == id2 || id2 == id3 {
		t.Error("GenID produced duplicate values")
	}
	if id2 != id1+1 || id3 != id2+1 {
		t.Error("GenID is not incremental")
	}
}

func TestMutatorsAndRandomPickers(t *testing.T) {
	m := entity.NewDefaultMap()

	for i := 0; i < 100; i++ {
		seed := time.Now().UnixNano()
		m.SetSeed(seed + int64(i))

		m.ClearLevel()
		m.GenerateTopology(3, 3)
		var room *entity.Room

		for i := range m.GetRoomsCount() {
			r, _ := m.GetRoomByID(i)
			if len(r.GetEmptyItemPoints()) > 1 {
				room = r
				break
			}
		}

		t.Run("RandomPoints", func(t *testing.T) {
			// Take random item point
			itemOldLen := len(room.GetEmptyItemPoints())
			actorOldLen := len(room.GetEmptyActorPoints())

			p1, ok := m.TakeRandomItemPoint(room)

			if !ok {
				t.Errorf("Seed: %v\nShould be available points to spawn an item", seed)
			}

			// Verify point is actually in the room
			if r, _ := m.GetRoomByPoint(p1); r != room {
				t.Errorf("Seed: %v\nPoint %v belongs to wrong room", seed, p1)
			}

			if len(room.GetEmptyItemPoints()) != itemOldLen-1 {
				t.Errorf("Seed: %v\nNumber of empty item points should have decreased", seed)
			}

			m.RemoveItem(p1)

			// Take random actor point
			p2, ok := m.TakeRandomActorPoint(room)

			if !ok {
				t.Errorf("Seed: %v\nShould be available points to spawn an item", seed)
			}

			if !m.IsWalkable(p2) {
				t.Errorf("Seed: %v\nRandom actor point %v should be walkable", seed, p2)
			}

			if len(room.GetEmptyActorPoints()) != actorOldLen-1 {
				t.Errorf("Seed: %v\nNumber of empty actor points should have decreased", seed)
			}

			m.RemoveActor(p2)
		})

		t.Run("RemoveItem", func(t *testing.T) {
			p := room.GetEmptyItemPoints()[0]
			oldLen := len(room.GetEmptyItemPoints())

			// Setup item
			m.SetItem(p, 100)
			if !m.IsItem(p) {
				t.Errorf("Seed: %v\nSetup failed: item not set", seed)
			}
			if len(room.GetEmptyItemPoints()) != oldLen-1 {
				t.Errorf("Seed: %v\nNumber of empty item points should have decreased", seed)
			}

			// Test Remove
			if !m.RemoveItem(p) {
				t.Errorf("Seed: %v\nRemoveItem returned false", seed)
			}
			if m.IsItem(p) {
				t.Errorf("Seed: %v\nItem should be removed", seed)
			}
			if len(room.GetEmptyItemPoints()) != oldLen {
				t.Errorf("Seed: %v\nNumber of empty item points should have increased after removal", seed)
			}

			// Verify internal room map updated (item can be placed again)
			// SetItem returns true only if it can place (logic inside SetItem handles map update)
			// We verify state by checking ID
			id, _ := m.GetItemID(p)
			if id != 0 {
				t.Errorf("Seed: %v\nExpected ID 0 after removal, got %d", seed, id)
			}

			m.SetItem(p, 100)
			if !m.IsItem(p) {
				t.Errorf("Seed: %v\nSetup failed: item not set", seed)
			}
			if len(room.GetEmptyItemPoints()) != oldLen-1 {
				t.Errorf("Seed: %v\nNumber of empty item points should have decreased", seed)
			}
			m.RemoveItem(p)
		})

		t.Run("RemoveActor", func(t *testing.T) {
			p := room.GetEmptyActorPoints()[0]
			oldLen := len(room.GetEmptyActorPoints())

			// Setup actor
			m.SetActor(p, 100)
			if !m.IsActor(p) {
				t.Errorf("Seed: %v\nSetup failed: actor not set", seed)
			}
			if len(room.GetEmptyActorPoints()) != oldLen-1 {
				t.Errorf("Seed: %v\nNumber of empty actor points should have decreased", seed)
			}

			// Test Remove
			if !m.RemoveActor(p) {
				t.Errorf("Seed: %v\nRemoveActor returned false", seed)
			}
			if m.IsActor(p) {
				t.Errorf("Seed: %v\nActor should be removed", seed)
			}
			if len(room.GetEmptyActorPoints()) != oldLen {
				t.Errorf("Seed: %v\nNumber of empty actor points should have increased", seed)
			}

			// Verify internal room map updated (actor can be placed again)
			// SetActor returns true only if it can place (logic inside SetItem handles map update)
			// We verify state by checking ID
			id, _ := m.GetActorID(p)
			if id != 0 {
				t.Errorf("Seed: %v\nExpected ID 0 after removal, got %d", seed, id)
			}

			m.SetActor(p, 100)
			if !m.IsActor(p) {
				t.Errorf("Seed: %v\nSetup failed: actor not set", seed)
			}
			if len(room.GetEmptyActorPoints()) != oldLen-1 {
				t.Errorf("Seed: %v\nNumber of empty actor point should have decreased", seed)
			}
			m.RemoveActor(p)
		})

		t.Run("ItemAndActorInSamePoint", func(t *testing.T) {
			p := room.GetEmptyItemPoints()[0]

			m.SetItem(p, 100)
			if !m.IsItem(p) {
				t.Errorf("Seed: %v\nSetup failed: item not set", seed)
			}

			m.SetActor(p, 100)
			if !m.IsActor(p) {
				t.Errorf("Seed: %v\nSetup failed: actor not set", seed)
			}

			if !m.RemoveActor(p) {
				t.Errorf("Seed: %v\nRemoveActor returned false", seed)
			}
			if m.IsActor(p) {
				t.Errorf("Seed: %v\nActor should be removed", seed)
			}
			if !m.IsItem(p) {
				t.Errorf("Seed: %v\nItem should stay after actor removal", seed)
			}
		})
	}

}

//
//
// -- STABILITY & DETERMINISM TESTS --
//
//

func TestMapDeterminism(t *testing.T) {
	// Seed must guarantee identical map generation
	seed := int64(1769754240669420900)

	// Map A
	mA := entity.NewDefaultMap()
	mA.SetSeed(seed)
	mA.GenerateLevel(5, 5)
	strA := mA.String() // String representation captures topology

	// Map B
	mB := entity.NewDefaultMap()
	mB.SetSeed(seed)
	mB.GenerateLevel(5, 5)
	strB := mB.String()

	if strA != strB {
		t.Error("Maps with same seed differ in topology")
	}

	// Compare object positions
	pA := mA.GetEntrancePoint()
	pB := mB.GetEntrancePoint()
	if pA != pB {
		t.Errorf("Entrance points differ: %v vs %v", pA, pB)
	}

	// Map C (different seed)
	mC := entity.NewDefaultMap()
	mC.SetSeed(seed + 1)
	mC.GenerateLevel(5, 5)
	if mA.String() == mC.String() {
		t.Error("Maps with different seeds are identical")
	}
}

//
//
// --- HELPERS & UTILITIES ---
//
//

func getValues[K, V comparable](kv map[K]V) map[V]struct{} {
	vals := make(map[V]struct{}, len(kv))
	for _, v := range kv {
		vals[v] = struct{}{}
	}
	return vals
}

func sliceToSet[T comparable](s []T) map[T]struct{} {
	set := make(map[T]struct{}, len(s))
	for _, v := range s {
		set[v] = struct{}{}
	}
	return set
}

func mapDiff[K comparable, V any](m1, m2 map[K]V) map[K]V {
	diff := make(map[K]V, len(m1)+len(m2))

	for k, v := range m1 {
		if _, ok := m2[k]; !ok {
			diff[k] = v
		}
	}

	for k, v := range m2 {
		if _, ok := m1[k]; !ok {
			diff[k] = v
		}
	}

	return diff
}
