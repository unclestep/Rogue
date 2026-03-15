package dto

type Command struct {
	PlaythroughId     int64
	PlaythroughParams *PlaythroughParams
	PlayerUUID        string
	Action            ActionType
	ItemID            int64
	MoveVector        Vector
}

type PlaythroughParams struct {
	RulesId int64
	Seed    int64
}

type Vector struct {
	X, Y int
}

type ActionType int

const (
	NoAction ActionType = iota
	ActionJoin
	ActionLeave
	ActionYes
	ActionNo
	ActionMove
	ActionTick
	ActionConsumeItem
	ActionEquipWeapon
	ActionUnequipWeapon
)
