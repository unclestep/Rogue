package service_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/geometry"
)

//
//
// --- HELPERS ---
//
//

// buildPathfinderOpenMap creates a 9x9 map with walls on all borders
// and floor tiles for all interior cells, all belonging to room 1.
func buildPathfinderOpenMap() *model.Map {
	const W, H = 9, 9

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
		Rooms: []*model.Room{
			{
				Id:     1,
				Pos:    geometry.Point{X: 1, Y: 1},
				Width:  W - 2,
				Height: H - 2,
				Center: geometry.Point{X: W / 2, Y: H / 2},
			},
		},
		Doors:          map[geometry.Point]*model.DoorMetadata{},
		EntranceRoomId: 1,
		ExitRoomId:     1,
		ExitPoint:      model.NewInvalidPoint(),
	}

	return model.NewMapFromBlueprint(blueprint)
}

// buildCorridorMap creates a 7x3 map with a single walkable corridor at Y=1.
// The center tile at (3,1) is set to midTileType, allowing door or floor tests.
func buildCorridorMap(midTileType model.TileType) *model.Map {
	const W, H = 7, 3

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
	grid[1][3] = model.Cell{Type: midTileType, RoomId: 1}

	blueprint := &model.MapBlueprint{
		Width:    W,
		Height:   H,
		TileGrid: grid,
		Rooms: []*model.Room{
			{
				Id:     1,
				Pos:    geometry.Point{X: 1, Y: 1},
				Width:  W - 2,
				Height: 1,
				Center: geometry.Point{X: W / 2, Y: 1},
			},
		},
		Doors:          map[geometry.Point]*model.DoorMetadata{},
		EntranceRoomId: 1,
		ExitRoomId:     1,
		ExitPoint:      model.NewInvalidPoint(),
	}

	return model.NewMapFromBlueprint(blueprint)
}

// buildSplitMap creates a 9x9 map with a solid wall column at X=4,
// creating two isolated floor regions: left (X=1..3) and right (X=5..7).
func buildSplitMap() *model.Map {
	const W, H = 9, 9

	grid := make([][]model.Cell, H)
	for y := range grid {
		grid[y] = make([]model.Cell, W)
		for x := range grid[y] {
			t := model.Floor
			if y == 0 || y == H-1 || x == 0 || x == W-1 || x == 4 {
				t = model.Wall
			}
			grid[y][x] = model.Cell{Type: t, RoomId: 1}
		}
	}

	blueprint := &model.MapBlueprint{
		Width:    W,
		Height:   H,
		TileGrid: grid,
		Rooms: []*model.Room{
			{
				Id:     1,
				Pos:    geometry.Point{X: 1, Y: 1},
				Width:  3,
				Height: H - 2,
				Center: geometry.Point{X: 2, Y: H / 2},
			},
			{
				Id:     2,
				Pos:    geometry.Point{X: 5, Y: 1},
				Width:  3,
				Height: H - 2,
				Center: geometry.Point{X: 6, Y: H / 2},
			},
		},
		Doors:          map[geometry.Point]*model.DoorMetadata{},
		EntranceRoomId: 1,
		ExitRoomId:     2,
		ExitPoint:      model.NewInvalidPoint(),
	}

	return model.NewMapFromBlueprint(blueprint)
}

// makeScentMap creates an H-by-W scent grid with every cell initialized to fill.
func makeScentMap(h, w, fill int) [][]int {
	sm := make([][]int, h)
	for y := range sm {
		sm[y] = make([]int, w)
		for x := range sm[y] {
			sm[y][x] = fill
		}
	}
	return sm
}

func newPathfinderCtx(m *model.Map) *model.SessionContext {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = m
	return model.NewSessionContext(play)
}

func newPathfinder() *service.Pathfinder {
	return service.NewPathfinderService()
}

//
//
// --- A* FIND: ENDPOINT VALIDATION ---
//
//

//
// -- Valid endpoints --
//

