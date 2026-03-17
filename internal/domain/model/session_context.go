package model

import (
	"math/rand"

	"github.com/unclestep/Rogue/pkg/geometry"
)

type SessionContext struct {
	Playthrough *Playthrough
	ScentMaps   map[ScentType]*ScentMap // Pre-calculate (update every turn)
	rng         *rand.Rand
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
		interest = append(interest, player.Pos)
	}

	ctx.ScentMaps[ScentMapChase] = &ScentMap{
		Scent: ctx.Playthrough.Map.GenerateScentMap(interest),
	}
}

func (ctx *SessionContext) Rng() *rand.Rand {
	return ctx.rng
}

type ScentType int

const (
	ScentMapWander ScentType = iota
	ScentMapChase
)

type ScentMap struct {
	Scent [][]int
}
