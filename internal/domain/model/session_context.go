package model

import (
	"math/rand"

	"github.com/unclestep/Rogue/pkg/geometry"
)

type SessionContext struct {
	Playthrough *Playthrough
	ScentMaps   map[ScentType]*ScentMap // Pre-calculate (update every turn)
	rng         *rand.Rand

	// exitScentCache is lazily filled by ExitScentMap() on first use.
	// It is intentionally NOT computed inside BuildScentMaps() because the map's
	// ExitPoint is not valid until the dungeon generator has finished populating
	// the map; many test helpers construct SessionContext before the map is
	// fully populated, so eager computation would panic on {-1,-1}.
	exitScentCache *ScentMap
}

func NewSessionContext(p *Playthrough) *SessionContext {
	ctx := &SessionContext{
		Playthrough: p,
		ScentMaps:   make(map[ScentType]*ScentMap, 3),
		rng:         rand.New(rand.NewSource(p.Seed)),
	}
	ctx.BuildScentMaps()
	return ctx
}

func (ctx *SessionContext) BuildScentMaps() {
	if ctx.Playthrough.Map == nil {
		return
	}

	ctx.buildWanderMap()
	ctx.buildChaseMap()
}

func (ctx *SessionContext) buildWanderMap() {
	interest := make([]geometry.Point, 0, len(ctx.Playthrough.Map.rooms))

	for _, room := range ctx.Playthrough.Map.rooms {
		xRnd := room.Pos.X + ctx.rng.Intn(room.Width)
		yRnd := room.Pos.Y + ctx.rng.Intn(room.Height)
		interest = append(interest, geometry.Point{X: xRnd, Y: yRnd})
	}

	ctx.ScentMaps[ScentMapWander] = &ScentMap{
		Scent: ctx.Playthrough.Map.GenerateScentMap(interest),
	}
}

func (ctx *SessionContext) buildChaseMap() {
	interest := make([]geometry.Point, 0, len(ctx.Playthrough.Players))

	for _, player := range ctx.Playthrough.Players {
		// Skip hidden players — their Pos is NewInvalidPoint() = {-1,-1} which
		// would cause an out-of-bounds access inside GenerateScentMap.
		if player.Pos != NewInvalidPoint() {
			interest = append(interest, player.Pos)
		}
	}

	ctx.ScentMaps[ScentMapChase] = &ScentMap{
		Scent: ctx.Playthrough.Map.GenerateScentMap(interest),
	}
}

func (ctx *SessionContext) Rng() *rand.Rand {
	return ctx.rng
}

// ExitScentMap lazily builds (and caches) a BFS distance grid from the map's
// ExitPoint over walkable cells. Returns nil when the map or ExitPoint is not
// ready — callers must handle nil (treat as "no exit info"). See the cache
// field docstring for why this is not eager.
func (ctx *SessionContext) ExitScentMap() *ScentMap {
	if ctx.exitScentCache != nil {
		return ctx.exitScentCache
	}
	if ctx.Playthrough == nil || ctx.Playthrough.Map == nil {
		return nil
	}
	exit := ctx.Playthrough.Map.ExitPoint
	if exit == NewInvalidPoint() {
		return nil
	}
	if exit.X < 0 || exit.Y < 0 ||
		exit.X >= ctx.Playthrough.Map.Width ||
		exit.Y >= ctx.Playthrough.Map.Height {
		return nil
	}
	ctx.exitScentCache = &ScentMap{
		Scent: ctx.Playthrough.Map.GenerateScentMap([]geometry.Point{exit}),
	}
	return ctx.exitScentCache
}

type ScentType int

const (
	ScentMapWander ScentType = iota
	ScentMapChase
)

type ScentMap struct {
	Scent [][]int
}
