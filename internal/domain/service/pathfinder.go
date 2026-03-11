package service

import (
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/algorithm"
	"github.com/unclestep/Rogue/pkg/geometry"
	"math"
)

type Pathfinder struct{}

// FindPath - finds the path between two points in map's topology.
// Algorithm can only pass through corridors, floors and open doors.
// It tries to avoid the cells with items and actors.
// Returns slice of points between two given points including them and bool value, if it could find the way.
func (p *Pathfinder) Find(m *model.Map, p1, p2 geometry.Point) ([]geometry.Point, bool) {
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
