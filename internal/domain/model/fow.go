package model

import (
	"github.com/unclestep/Rogue/pkg/geometry"
	"github.com/unclestep/Rogue/pkg/utils"
)

type VisibleArea struct {
	Area         [][]VisibilityState
	VisibleCells []geometry.Point // Swift access to visible cells
}

// VisibilityState - enumeration of cell visibility states
type VisibilityState int

// Visibility states
const (
	Unexplored VisibilityState = iota
	Explored
	Visible
)

func NewVisibleArea(height, width int) *VisibleArea {
	return &VisibleArea{
		Area:         utils.CreateMatrix[VisibilityState](height, width),
		VisibleCells: make([]geometry.Point, 0, 32),
	}
}