func TestAFindTwoFloorTilesReturnsPathStartingAndEndingAtGivenPoints(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()

	p1 := geometry.Point{X: 1, Y: 1}
	p2 := geometry.Point{X: 7, Y: 7}

	path, ok := pf.AFind(m, p1, p2)

	if !ok {
		t.Fatalf("Expected AFind to return true for two connected floor tiles, got false")
	}
	if len(path) == 0 {
		t.Fatalf("Expected non-empty path, got empty slice")
	}
	if path[0] != p1 {
		t.Errorf("Expected path[0]=%v (start), got %v", p1, path[0])
	}
	if path[len(path)-1] != p2 {
		t.Errorf("Expected path[last]=%v (end), got %v", p2, path[len(path)-1])
	}
}

// When p1==p2, A* immediately matches start==target.
// reconstructPath returns a single-element slice [p].
func TestAFindSameStartAndEndReturnsSingletonPath(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()

	p := geometry.Point{X: 4, Y: 4}

	path, ok := pf.AFind(m, p, p)

	if !ok {
		t.Fatalf("Expected AFind(p, p) to succeed, got false")
	}
	if len(path) != 1 {
		t.Errorf("Expected path of length 1 when start==end, got %d", len(path))
	}
	if len(path) > 0 && path[0] != p {
		t.Errorf("Expected singleton path element to be %v, got %v", p, path[0])
	}
}

//
// -- Unwalkable endpoints --
//

func TestAFindWallStartReturnsNilFalse(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()

	wallStart := geometry.Point{X: 0, Y: 0}
	floorEnd := geometry.Point{X: 7, Y: 7}

	path, ok := pf.AFind(m, wallStart, floorEnd)

	if ok {
		t.Errorf("Expected false when start is a wall tile, got true")
	}
	if path != nil {
		t.Errorf("Expected nil path when start is a wall tile, got %v", path)
	}
}

func TestAFindWallEndReturnsNilFalse(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()

	floorStart := geometry.Point{X: 1, Y: 1}
	wallEnd := geometry.Point{X: 8, Y: 8}

	path, ok := pf.AFind(m, floorStart, wallEnd)

	if ok {
		t.Errorf("Expected false when end is a wall tile, got true")
	}
	if path != nil {
		t.Errorf("Expected nil path when end is a wall tile, got %v", path)
	}
}

// ClosedDoor is not walkable (IsWalkable returns false for ClosedDoor),
// so using it as a start point must return nil, false without entering A*.
func TestAFindClosedDoorStartReturnsNilFalse(t *testing.T) {
	pf := newPathfinder()
	// Corridor map has ClosedDoor at (3,1).
	m := buildCorridorMap(model.ClosedDoor)

	closedDoorStart := geometry.Point{X: 3, Y: 1}
	floorEnd := geometry.Point{X: 5, Y: 1}

	path, ok := pf.AFind(m, closedDoorStart, floorEnd)

	if ok {
		t.Errorf("Expected false when start is a ClosedDoor (not walkable), got true")
	}
	if path != nil {
		t.Errorf("Expected nil path when start is ClosedDoor, got %v", path)
	}
}

//
//
// --- A* FIND: BLOCKED PATHS ---
//
//

//
// -- Wall barrier --
//

func TestAFindNoPathThroughWallBarrierReturnsNilFalse(t *testing.T) {
	pf := newPathfinder()
	// Split map has a solid wall column at X=4 separating left and right regions.
	m := buildSplitMap()

	leftFloor := geometry.Point{X: 1, Y: 4}
	rightFloor := geometry.Point{X: 7, Y: 4}

	path, ok := pf.AFind(m, leftFloor, rightFloor)

	if ok {
		t.Errorf("Expected false when map is fully split by a wall column, got true")
	}
	if path != nil {
		t.Errorf("Expected nil path when no route exists, got %v", path)
	}
}

//
// -- Closed door as the only passage --
//

