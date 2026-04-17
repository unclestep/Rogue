// Smoke test wiring ONNXPolicy through the real MonsterController loop.
// Verifies that N turns run without panics and produce valid Pursuer intents
// when the trained model (or at least the tiny argmax-always-0 testdata) is
// present and ORT native library resolvable. Skips gracefully when ORT isn't
// installed so local runs stay green.
package application_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func TestPursuerONNXPolicySmoke(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	modelPath := filepath.Join(repoRoot, "internal", "domain", "service", "testdata", "pursuer_tiny.onnx")
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("tiny onnx model missing at %s: %v", modelPath, err)
	}

	policy, err := service.NewONNXPolicy(modelPath)
	if err != nil {
		// ORT shared library usually isn't installed locally; CI wires it via
		// the workflow. Skip rather than fail so `make test` stays green.
		t.Skipf("ORT not available — skipping ONNX smoke: %v", err)
	}
	if policy == nil {
		t.Fatal("NewONNXPolicy returned nil without error")
	}

	play := model.NewPlaythrough("smoke", 0, 0)
	play.Map = buildSmokeMap()

	player := model.NewDefaultPlayer(1, geometry.Point{X: 2, Y: 2}, 8)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	pursuer := model.NewDefaultPursuer(42, geometry.Point{X: 6, Y: 6})
	play.Monsters[pursuer.Id] = pursuer
	play.Map.SetActor(pursuer.Pos, int64(pursuer.Id))

	pathfinder := service.NewPathfinderService()
	resolver := service.NewMoveResolverService()
	raycaster := service.NewRaycaster()
	ctrl := service.NewMonsterControllerService(pathfinder, resolver, raycaster, policy)
	ctx := model.NewSessionContext(play)

	const turns = 10
	validIntents := 0
	for turn := 0; turn < turns; turn++ {
		intents := ctrl.Tick(ctx)
		for _, it := range intents {
			if it == nil || it.Actor != pursuer.Id {
				continue
			}
			if it.IntentType != model.IntentMove && it.IntentType != model.IntentAttack && it.IntentType != model.IntentUnknown {
				t.Errorf("turn %d: unexpected IntentType %v", turn, it.IntentType)
			}
			validIntents++
		}
	}
	if validIntents == 0 {
		t.Fatal("ONNX-driven Pursuer produced no intents over 10 turns")
	}
}

func buildSmokeMap() *model.Map {
	const W, H = 12, 12
	grid := make([][]model.Cell, H)
	for y := range grid {
		grid[y] = make([]model.Cell, W)
		for x := range grid[y] {
			t := model.Floor
			if y == 0 || y == H-1 || x == 0 || x == W-1 {
				t = model.Wall
			}
			grid[y][x] = model.Cell{Type: t, RoomId: 1}
		}
	}
	blueprint := &model.MapBlueprint{
		Width:    W,
		Height:   H,
		TileGrid: grid,
		Rooms: []*model.Room{{
			Id:     1,
			Pos:    geometry.Point{X: 1, Y: 1},
			Width:  W - 2,
			Height: H - 2,
			Center: geometry.Point{X: W / 2, Y: H / 2},
		}},
		Doors:          map[geometry.Point]*model.DoorMetadata{},
		EntranceRoomId: 1,
		ExitRoomId:     1,
		ExitPoint:      model.NewInvalidPoint(),
	}
	return model.NewMapFromBlueprint(blueprint)
}
