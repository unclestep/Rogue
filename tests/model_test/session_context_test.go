package model_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

// --- NewSessionContext ---

func TestNewSessionContext_NoMap(t *testing.T) {
	p := setupPlaythrough()

	ctx := model.NewSessionContext(p)

	if ctx == nil {
		t.Fatal("Expected non-nil SessionContext")
	}
	if ctx.Playthrough != p {
		t.Errorf("Expected SessionContext to hold the provided Playthrough")
	}
	if ctx.ScentMaps == nil {
		t.Errorf("Expected ScentMaps map to be initialized")
	}
	// Without a map, BuildScentMaps is a no-op; the map should remain empty.
	if len(ctx.ScentMaps) != 0 {
		t.Errorf("Expected ScentMaps to be empty when no map is attached, got %d entries", len(ctx.ScentMaps))
	}
}

func TestNewSessionContextWithMap(t *testing.T) {
	p := setupPlaythrough()
	p.Map = setupTestMap()

	ctx := model.NewSessionContext(p)

	if _, ok := ctx.ScentMaps[model.ScentMapWander]; !ok {
		t.Errorf("Expected ScentMapWander to be built when map is present")
	}
	if _, ok := ctx.ScentMaps[model.ScentMapChase]; !ok {
		t.Errorf("Expected ScentMapChase to be built when map is present")
	}
}

// --- Rng ---

func TestRngReturnsSameInstance(t *testing.T) {
	p := setupPlaythrough()
	ctx := model.NewSessionContext(p)

	r1 := ctx.Rng()
	r2 := ctx.Rng()

	if r1 != r2 {
		t.Errorf("Expected Rng to return the same rand.Rand instance on every call")
	}
	if r1 == nil {
		t.Errorf("Expected Rng to return a non-nil rand.Rand")
	}
}

func TestRngDeterministicSequence(t *testing.T) {
	p1 := model.NewPlaythrough(1, 0, 12345)
	p2 := model.NewPlaythrough(2, 0, 12345)

	ctx1 := model.NewSessionContext(p1)
	ctx2 := model.NewSessionContext(p2)

	// Both contexts share the same seed; their RNG sequences must be identical.
	for i := 0; i < 10; i++ {
		v1 := ctx1.Rng().Int63()
		v2 := ctx2.Rng().Int63()
		if v1 != v2 {
			t.Errorf("Expected identical RNG values for equal seeds at step %d: ctx1=%d ctx2=%d", i, v1, v2)
		}
	}
}

func TestRngDifferentSeedsDifferentSequences(t *testing.T) {
	p1 := model.NewPlaythrough(1, 0, 1)
	p2 := model.NewPlaythrough(2, 0, 2)

	ctx1 := model.NewSessionContext(p1)
	ctx2 := model.NewSessionContext(p2)

	v1 := ctx1.Rng().Int63()
	v2 := ctx2.Rng().Int63()

	if v1 == v2 {
		t.Errorf("Expected different RNG sequences for different seeds, both produced %d", v1)
	}
}

// --- BuildScentMaps ---

func TestBuildScentMapsNoMapNoOp(t *testing.T) {
	p := setupPlaythrough()
	ctx := model.NewSessionContext(p)

	// Calling BuildScentMaps without a map must not panic and must leave ScentMaps unchanged.
	ctx.BuildScentMaps()

	if len(ctx.ScentMaps) != 0 {
		t.Errorf("Expected ScentMaps to remain empty, got %d entries", len(ctx.ScentMaps))
	}
}

func TestBuildScentMapsWanderMapDimensions(t *testing.T) {
	p := setupPlaythrough()
	p.Map = setupTestMap()

	ctx := model.NewSessionContext(p)

	wander, ok := ctx.ScentMaps[model.ScentMapWander]
	if !ok || wander == nil {
		t.Fatal("Expected ScentMapWander to be present")
	}

	mapDim := p.Map.GetDimensions()
	if len(wander.Scent) != mapDim.Y {
		t.Errorf("Expected wander scent map height=%d, got %d", mapDim.Y, len(wander.Scent))
	}
	if len(wander.Scent) > 0 && len(wander.Scent[0]) != mapDim.X {
		t.Errorf("Expected wander scent map width=%d, got %d", mapDim.X, len(wander.Scent[0]))
	}
}

func TestBuildScentMapsChaseMapWithPlayer(t *testing.T) {
	p := setupPlaythrough()
	p.Map = setupTestMap()

	playerPos := geometry.Point{X: 2, Y: 2}
	addPlayer(p, "uuid-1", 1, playerPos, 100)

	ctx := model.NewSessionContext(p)

	chase, ok := ctx.ScentMaps[model.ScentMapChase]
	if !ok || chase == nil {
		t.Fatal("Expected ScentMapChase to be present")
	}

	// The scent value at the player's position (the source) must be 0.
	if chase.Scent[playerPos.Y][playerPos.X] != 0 {
		t.Errorf("Expected scent=0 at player position %v, got %d", playerPos, chase.Scent[playerPos.Y][playerPos.X])
	}
}

func TestBuildScentMapsChaseMapNoPlayers(t *testing.T) {
	p := setupPlaythrough()
	p.Map = setupTestMap()

	// No players; BuildScentMaps must not panic.
	ctx := model.NewSessionContext(p)

	if _, ok := ctx.ScentMaps[model.ScentMapChase]; !ok {
		t.Errorf("Expected ScentMapChase key to be present even when there are no players")
	}
}

func TestBuildScentMapsRebuild(t *testing.T) {
	p := setupPlaythrough()
	p.Map = setupTestMap()

	ctx := model.NewSessionContext(p)

	// Add a player and rebuild; the chase map must now reflect the new player position.
	playerPos := geometry.Point{X: 1, Y: 1}
	addPlayer(p, "uuid-1", 1, playerPos, 100)

	ctx.BuildScentMaps()

	chase, ok := ctx.ScentMaps[model.ScentMapChase]
	if !ok || chase == nil {
		t.Fatal("Expected ScentMapChase after rebuild")
	}
	if chase.Scent[playerPos.Y][playerPos.X] != 0 {
		t.Errorf("Expected scent=0 at player position after rebuild, got %d", chase.Scent[playerPos.Y][playerPos.X])
	}
}