// getFollowingCost assigns math.Inf(1) to ClosedDoor (default case in switch).
// A* skips neighbors with infinite cost, so ClosedDoor is impassable mid-path.
func TestAFindClosedDoorOnOnlyRouteReturnsNilFalse(t *testing.T) {
	pf := newPathfinder()
	m := buildCorridorMap(model.ClosedDoor)

	p1 := geometry.Point{X: 1, Y: 1}
	p2 := geometry.Point{X: 5, Y: 1}

	path, ok := pf.AFind(m, p1, p2)

	if ok {
		t.Errorf("Expected false when only route passes through ClosedDoor (Inf cost), got true")
	}
	if path != nil {
		t.Errorf("Expected nil path when ClosedDoor blocks only route, got %v", path)
	}
}

//
//
// --- A* FIND: PASSABLE TILES ---
//
//

//
// -- Open door --
//

// OpenDoor has cost 1.0 in getFollowingCost and satisfies IsWalkable.
// A path that passes through it must be found and include the door tile.
func TestAFindOpenDoorOnOnlyRouteIsPassable(t *testing.T) {
	pf := newPathfinder()
	m := buildCorridorMap(model.OpenDoor)

	p1 := geometry.Point{X: 1, Y: 1}
	p2 := geometry.Point{X: 5, Y: 1}

	path, ok := pf.AFind(m, p1, p2)

	if !ok {
		t.Fatalf("Expected AFind to succeed through OpenDoor, got false")
	}
	if len(path) == 0 {
		t.Fatalf("Expected non-empty path through OpenDoor")
	}

	doorPos := geometry.Point{X: 3, Y: 1}
	passedThroughDoor := false
	for _, pt := range path {
		if pt == doorPos {
			passedThroughDoor = true
			break
		}
	}
	if !passedThroughDoor {
		t.Errorf("Expected path to include OpenDoor tile at %v, but it did not. Path: %v", doorPos, path)
	}
}

//
// -- Actor in path --
//

// An actor on a tile raises that tile's cost from 1.0 to 6.0 (1 base + 5 penalty)
// but does not raise it to Inf. A path is still found on the open map.
func TestAFindActorOnTileIncreasesPathCostButDoesNotBlockRoute(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	actor := model.NewDefaultZombie(10, geometry.Point{X: 4, Y: 4})
	m.SetActor(actor.Pos, int64(actor.Id))

	p1 := geometry.Point{X: 1, Y: 1}
	p2 := geometry.Point{X: 7, Y: 7}

	path, ok := pf.AFind(m, p1, p2)

	if !ok {
		t.Fatalf("Expected path to be found even with an actor occupying one tile, got false")
	}
	if len(path) == 0 {
		t.Errorf("Expected non-empty path when actor is present on the map")
	}
}

//
//
// --- DIJKSTRA FIND: CARDINAL (MovePatternDefault) ---
//
//

//
// -- Correct direction selection --
//

