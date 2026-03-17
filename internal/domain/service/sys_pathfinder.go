package service

import (
	"math"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/algorithm"
	"github.com/unclestep/Rogue/pkg/geometry"
)

type Pathfinder struct {
	dijkstraFinders map[model.MovePatternType]DijkstraFinder
}

type DijkstraFinder interface {
	Find(ctx *model.SessionContext, scentMap [][]int, mover *model.Actor) geometry.Point
}

func NewPathfinderService() *Pathfinder {
	p := &Pathfinder{
		dijkstraFinders: make(map[model.MovePatternType]DijkstraFinder),
	}
	p.RegisterDijkstraFinders()
	return p
}

func (p *Pathfinder) RegisterDijkstraFinders() {
	p.dijkstraFinders[model.MovePatternDefault] = dijkstraCardinal{}
	p.dijkstraFinders[model.MovePatternDiagonal] = dijkstraDiagonal{}
	p.dijkstraFinders[model.MovePatternTeleport] = dijkstraMoore{}
}

//
//
// --- A* Pathfinder ---
//
//

// FindPath - finds the path between two points in map's topology.
// Algorithm can only pass through corridors, floors and open doors.
// It tries to avoid the cells with items and actors.
// Returns slice of points between two given points including them and bool value, if it could find the way.
func (p *Pathfinder) AFind(m *model.Map, p1, p2 geometry.Point) ([]geometry.Point, bool) {
	if !m.IsWalkable(p1) || !m.IsWalkable(p2) {
		return nil, false
	}

	followingGraph := &followingGraph{m: m}

	path := algorithm.FindPath(followingGraph, p1, p2, followingGraph.getFollowingCost)

	if path == nil {
		return nil, false
	}

	return path, true
}

type followingGraph struct {
	m *model.Map
}

// getFollowingCost - cost function to find the way between points inside the game map: floors, open doors, corridors.
func (f *followingGraph) getFollowingCost(p geometry.Point) float64 {
	tile, _ := f.m.GetTileType(p)
	cost := 0.0

	switch tile {
	case model.Corridor, model.Floor, model.OpenDoor, model.Exit:
		cost = 1.0
	default:
		cost = math.Inf(1)
	}

	if f.m.IsActor(p) || f.m.IsItem(p) {
		cost += 5.0
	}

	return cost
}

// GetNeighbors - returns a slice of neighboring points
// Returns points within the boundaries to the right, below, left and above given point
func (f *followingGraph) GetNeighbors(p geometry.Point) []geometry.Point {
	valid := make([]geometry.Point, 0, 4)

	for _, dir := range geometry.GetCardinalDirs() {
		n := p.Add(dir)
		// Only points in bounds
		if f.m.InBounds(n) {
			valid = append(valid, n)
		}
	}

	return valid
}

// CalcHeuristic - implements Manhattan's distance
func (f *followingGraph) CalcHeuristic(p1, p2 geometry.Point) float64 {
	x1, y1 := p1.X, p1.Y
	x2, y2 := p2.X, p2.Y
	return math.Abs(float64(x2-x1)) + math.Abs(float64(y2-y1))
}

//
//
// --- DIJKSTRA PATHFINDER VARIATION ---
//
//

func (p *Pathfinder) DijkstraFind(ctx *model.SessionContext, scentMap [][]int, mover *model.Actor) geometry.Point {
	if finder, exists := p.dijkstraFinders[mover.MovePattern]; exists {
		return finder.Find(ctx, scentMap, mover)
	}
	return mover.Pos
}

type dijkstraCardinal struct{}

func (c dijkstraCardinal) Find(ctx *model.SessionContext, scentMap [][]int, mover *model.Actor) geometry.Point {
	optimalPoints := c.findOptimals(ctx, scentMap, mover)
	if len(optimalPoints) == 0 {
		return mover.Pos
	}
	return optimalPoints[ctx.Rng().Intn(len(optimalPoints))]
}

