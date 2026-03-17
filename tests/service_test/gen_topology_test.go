// gen_topology_test.go
package service_test

import (
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func createTestContext(seed int64) *model.SessionContext {
	playthrough := model.NewPlaythrough(1, 1, seed)
	playthrough.Map = &model.Map{}
	return model.NewSessionContext(playthrough)
}

func TestGenerateTopologyGridSizes(t *testing.T) {
	tg := service.NewTopologyGenerator()
	ctx := createTestContext(time.Now().UnixNano())

	t.Run("NormalGrid", func(t *testing.T) {
		ctx.Playthrough.Map = nil
		tg.Gen(ctx, 80, 24, 3, 3)
		if ctx.Playthrough.Map == nil {
			t.Error("Expected map to be generated (not nil)")
		}
	})

	t.Run("TooSmallGrid", func(t *testing.T) {
		ctx.Playthrough.Map = nil
		tg.Gen(ctx, 10, 10, 3, 3)
		if ctx.Playthrough.Map != nil {
			t.Error("Expected map to fail generation (nil) for too small grid")
		}
	})

	t.Run("ZeroGrid", func(t *testing.T) {
		ctx.Playthrough.Map = nil
		tg.Gen(ctx, 10, 10, 0, 0)
		if ctx.Playthrough.Map != nil {
			t.Error("Expected map to fail generation (nil) for 0 rooms")
		}
	})

	t.Run("NegativeGrid", func(t *testing.T) {
		ctx.Playthrough.Map = nil
		tg.Gen(ctx, 10, 10, -1, -1)
		if ctx.Playthrough.Map != nil {
			t.Error("Expected map to fail generation (nil) for negative rooms")
		}
	})
}

func TestGenerateTopologyConnectivity(t *testing.T) {
	tg := service.NewTopologyGenerator()

	for i := 0; i < 100; i++ {
		seed := time.Now().UnixNano() + int64(i)
		ctx := createTestContext(seed)

		tg.Gen(ctx, 80, 24, 3, 3)
		m := ctx.Playthrough.Map

		if ctx.Playthrough.Map == nil {
			t.Errorf("Seed %v: expected generated map, got nil", seed)
			break
		}

		entranceRoom := m.GetEntranceRoom()
		exitRoom := m.GetExitRoom()

		if entranceRoom == nil || exitRoom == nil {
			t.Errorf("Seed %v: entrance or exit room is nil", seed)
			break
		}

		entrance := entranceRoom.Center
		exit := m.ExitPoint

		if !m.IsWalkable(entrance) {
			t.Errorf("Seed %v: entrance %v is not walkable!", seed, entrance)
			break
		}

		if !m.IsWalkable(exit) {
			t.Errorf("Seed %v: exit %v is not walkable!", seed, exit)
			break
		}

		for _, room := range m.GetRooms() {
			pos := room.Pos
			xMin, yMin := pos.X, pos.Y
			xMax, yMax := xMin+room.Width-1, yMin+room.Height-1

			for y := yMin; y <= yMax; y++ {
				for x := xMin; x <= xMax; x++ {
					p := geometry.Point{X: x, Y: y}
					if !m.IsWalkable(p) {
						t.Errorf("Seed %v: room #%d tile %v is not walkable", seed, room.Id, p)
					}
				}
			}
		}

		if !isMapConnected(m, entrance, exit) {
			t.Errorf("Seed %v: map is not fully connected\n", seed)
			t.Errorf("\n%v", m)
			break
		}
	}
}

func TestGenerateTopologyDeterminism(t *testing.T) {
	tg := service.NewTopologyGenerator()
	var testSeed int64 = 42

	ctxA := createTestContext(testSeed)
	tg.Gen(ctxA, 80, 24, 3, 3)
	strA := ctxA.Playthrough.Map.String()

	ctxB := createTestContext(testSeed)
	tg.Gen(ctxB, 80, 24, 3, 3)
	strB := ctxB.Playthrough.Map.String()

	if strA != strB {
		t.Error("Maps with same seed differ in topology")
	}

	pA := ctxA.Playthrough.Map.GetEntranceRoom().Center
	pB := ctxB.Playthrough.Map.GetEntranceRoom().Center
	if pA != pB {
		t.Errorf("Entrance points differ: %v vs %v", pA, pB)
	}

	testSeed += 1
	ctxC := createTestContext(testSeed)
	tg.Gen(ctxC, 80, 24, 3, 3)

	if ctxA.Playthrough.Map.String() == ctxC.Playthrough.Map.String() {
		t.Error("Maps with different seeds are identical")
	}
}

func isMapConnected(m *model.Map, start, end geometry.Point) bool {
	queue := []geometry.Point{start}
	visited := make(map[geometry.Point]bool)
	visited[start] = true

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur == end {
			return true
		}

		for _, dir := range geometry.GetCardinalDirs() {
			n := cur.Add(dir)
			if m.IsWalkable(n) && !visited[n] {
				visited[n] = true
				queue = append(queue, n)
			}
		}
	}

	return false
}
