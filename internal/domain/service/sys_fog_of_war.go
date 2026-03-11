package service

import (
	"github.com/unclestep/Rogue/pkg/geometry"
	"github.com/unclestep/Rogue/pkg/utils"
)

type VisibleArea struct {
	Area         [][]VisibilityState
	VisibleCells []geometry.Point // Swift access to visible cells
	Grid         WalkableGrid
}

// VisibilityState - enumeration of cell visibility states
type VisibilityState int

// Visibility states
const (
	Unexplored VisibilityState = iota
	Explored
	Visible
)

func NewVisibleArea(grid WalkableGrid) *VisibleArea {
	cols, rows := grid.GetDimensions().X, grid.GetDimensions().Y

	area := &VisibleArea{
		Area:         utils.CreateMatrix[VisibilityState](rows, cols),
		VisibleCells: make([]geometry.Point, 0, 32),
		Grid:         grid,
	}

	return area
}

func (a *VisibleArea) Update(pos geometry.Point, visibleRange int) bool {
	_ = pos
	_ = visibleRange

	for row := range a.Area {
		for col := range a.Area[row] {
			a.Area[row][col] = Visible
		}
	}

	return true
}
