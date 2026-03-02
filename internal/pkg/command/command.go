package command

import (
	"github.com/unclestep/Rogue/internal/domain/entity"
)

type Command struct {
	CommandType CommandType
	PlayerID    entity.ActorId
	ItemID      entity.ItemId
	PlayerStats *entity.GameStats
}

type CommandType int

const (
	NoCommand CommandType = iota
	CommandJoin
	CommandAgree
	CommandDisagree
	CommandUp
	CommandRight
	CommandDown
	CommandLeft
	CommandConsumeItem
	CommandEquipWeapon
	CommandUnequipWeapon
)