// dijkstraCardinal scans north (Y-1) and south (Y+1) at center.X,
// and west (X-1) and east (X+1) at center.Y.
func TestDijkstraFindCardinalMovesToLowerScentNorth(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultZombie(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	// scentMap[y][x]: lower = closer to player.
	sm := makeScentMap(9, 9, 10)
	sm[4][4] = 10 // mover
	sm[3][4] = 5  // north neighbor (X=4,Y=3): lower than mover, only lower cardinal

	result := pf.DijkstraFind(ctx, sm, mover)

	expected := geometry.Point{X: 4, Y: 3}
	if result != expected {
		t.Errorf("Expected cardinal mover to step north to %v, got %v", expected, result)
	}
}

func TestDijkstraFindCardinalMovesToLowerScentEast(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultZombie(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	sm := makeScentMap(9, 9, 10)
	sm[4][4] = 10
	sm[4][5] = 5 // east neighbor (X=5,Y=4): only lower cardinal

	result := pf.DijkstraFind(ctx, sm, mover)

	expected := geometry.Point{X: 5, Y: 4}
	if result != expected {
		t.Errorf("Expected cardinal mover to step east to %v, got %v", expected, result)
	}
}

//
// -- No improvement available --
//

func TestDijkstraFindCardinalStaysWhenAllNeighborsHaveHigherScent(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultZombie(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	// Mover has the lowest scent in the area — no cardinal neighbor qualifies.
	sm := makeScentMap(9, 9, 20)
	sm[4][4] = 5

	result := pf.DijkstraFind(ctx, sm, mover)

	if result != mover.Pos {
		t.Errorf("Expected cardinal mover to stay at %v when all neighbors have higher scent, got %v",
			mover.Pos, result)
	}
}

//
// -- Diagonal neighbors must be ignored --
//

// dijkstraCardinal never visits (center.X±1, center.Y±1) diagonal cells.
// Even a drastically lower diagonal scent must not cause movement.
func TestDijkstraFindCardinalIgnoresDiagonalNeighborWithLowerScent(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultZombie(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	sm := makeScentMap(9, 9, 10)
	sm[4][4] = 10
	sm[3][3] = 1 // NW diagonal (X=3,Y=3): very low — not a cardinal neighbor
	// All four cardinal neighbors remain at 10 → no improvement → stays.

	result := pf.DijkstraFind(ctx, sm, mover)

	if result.Equal(geometry.Point{X: 3, Y: 3}) {
		t.Errorf(
			"Expected cardinal mover to stay at %v (NW diagonal must not be visited), got %v",
			mover.Pos, result,
		)
	}
}

//
// -- Obstacle handling --
//

func TestDijkstraFindCardinalSkipsUnwalkableWallNeighbor(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	// At (1,1), north (1,0) and west (0,1) are wall border tiles.
	mover := model.NewDefaultZombie(1, geometry.Point{X: 1, Y: 1})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	sm := makeScentMap(9, 9, 10)
	sm[1][1] = 10
	sm[0][1] = 1 // north (1,0): wall, must be skipped by IsWalkable
	sm[1][0] = 1 // west (0,1): wall, must be skipped
	sm[2][1] = 5 // south (1,2): floor, lower scent
	sm[1][2] = 5 // east (2,1): floor, lower scent

	result := pf.DijkstraFind(ctx, sm, mover)

	southFloor := geometry.Point{X: 1, Y: 2}
	eastFloor := geometry.Point{X: 2, Y: 1}

	if result != southFloor && result != eastFloor {
		t.Errorf(
			"Expected cardinal mover to move south or east (wall neighbors skipped), got %v",
			result,
		)
	}
}

// A cell occupied by another monster is skipped by the monster-to-monster collision check.
// Condition: IsMonster(actorId) && mover.Kind != ActorPlayer
func TestDijkstraFindCardinalSkipsMonsterOccupiedNeighbor(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultZombie(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	// Block all tiles around mover
	sm := makeScentMap(9, 9, 10)
	sm[4][4] = 10

	for i := mover.Pos.Y - 1; i <= mover.Pos.Y+1; i++ {
		for j := mover.Pos.X - 1; j <= mover.Pos.X+1; j++ {
			p := geometry.Point{X: j, Y: i}
			if p != mover.Pos {
				blocker := model.NewDefaultZombie(2, geometry.Point{X: j, Y: i})
				m.SetActor(blocker.Pos, int64(blocker.Id))
				ctx.Playthrough.Monsters[blocker.Id] = blocker
				sm[i][j] = 1
			}
		}
	}

	result := pf.DijkstraFind(ctx, sm, mover)

	if result != mover.Pos {
		t.Errorf(
			"Expected mover to stay at %v (north occupied by monster), got %v",
			mover.Pos, result,
		)
	}
}

// IsMonster returns false for a player actor, so player-occupied cells are
// NOT blocked for a monster mover — the monster can step into them.
func TestDijkstraFindCardinalPlayerOccupiedCellIsNotBlocked(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultZombie(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	player := model.NewDefaultPlayer(99, geometry.Point{X: 4, Y: 3}, 9)
	m.SetActor(player.Pos, int64(player.Id))
	ctx.Playthrough.Players[player.Id] = player

	sm := makeScentMap(9, 9, 10)
	sm[4][4] = 10
	sm[3][4] = 1 // north: player occupies it, but monster collision check must not block it

	result := pf.DijkstraFind(ctx, sm, mover)

	expected := geometry.Point{X: 4, Y: 3}
	if result != expected {
		t.Errorf(
			"Expected monster to move into player cell %v (player does not block monster), got %v",
			expected, result,
		)
	}
}

//
// -- Map boundary safety --
//

func TestDijkstraFindCardinalMoverAtMapCornerDoesNotPanic(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultZombie(1, geometry.Point{X: 1, Y: 1})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	sm := makeScentMap(9, 9, 10)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("DijkstraFind panicked for mover at map corner (1,1): %v", r)
		}
	}()

	pf.DijkstraFind(ctx, sm, mover)
}

// scentMap[center.Y][center.X] is accessed before any bounds check.
// If mover.Pos is an invalid point (-1,-1), the negative index causes a runtime panic.
func TestDijkstraFindCardinalInvalidMoverPositionPanics(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	// Mover at InvalidPoint (-1,-1): simulates a removed actor still processed.
	mover := model.NewDefaultZombie(1, model.NewInvalidPoint())
	ctx.Playthrough.Monsters[mover.Id] = mover

	sm := makeScentMap(9, 9, 10)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf(
				"DijkstraFind panicked for mover at InvalidPoint %v: %v. "+
					"scentMap[center.Y][center.X] is read with a negative index before any bounds check.",
				mover.Pos, r,
			)
		}
	}()

	pf.DijkstraFind(ctx, sm, mover)
}

//
//
// --- DIJKSTRA FIND: DIAGONAL (MovePatternDiagonal) ---
//
//

//
// -- Only NW / SW / SE / NE are examined --
//

// dijkstraDiagonal iterates x from xMin to xMax with a paired upper/bottom row.
// When x == center.X (middle column), both upper and bottom resolve to center,
// which is skipped. This means N (X=center,Y-1) and S (X=center,Y+1) are
// never evaluated — a snake-mage cannot move cardinally.
func TestDijkstraFindDiagonalMovesToLowerScentNorthWest(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultSnakeMage(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	sm := makeScentMap(9, 9, 10)
	sm[4][4] = 10
	sm[3][3] = 3 // NW diagonal (X=3,Y=3): lower — only lower diagonal candidate

	result := pf.DijkstraFind(ctx, sm, mover)

	expected := geometry.Point{X: 3, Y: 3}
	if result != expected {
		t.Errorf("Expected diagonal mover to step NW to %v, got %v", expected, result)
	}
}

func TestDijkstraFindDiagonalMovesToLowerScentSouthEast(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultSnakeMage(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	sm := makeScentMap(9, 9, 10)
	sm[4][4] = 10
	sm[5][5] = 2 // SE diagonal (X=5,Y=5): only lower diagonal candidate

	result := pf.DijkstraFind(ctx, sm, mover)

	expected := geometry.Point{X: 5, Y: 5}
	if result != expected {
		t.Errorf("Expected diagonal mover to step SE to %v, got %v", expected, result)
	}
}

//
// -- Cardinal neighbors silently ignored --
//

// dijkstraDiagonal processes the center X column as center-point pairs,
// so N (X=center,Y-1) is never a candidate. A monster that must travel
// straight north will instead stay put even when north has the lowest scent.
func TestDijkstraFindDiagonalIgnoresCardinalNorthWithLowerScent(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultSnakeMage(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	sm := makeScentMap(9, 9, 10)
	sm[4][4] = 10
	sm[3][4] = 1 // north cardinal (X=4,Y=3): very low — never visited by diagonal finder
	// All diagonal neighbors stay at 10 → no improvement → stays.

	result := pf.DijkstraFind(ctx, sm, mover)

	if result.Equal(geometry.Point{X: 3, Y: 4}) {
		t.Errorf(
			"Expected diagonal mover to stay at %v (north cardinal not examined by dijkstraDiagonal), got %v. "+
				"When x==center.X the upper/bottom row both equal center and are skipped, "+
				"so N and S are permanently invisible to the diagonal finder.",
			mover.Pos, result,
		)
	}
}

func TestDijkstraFindDiagonalIgnoresCardinalEastWithLowerScent(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultSnakeMage(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	sm := makeScentMap(9, 9, 10)
	sm[4][4] = 10
	sm[4][5] = 1 // east cardinal (X=5,Y=4): lower — but Y==center.Y is only in second
	// Note: the diagonal finder loops over (upper,bottom) pairs and never visits Y=center.Y with X!=center.X.
	// All diagonal neighbors stay at 10 → stays.

	result := pf.DijkstraFind(ctx, sm, mover)

	if result.Equal(geometry.Point{X: 4, Y: 5}) {
		t.Errorf(
			"Expected diagonal mover to stay at %v (east cardinal not examined by dijkstraDiagonal), got %v",
			mover.Pos, result,
		)
	}
}

//
// -- No diagonal improvement --
//

func TestDijkstraFindDiagonalStaysWhenNoDiagonalHasLowerScent(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultSnakeMage(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	// Mover has the lowest scent among all examined points.
	sm := makeScentMap(9, 9, 15)
	sm[4][4] = 5

	result := pf.DijkstraFind(ctx, sm, mover)

	if result != mover.Pos {
		t.Errorf(
			"Expected diagonal mover to stay at %v when no diagonal neighbor has lower scent, got %v",
			mover.Pos, result,
		)
	}
}

//
//
// --- DIJKSTRA FIND: MOORE (MovePatternTeleport) ---
//
//

//
// -- All 8 neighbors considered --
//

func TestDijkstraFindMooreMovesToLowerScentDiagonal(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultGhost(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	sm := makeScentMap(9, 9, 10)
	sm[4][4] = 10
	sm[3][3] = 2 // NW diagonal (X=3,Y=3): Moore considers all 8 neighbors

	result := pf.DijkstraFind(ctx, sm, mover)

	expected := geometry.Point{X: 3, Y: 3}
	if result != expected {
		t.Errorf("Expected Moore mover to step NW to %v (diagonal must be considered), got %v",
			expected, result)
	}
}

func TestDijkstraFindMooreMovesToLowerScentCardinalNorth(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultGhost(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	sm := makeScentMap(9, 9, 10)
	sm[4][4] = 10
	sm[3][4] = 2 // north cardinal (X=4,Y=3): lower — Moore must see it unlike dijkstraDiagonal

	result := pf.DijkstraFind(ctx, sm, mover)

	expected := geometry.Point{X: 4, Y: 3}
	if result != expected {
		t.Errorf("Expected Moore mover to step north to %v (cardinal must be considered), got %v",
			expected, result)
	}
}

func TestDijkstraFindMooreStaysWhenAllNeighborsHigherScent(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultGhost(1, geometry.Point{X: 4, Y: 4})
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	sm := makeScentMap(9, 9, 30)
	sm[4][4] = 5 // mover has minimum scent in neighborhood

	result := pf.DijkstraFind(ctx, sm, mover)

	if result != mover.Pos {
		t.Errorf("Expected Moore mover to stay at %v when all 8 neighbors have higher scent, got %v",
			mover.Pos, result)
	}
}

//
//
// --- DIJKSTRAFIND: UNREGISTERED PATTERN ---
//
//

// DijkstraFind falls back to mover.Pos when mover.MovePattern has no registered finder.
// Registered: MovePatternDefault(0), MovePatternTeleport(1), MovePatternDiagonal(2).
func TestDijkstraFindUnregisteredPatternReturnsMoverPos(t *testing.T) {
	pf := newPathfinder()
	m := buildPathfinderOpenMap()
	ctx := newPathfinderCtx(m)

	mover := model.NewDefaultZombie(1, geometry.Point{X: 4, Y: 4})
	mover.MovePattern = model.MovePatternType(99) // not in dijkstraFinders map
	m.SetActor(mover.Pos, int64(mover.Id))
	ctx.Playthrough.Monsters[mover.Id] = mover

	// All neighbors have very low scent; a working finder would move.
	sm := makeScentMap(9, 9, 1)
	sm[4][4] = 50

	result := pf.DijkstraFind(ctx, sm, mover)

	if result != mover.Pos {
		t.Errorf("Expected unregistered MovePattern to return mover.Pos=%v, got %v",
			mover.Pos, result)
	}
}
