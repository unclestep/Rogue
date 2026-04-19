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

// buildPursuerObsMap produces a 15x9 open arena with walls on the border.
// Wide enough that the 11-cell crop around the centre (7,4) sits entirely
// inside the map, so channel values are fully testable.
func buildPursuerObsMap() *model.Map {
	const W, H = 15, 9

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

	bp := &model.MapBlueprint{
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
	return model.NewMapFromBlueprint(bp)
}

func freshMemory() *service.PursuerMemory {
	return service.NewPursuerMemory()
}

func memoryWithLastSeen(lastSeen geometry.Point, turnsSinceLOS int) *service.PursuerMemory {
	m := service.NewPursuerMemory()
	m.LastSeen = lastSeen
	m.TurnsSinceLOS = turnsSinceLOS
	return m
}

// pursuerAt builds a playthrough with one player (far away) and one Pursuer
// centred at the given position, returning (ctx, pursuer).
func pursuerAt(pos geometry.Point) (*model.SessionContext, *model.Actor) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildPursuerObsMap()

	player := model.NewDefaultPlayer(1, geometry.Point{X: 2, Y: 2}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	pursuer := model.NewDefaultPursuer(2, pos)
	play.Monsters[pursuer.Id] = pursuer
	play.Map.SetActor(pursuer.Pos, int64(pursuer.Id))

	return model.NewSessionContext(play), pursuer
}

//
// --- SHAPE AND RANGE ---
//

// Every spatial channel cell must be in [0, 1]. Values outside this range
// break fp16 quantisation and corrupt ONNX inference.
func TestBuildObservationAllGridValuesInUnitRange(t *testing.T) {
	ctx, pursuer := pursuerAt(geometry.Point{X: 7, Y: 4})
	obs := service.BuildObservation(ctx, pursuer, freshMemory())
	for i, v := range obs[:service.ObservationGridFloats] {
		if v < 0 || v > 1 {
			t.Errorf("obs[%d]=%v out of [0, 1]", i, v)
		}
	}
}

//
// --- SPATIAL CHANNELS ---
//

// The 11-cell crop centred on (7,4) in a 15x9 map fits entirely inside the
// arena. Interior cells are Floor (walkable); the outer ring should be 0
// where a wall intrudes.
func TestBuildObservationWalkableChannelMirrorsMap(t *testing.T) {
	ctx, pursuer := pursuerAt(geometry.Point{X: 7, Y: 4})
	obs := service.BuildObservation(ctx, pursuer, freshMemory())

	for y := 0; y < service.ObservationCropSize; y++ {
		for x := 0; x < service.ObservationCropSize; x++ {
			mapX := pursuer.Pos.X - service.ObservationHalfCrop + x
			mapY := pursuer.Pos.Y - service.ObservationHalfCrop + y
			v := service.ObservationCell(obs, service.ChWalkable, x, y)
			if mapX <= 0 || mapX >= 14 || mapY <= 0 || mapY >= 8 {
				if v != 0 {
					t.Errorf("wall at crop(%d,%d)/map(%d,%d) should be 0, got %v",
						x, y, mapX, mapY, v)
				}
			} else if v != 1 {
				t.Errorf("floor at crop(%d,%d)/map(%d,%d) should be 1, got %v",
					x, y, mapX, mapY, v)
			}
		}
	}
}

// Clear LOS between Pursuer and player, both inside the crop window.
// ChPlayerVisNow must light up exactly the player cell.
func TestBuildObservationPlayerVisibleWhenLOSClear(t *testing.T) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildPursuerObsMap()

	player := model.NewDefaultPlayer(1, geometry.Point{X: 5, Y: 4}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	pursuer := model.NewDefaultPursuer(2, geometry.Point{X: 7, Y: 4})
	play.Monsters[pursuer.Id] = pursuer
	play.Map.SetActor(pursuer.Pos, int64(pursuer.Id))

	ctx := model.NewSessionContext(play)
	obs := service.BuildObservation(ctx, pursuer, freshMemory())

	// Player offset from Pursuer: dx=-2, dy=0 → crop (3, 5).
	got := service.ObservationCell(obs, service.ChPlayerVisNow, 3, 5)
	if got != 1 {
		t.Errorf("expected ChPlayerVisNow=1 at crop(3,5), got %v", got)
	}
}

// Put a solid wall between player and Pursuer and verify ChPlayerVisNow is
// not set on any cell in the crop.
func TestBuildObservationPlayerHiddenByWall(t *testing.T) {
	const W, H = 11, 5
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
	for y := 1; y < H-1; y++ {
		grid[y][5] = model.Cell{Type: model.Wall, RoomId: 1}
	}

	bp := &model.MapBlueprint{
		Width: W, Height: H, TileGrid: grid,
		Rooms: []*model.Room{{
			Id: 1, Pos: geometry.Point{X: 1, Y: 1},
			Width: W - 2, Height: H - 2,
			Center: geometry.Point{X: W / 2, Y: H / 2},
		}},
		Doors:          map[geometry.Point]*model.DoorMetadata{},
		EntranceRoomId: 1,
		ExitRoomId:     1,
		ExitPoint:      model.NewInvalidPoint(),
	}

	play := model.NewPlaythrough("", 0, 0)
	play.Map = model.NewMapFromBlueprint(bp)

	player := model.NewDefaultPlayer(1, geometry.Point{X: 2, Y: 2}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	pursuer := model.NewDefaultPursuer(2, geometry.Point{X: 8, Y: 2})
	play.Monsters[pursuer.Id] = pursuer
	play.Map.SetActor(pursuer.Pos, int64(pursuer.Id))

	ctx := model.NewSessionContext(play)
	obs := service.BuildObservation(ctx, pursuer, freshMemory())

	for y := 0; y < service.ObservationCropSize; y++ {
		for x := 0; x < service.ObservationCropSize; x++ {
			if service.ObservationCell(obs, service.ChPlayerVisNow, x, y) != 0 {
				t.Errorf("expected ChPlayerVisNow=0 at crop(%d,%d) (wall blocks LOS)",
					x, y)
			}
		}
	}
}

//
// --- MEMORY CHANNEL ---
//

func TestBuildObservationMemoryChannelDecaysWithHorizon(t *testing.T) {
	ctx, pursuer := pursuerAt(geometry.Point{X: 7, Y: 4})
	lastSeen := geometry.Point{X: pursuer.Pos.X - 1, Y: pursuer.Pos.Y}

	fresh := service.BuildObservation(ctx, pursuer, memoryWithLastSeen(lastSeen, 0))
	// lastSeen offset dx=-1, dy=0 → crop (4, 5).
	if got := service.ObservationCell(fresh, service.ChPlayerMemory, 4, 5); got != 1 {
		t.Errorf("expected fresh ChPlayerMemory=1, got %v", got)
	}

	stale := service.BuildObservation(ctx, pursuer,
		memoryWithLastSeen(lastSeen, service.PursuerMemoryHorizon))
	if got := service.ObservationCell(stale, service.ChPlayerMemory, 4, 5); got != 0 {
		t.Errorf("expected stale ChPlayerMemory=0 at horizon, got %v", got)
	}
}

//
// --- SCALARS ---
//

// Half HP and half stamina must land at exactly 0.5 (±1e-4) confirming both
// the normalisation formula and the byte layout (scalars after the channel block).
func TestBuildObservationScalarsEncodeHalfVitals(t *testing.T) {
	ctx, pursuer := pursuerAt(geometry.Point{X: 7, Y: 4})

	pursuer.Vitals[model.VitalHP] = pursuer.DerivedAttrs[model.AttrMaxHP] / 2
	pursuer.Vitals[model.VitalStamina] = pursuer.DerivedAttrs[model.AttrMaxStamina] / 2

	obs := service.BuildObservation(ctx, pursuer, freshMemory())
	scalars := obs[service.ObservationGridFloats:]
	if len(scalars) != service.ObservationScalars {
		t.Fatalf("scalar slice length=%d want %d", len(scalars), service.ObservationScalars)
	}
	const eps = 1e-4
	if scalars[0] < 0.5-eps || scalars[0] > 0.5+eps {
		t.Errorf("hp_frac expected 0.5 ±%.0e, got %v", eps, scalars[0])
	}
	if scalars[1] < 0.5-eps || scalars[1] > 0.5+eps {
		t.Errorf("stamina_frac expected 0.5 ±%.0e, got %v", eps, scalars[1])
	}
}

// Full HP must encode as 1.0; zero HP must encode as 0.0.
// A normalisation bug (off-by-one, wrong max) fails at these extremes first.
func TestBuildObservationScalarsFullAndZeroHP(t *testing.T) {
	ctx, pursuer := pursuerAt(geometry.Point{X: 7, Y: 4})

	pursuer.Vitals[model.VitalHP] = pursuer.DerivedAttrs[model.AttrMaxHP]
	obs := service.BuildObservation(ctx, pursuer, freshMemory())
	if obs[service.ObservationGridFloats] != 1.0 {
		t.Errorf("full HP: hp_frac expected 1.0, got %v", obs[service.ObservationGridFloats])
	}

	pursuer.Vitals[model.VitalHP] = 0
	obs = service.BuildObservation(ctx, pursuer, freshMemory())
	if obs[service.ObservationGridFloats] != 0.0 {
		t.Errorf("zero HP: hp_frac expected 0.0, got %v", obs[service.ObservationGridFloats])
	}
}

// ChScent must be 1.0 at the player's own cell (BFS distance = 0 → value = 1.0).
// pursuerAt places the player at (2,2); with pursuer at (7,4) that maps to
// crop (0, 3). If writeChScent is broken or the scent map is missing this is 0.
func TestBuildObservationScentChannelHotAtPlayerPosition(t *testing.T) {
	// NewSessionContext builds the chase scent map automatically from player pos.
	ctx, pursuer := pursuerAt(geometry.Point{X: 7, Y: 4})
	obs := service.BuildObservation(ctx, pursuer, freshMemory())

	// player (2,2) relative to pursuer (7,4): dx=-5, dy=-2 → crop x=0, y=3.
	got := service.ObservationCell(obs, service.ChScent, 0, 3)
	if got < 0.99 {
		t.Errorf("ChScent at player crop (0,3) expected ≈1.0 (BFS dist=0), got %v", got)
	}
}

// ChAgeLastSeen must peak at 1.0 directly on the last-seen cell when
// TurnsSinceLOS=0, and fall off with Euclidean distance from that cell.
func TestBuildObservationAgeLastSeenPeaksAtLastSeenCell(t *testing.T) {
	ctx, pursuer := pursuerAt(geometry.Point{X: 7, Y: 4})
	// last_seen one cell left of pursuer: dx=-1, dy=0 → crop (4, 5).
	mem := memoryWithLastSeen(geometry.Point{X: 6, Y: 4}, 0)
	obs := service.BuildObservation(ctx, pursuer, mem)

	peak := service.ObservationCell(obs, service.ChAgeLastSeen, 4, 5)
	atPursuer := service.ObservationCell(obs, service.ChAgeLastSeen, 5, 5)
	if peak < 0.99 {
		t.Errorf("ChAgeLastSeen at last-seen cell expected ≈1.0, got %v", peak)
	}
	if peak <= atPursuer {
		t.Errorf("ChAgeLastSeen must peak at last-seen (%v) > at pursuer (%v)", peak, atPursuer)
	}
}

// ChPlayerMemory amplitude decays linearly with TurnsSinceLOS.
// At half-horizon the weight must be exactly 0.5 ±1e-4.
func TestBuildObservationMemoryDecaysAtHalfHorizon(t *testing.T) {
	ctx, pursuer := pursuerAt(geometry.Point{X: 7, Y: 4})
	lastSeen := geometry.Point{X: pursuer.Pos.X - 1, Y: pursuer.Pos.Y}
	halfHorizon := service.PursuerMemoryHorizon / 2 // 10

	obs := service.BuildObservation(ctx, pursuer, memoryWithLastSeen(lastSeen, halfHorizon))
	// last_seen dx=-1, dy=0 → crop (4, 5); weight = 1 - 10/20 = 0.5.
	got := service.ObservationCell(obs, service.ChPlayerMemory, 4, 5)
	const eps = 1e-4
	if got < 0.5-eps || got > 0.5+eps {
		t.Errorf("ChPlayerMemory at half-horizon expected 0.5 ±%.0e, got %v", eps, got)
	}
}
