package entities

import (
	"testing"

	"github.com/Nikolay-Yakunin/gouge/internal/domain/entity"
	"github.com/Nikolay-Yakunin/gouge/internal/pkg/geometry"
)

func TestGenerateLevelNormalGrid(t *testing.T) {
	m := entity.NewDefaultMap()
	generated := m.GenerateLevel(3, 3)
	if !generated {
		t.Error("Expected true")
	}
}

func TestGenerateLevelTooSmallGrid(t *testing.T) {
	m := entity.NewCustomMap(10, 10)
	generated := m.GenerateLevel(3, 3)
	if generated {
		t.Error("Expected false")
	}
}

func TestGenerateLevelZeroGrid(t *testing.T) {
	m := entity.NewCustomMap(10, 10)
	generated := m.GenerateLevel(0, 0)
	if generated {
		t.Error("Expected false")
	}
}

func TestGenerateLevelNegativeGrid(t *testing.T) {
	m := entity.NewCustomMap(10, 10)
	generated := m.GenerateLevel(-1, -1)
	if generated {
		t.Error("Expected false")
	}
}

func TestGenerateLevelConnectivity(t *testing.T) {
	m := entity.NewDefaultMap()

	for range 100 {
		seed := m.GetSeed()

		generated := m.GenerateLevel(3, 3)

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

func TestMapEntities(t *testing.T) {
	m := entity.NewCustomMap(10, 10)
	m.GenerateLevel(1, 1)

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
		id, ok := m.GetActorID(wallPos)
		if ok || id != 0 {
			t.Errorf("Should not be able to set actor on non-walkable tile")
		}
	})
}

func TestMapBoundsAndWalkability(t *testing.T) {
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

func TestRoomQueries(t *testing.T) {
	m := entity.NewDefaultMap()
	m.GenerateLevel(2, 2)

	t.Run("GetRoomByPoint", func(t *testing.T) {
		room, ok := m.GetRoomByID(0)
		if !ok {
			t.Fatal("Room with ID 0 should exist")
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
