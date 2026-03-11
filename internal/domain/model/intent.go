package model

import (
	"github.com/unclestep/Rogue/pkg/geometry"
)

type Intent interface{}

type AttackIntent struct {
	Attacker ActorId
	Defender ActorId
}

type MoveIntent struct {
	Mover  ActorId
	NewPos geometry.Point
}
