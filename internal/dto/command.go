package dto

type Command struct {
	CommandType CommandType
	PlayerUUID  string
	ItemID      int
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

func (c *Command) IsMove() bool {
	return c.CommandType == CommandUp || c.CommandType == CommandRight || c.CommandType == CommandDown || c.CommandType == CommandLeft
}

func (c *Command) IsAction() bool {
	return c.CommandType == CommandConsumeItem || c.CommandType == CommandEquipWeapon || c.CommandType == CommandUnequipWeapon
}
