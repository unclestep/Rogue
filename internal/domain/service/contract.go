package service

import (
	"math/rand"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

//
// -- MAP INTERFACE --
//

type WalkableGrid interface {
	IsWalkable(p geometry.Point) bool
	GetActorID(p geometry.Point) (int64, bool)
	GetDimensions() geometry.Point
}

// -- EVENT INTERFACE --
