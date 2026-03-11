package dto

// Rich DTO
// Stores preprocess data ready to display
// And raw data for more advance processing

type WorldInfo struct {
	Height int
	Width  int
	Grid   [][]*Cell
	State  GameState // Lobby, Playing or GameOver
	Player *Player
	Events []*Event
}

type GameState int

const (
	StateUnknown  GameState = 0
	StateLobby    GameState = 1
	StatePlaying  GameState = 2
	StateGameover GameState = 3
)

type Event struct {
	EventType EventType
	Desc      string
}

type EventType int

const (
	EventUnknown    EventType = 0
	EventAttack     EventType = 1
	EventMove       EventType = 2
	EventItemPickup EventType = 3
	EventItemUsage  EventType = 4
)

type Cell struct {
	VisibilityState VisibilityState // Unexplored, visible, explored
	TopologyType    TopologyType    // Empty, floor, wall etc.
	Actor           *Actor          // Must be nil if cell is hidden
	Item            *Item           // Must be nil if cell is hidden
}

type VisibilityState int

const (
	VisibilityUnknown    VisibilityState = 0
	VisibilityUnexplored VisibilityState = 1
	VisibilityVisible    VisibilityState = 2
	VisibilityExplored   VisibilityState = 3
)

type TopologyType int

const (
	TopologyUnknown    TopologyType = 0
	TopologyEmpty      TopologyType = 1
	TopologyWall       TopologyType = 2
	TopologyFloor      TopologyType = 3
	TopologyOpenDoor   TopologyType = 4
	TopologyClosedDoor TopologyType = 5
	TopologyCorridor   TopologyType = 6
	TopologyExit       TopologyType = 7
)

type Actor struct {
	Id   int
	Kind ActorKind
	// Raw data
	BaseVitals   map[VitalType]int   // Base vitals
	BaseAttrs    map[AttrType]int    // Base attributes
	VitalsChange map[VitalType][]int // Effects influence on stats (like: hp: 100 (base) + 2 + 3 + 50)
	AttrsChange  map[AttrType][]int  // Effects influence on attributes (like: strength: 50 (base) + 5 + 10 + 2)
	// Processed data
	Vitals   map[string][]string
	Attrs    map[string][]string
	Statuses []string
	// Detailed info about applied effects
	AppliedEffects []*Effect
	// Providing only in string format
	// TODO: TraitsDesc []string
}

type ActorKind string

type VitalType int

const (
	VitalUnknown VitalType = 0
	VitalHP      VitalType = 1
	VitalStamina VitalType = 2
)

type AttrType int

const (
	AttrUnknown AttrType = iota
	MaxHealth
	MaxStamina
	AttackStaminaCost
	MoveStaminaCost
	ActionStaminaCost // e.g. item usage, weapon equip and unequip
	StaminaRegen
	Strength
	Dexterity
	Hostility
	CounterAttackChance
)

type Effect struct {
	Kind     EffectKind
	Duration int
	Charges  int
	// Raw data
	VitalsChange map[VitalType]int
	AttrsChange  map[AttrType]int
	// Processed into strings data
	Buffs    []string
	Debuffs  []string
	Statuses []string
	// Providing only in string format
	// TODO: ProcsDesc []string
}

type EffectKind string

type Player struct {
	Actor     *Actor
	HUD       *HUD
	RunStats  *RunStats
	Inventory map[ItemType][]*Item // Stores item type as key and slice of IDs
	Equipped  map[ItemType]*Item
}

type Item struct {
	Id           int
	Kind         ItemType
	Label        ItemLabel
	Keyhole      Keyhole
	VitalsChange map[VitalType]int
	AttrsChange  map[AttrType]int
	Effects      []string
	// TODO: ProcsDesc    []string
}

type ItemType string
type ItemLabel string

type Keyhole int

const (
	KeyholeNone Keyhole = 0 // Item is not a key
)

type HUD struct {
	HP        int
	MaxHP     int
	Strength  int
	Dexterity int
	Level     int // Dungeon depth
	Treasure  int
}

type RunStats struct {
	TotalTreasure    int `json:"total_treasure"`
	DeepestLevel     int `json:"deepest_level"`
	MonstersDefeated int `json:"enemies_defeated"`
	FoodConsumed     int `json:"food_consumed"`
	ElixirsDrunk     int `json:"elixirs_drunk"`
	ScrollsRead      int `json:"scrolls_read"`
	HitsDealt        int `json:"hits_dealt"`
	HitsReceived     int `json:"hits_received"`
	TilesTraveled    int `json:"tiles_traveled"`
}
