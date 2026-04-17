package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
)

// End-to-end: generate a real dungeon through the canonical TopologyGenerator,
// flatten it to the dump DTO, and JSON-round-trip. Catches regressions in
// either the mapper (missing fields, wrong ordinals) or the Map getters the
// mapper relies on.
func TestToTopologyDumpMatchesGeneratedMap(t *testing.T) {
	const seed int64 = 123

	play := model.NewPlaythrough("dump-test", 1, seed)
	play.Map = &model.Map{}
	ctx := model.NewSessionContext(play)
	service.NewTopologyGenerator().Gen(ctx, 80, 24, 3, 3)

	if ctx.Playthrough.Map == nil {
		t.Fatalf("topology generator produced nil map for seed=%d", seed)
	}
	m := ctx.Playthrough.Map

	dump := dto.ToTopologyDump(m)

	if dump.Width != m.Width || dump.Height != m.Height {
		t.Fatalf("dim mismatch: dump=%dx%d map=%dx%d",
			dump.Width, dump.Height, m.Width, m.Height)
	}

	if len(dump.Tiles) != m.Height {
		t.Fatalf("expected %d rows in tiles, got %d", m.Height, len(dump.Tiles))
	}
	for y, row := range dump.Tiles {
		if len(row) != m.Width {
			t.Fatalf("row %d: expected width %d, got %d", y, m.Width, len(row))
		}
	}

	// Floor tile count must match between model and dump — guards against
	// accidental transposition or off-by-one in the mapper.
	modelFloorCount := 0
	grid := m.GetTileGrid()
	for y := range grid {
		for x := range grid[y] {
			if grid[y][x].Type == model.Floor {
				modelFloorCount++
			}
		}
	}
	dumpFloorCount := 0
	for _, row := range dump.Tiles {
		for _, t := range row {
			if t == int(model.Floor) {
				dumpFloorCount++
			}
		}
	}
	if modelFloorCount != dumpFloorCount {
		t.Errorf("floor-tile count mismatch: model=%d dump=%d",
			modelFloorCount, dumpFloorCount)
	}
	if dumpFloorCount == 0 {
		t.Error("dump has zero Floor tiles — mapper likely broken")
	}

	if len(dump.Rooms) != len(m.GetRooms()) {
		t.Errorf("room count mismatch: dump=%d map=%d",
			len(dump.Rooms), len(m.GetRooms()))
	}
	if dump.EntranceRoom == 0 || dump.ExitRoom == 0 {
		t.Errorf("entrance/exit room ids are zero (entrance=%d exit=%d)",
			dump.EntranceRoom, dump.ExitRoom)
	}
	if dump.ExitPoint.X < 0 || dump.ExitPoint.Y < 0 {
		t.Errorf("exit point invalid: %+v", dump.ExitPoint)
	}

	blob, err := json.Marshal(dump)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back dto.TopologyDump
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Width != dump.Width || back.Height != dump.Height ||
		len(back.Rooms) != len(dump.Rooms) {
		t.Fatalf("round-trip lost data: before=%+v after=%+v", dump, back)
	}
}

// Seed parity: identical seeds through the TopologyGenerator must produce
// byte-identical JSON. The Python training loop reuses a dumped fixture set
// across runs and relies on this determinism.
func TestToTopologyDumpDeterministicAcrossSeeds(t *testing.T) {
	const seed int64 = 42

	genOnce := func() []byte {
		play := model.NewPlaythrough("det", 1, seed)
		play.Map = &model.Map{}
		ctx := model.NewSessionContext(play)
		service.NewTopologyGenerator().Gen(ctx, 80, 24, 3, 3)

		blob, err := json.Marshal(dto.ToTopologyDump(ctx.Playthrough.Map))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return blob
	}

	a, b := genOnce(), genOnce()
	if string(a) != string(b) {
		t.Errorf("same seed produced different JSON\nA=%s\nB=%s", a, b)
	}
}
