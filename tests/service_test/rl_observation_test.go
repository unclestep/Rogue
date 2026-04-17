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
// --- SHAPE ---
//

func TestBuildObservationLengthMatchesLayout(t *testing.T) {
	ctx, pursuer := pursuerAt(geometry.Point{X: 7, Y: 4})
	obs := service.BuildObservation(ctx, pursuer, freshMemory())
	if len(obs) != service.ObservationSize {
		t.Fatalf("Expected obs length %d, got %d", service.ObservationSize, len(obs))
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

// Half HP and half stamina should land exactly at 0.5 in the scalar block,
// confirming both the normalisation and the byte layout (scalars after the
// channel block).
func TestBuildObservationScalarsEncodeHalfVitals(t *testing.T) {
	ctx, pursuer := pursuerAt(geometry.Point{X: 7, Y: 4})

	pursuer.Vitals[model.VitalHP] = pursuer.DerivedAttrs[model.AttrMaxHP] / 2
	pursuer.Vitals[model.VitalStamina] = pursuer.DerivedAttrs[model.AttrMaxStamina] / 2

	obs := service.BuildObservation(ctx, pursuer, freshMemory())
	scalars := obs[service.ObservationGridFloats:]
	if len(scalars) != service.ObservationScalars {
		t.Fatalf("scalar slice length=%d want %d", len(scalars), service.ObservationScalars)
	}
	if scalars[0] < 0.49 || scalars[0] > 0.51 {
		t.Errorf("hp_frac scalar expected ~0.5, got %v", scalars[0])
	}
	if scalars[1] < 0.49 || scalars[1] > 0.51 {
		t.Errorf("stamina_frac scalar expected ~0.5, got %v", scalars[1])
	}
}
