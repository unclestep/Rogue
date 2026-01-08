package entities

import (
	"testing"

	"github.com/Nikolay-Yakunin/gouge/internal/domain/entity"
	"github.com/Nikolay-Yakunin/gouge/internal/pkg/algorithm"
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

		if algorithm.FindPath(m, entrance, exit, false) == nil {
			t.Errorf("Seed %v: map is not fully connected\n", seed)
			t.Errorf("\n%v", m)
			break
		}
	}
}
