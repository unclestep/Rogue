package dto

type Command struct {
	PlaythroughId     string
	PlaythroughParams *PlaythroughParams
	PlayerUUID        string
	PlayerNickname    string // display name; stored server-side on first join
	Action            ActionType
	ItemID            int64
	MoveVector        Vector
	AimAngle          float64 // radians; used only with ActionAim
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
	ActionWait // Player skips their turn; counts as a submitted intent so the turn can resolve.
	ActionAim  // Player moved the cursor; updates flashlight direction without consuming a turn.
)
