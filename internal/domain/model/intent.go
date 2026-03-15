package model

import (
	"github.com/unclestep/Rogue/pkg/geometry"
)

type Intent struct {
	IntentType IntentType
	Actor      ActorId
	Defender   ActorId
	ItemId     ItemId
	Vector     geometry.Point
}

type IntentType int

const (
	IntentUnknown IntentType = iota
	IntentAttack
	IntentMove
	IntentInteract
	IntentConsume
	IntentEquip
	IntentUnequip
)