func (c dijkstraCardinal) findOptimals(ctx *model.SessionContext, scentMap [][]int, mover *model.Actor) []geometry.Point {
	rad := 1
	grid := ctx.Playthrough.Map

	center := mover.Pos
	mins := make([]geometry.Point, 0)
	minScent := scentMap[center.Y][center.X]

	yMin, yMax := max(0, center.Y-rad), min(grid.Height-1, center.Y+rad)
	xMin, xMax := max(0, center.X-rad), min(grid.Width-1, center.X+rad)

	for y := yMin; y <= yMax; y++ {
		p := geometry.Point{Y: y, X: center.X}
		if p == center {
			continue
		}

		if !grid.IsWalkable(p) {
			continue
		}

		if actorId, _ := grid.GetActorID(p); ctx.Playthrough.IsMonster(model.ActorId(actorId)) && mover.Kind != model.ActorPlayer {
			continue
		}

		pScent := scentMap[y][center.X]

		if minScent > pScent {
			mins = mins[:0]
			mins = append(mins, p)
			minScent = pScent
		} else if minScent == pScent {
			mins = append(mins, p)
		}
	}

	for x := xMin; x <= xMax; x++ {
		p := geometry.Point{Y: center.Y, X: x}
		if p == center {
			continue
		}

		if !grid.IsWalkable(p) {
			continue
		}

		if actorId, _ := grid.GetActorID(p); ctx.Playthrough.IsMonster(model.ActorId(actorId)) && mover.Kind != model.ActorPlayer {
			continue
		}

		pScent := scentMap[center.Y][x]

		if minScent > pScent {
			mins = mins[:0]
			mins = append(mins, p)
			minScent = pScent
		} else if minScent == pScent {
			mins = append(mins, p)
		}
	}

	return mins
}

type dijkstraDiagonal struct{}

func (d dijkstraDiagonal) Find(ctx *model.SessionContext, scentMap [][]int, mover *model.Actor) geometry.Point {
	optimalPoints := d.findOptimals(ctx, scentMap, mover)
	if len(optimalPoints) == 0 {
		return mover.Pos
	}
	return optimalPoints[ctx.Rng().Intn(len(optimalPoints))]
}

func (d dijkstraDiagonal) findOptimals(ctx *model.SessionContext, scentMap [][]int, mover *model.Actor) []geometry.Point {
	rad := 1
	grid := ctx.Playthrough.Map

	center := mover.Pos
	mins := make([]geometry.Point, 0)
	minScent := scentMap[center.Y][center.X]
	i := 0

	yMin, yMax := max(center.Y-rad, 0), min(grid.Height-1, center.Y+rad)
	xMin, xMax := max(center.X-rad, 0), min(grid.Width-1, center.X+rad)

	upperAndBottom := make([]geometry.Point, 0, 2)

	for x := xMin; x <= xMax; x++ {
		upperAndBottom = upperAndBottom[:0]

		upper := geometry.Point{Y: yMin + i, X: x}
		bottom := geometry.Point{Y: yMax - i, X: x}

		upperAndBottom = append(upperAndBottom, bottom, upper)

		for _, p := range upperAndBottom {
			if p == center {
				continue
			}

			if !grid.IsWalkable(p) {
				continue
			}

			if actorId, _ := grid.GetActorID(p); ctx.Playthrough.IsMonster(model.ActorId(actorId)) && mover.Kind != model.ActorPlayer {
				continue
			}

			pScent := scentMap[p.Y][p.X]

			if minScent > pScent {
				mins = mins[:0]
				mins = append(mins, p)
				minScent = pScent
			} else if minScent == pScent {
				mins = append(mins, p)
			}
		}

		i++
	}

	return mins
}

type dijkstraMoore struct{}

func (m dijkstraMoore) Find(ctx *model.SessionContext, scentMap [][]int, mover *model.Actor) geometry.Point {
	optimalPoints := m.findOptimals(ctx, scentMap, mover)
	if len(optimalPoints) == 0 {
		return mover.Pos
	}
	return optimalPoints[ctx.Rng().Intn(len(optimalPoints))]
}

func (m dijkstraMoore) findOptimals(ctx *model.SessionContext, scentMap [][]int, mover *model.Actor) []geometry.Point {
	rad := 1
	grid := ctx.Playthrough.Map

	center := mover.Pos
	optimals := make([]geometry.Point, 0)
	minScent := scentMap[center.Y][center.X]

	yMin, yMax := max(0, center.Y-rad), min(grid.Height-1, center.Y+rad)
	xMin, xMax := max(0, center.X-rad), min(grid.Width-1, center.X+rad)

	for y := yMin; y <= yMax; y++ {
		for x := xMin; x <= xMax; x++ {
			p := geometry.Point{Y: y, X: x}
			if p == center {
				continue
			}

			if !grid.IsWalkable(p) {
				continue
			}

			if actorId, _ := grid.GetActorID(p); ctx.Playthrough.IsMonster(model.ActorId(actorId)) && mover.Kind != model.ActorPlayer {
				continue
			}

			pScent := scentMap[y][x]

			if minScent > pScent {
				optimals = optimals[:0]
				optimals = append(optimals, p)
				minScent = pScent
			} else if minScent == pScent {
				optimals = append(optimals, p)
			}
		}
	}

	return optimals
}
