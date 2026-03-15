package dto

// Rich DTO
// Stores preprocess data ready to display
// And raw data for more advance processing

type GameView struct {
	Height int       `json:"height"`
	Width  int       `json:"width"`
	Grid   [][]*Cell `json:"grid"`
	State  GameState `json:"state"`
	Player *Player   `json:"player"`
	Events []*Event  `json:"events"`
}

type GameState int

const (
	StateUnknown  GameState = 0
	StateLobby    GameState = 1
	StatePlaying  GameState = 2
	StateGameover GameState = 3
)

type Event struct {
	EventType EventType `json:"type"`
	Desc      string    `json:"desc"`
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
	VisibilityState VisibilityState `json:"visibility_state"` // Unexplored, visible, explored
	TopologyType    TopologyType    `json:"topology_type"`    // Empty, floor, wall etc.
	Actor           *Actor          `json:"actor"`            // Must be nil if cell is hidden
	Item            *Item           `json:"item"`             // Must be nil if cell is hidden
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
	Kind ActorKind `json:"kind"`
	// Raw data
	BaseVitals   map[VitalType]int   `json:"base_vitals"`   // Base vitals
	BaseAttrs    map[AttrType]int    `json:"base_attrs"`    // Base attributes
	VitalsChange map[VitalType][]int `json:"vitals_change"` // Effects influence on stats (like: hp: 100 (base) + 2 + 3 + 50)
	AttrsChange  map[AttrType][]int  `json:"attrs_change"`  // Effects influence on attributes (like: strength: 50 (base) + 5 + 10 + 2)
	// Processed data
	Vitals   map[string][]string `json:"vitals_stringed"`
	Attrs    map[string][]string `json:"attrs_stringed"`
	Statuses []string            `json:"statuses"`
	// Detailed info about applied effects
	AppliedEffects []*Effect `json:"applied_effects"`
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
	Kind     EffectKind `json:"kind"`
	Duration int        `json:"duration"`
	Charges  int        `json:"charges"`
	// Raw data
	VitalsChange map[VitalType]int `json:"vitals_change"`
	AttrsChange  map[AttrType]int  `json:"attrs_change"`
	// Processed into strings data
	Buffs    []string `json:"buffs"`
	Debuffs  []string `json:"debuffs"`
	Statuses []string `json:"statuses"`
	// Providing only in string format
	// TODO: ProcsDesc []string
}

type EffectKind string

type Player struct {
	Actor     *Actor               `json:"actor"`
	HUD       *HUD                 `json:"hud"`
	RunStats  *RunStats            `json:"run_stats"`
	Inventory map[ItemType][]*Item `json:"inventory"` // Stores item type as key and slice of IDs
	Equipped  map[ItemType]*Item   `json:"equipped"`
}

type Item struct {
	Id           int               `json:"id"`
	Kind         ItemType          `json:"kind"`
	Label        ItemLabel         `json:"label"`
	Keyhole      Keyhole           `json:"keyhole,omitempty"`
	VitalsChange map[VitalType]int `json:"vitals_change,omitempty"`
	AttrsChange  map[AttrType]int  `json:"attrs_change,omitempty"`
	Effects      []string          `json:"effects,omitempty"`
	// TODO: ProcsDesc    []string
}

type ItemType string
type ItemLabel string

type Keyhole int

const (
	KeyholeNone Keyhole = 0 // Item is not a key
)

type HUD struct {
	HP        int `json:"hp"`
	MaxHP     int `json:"max_hp"`
	Strength  int `json:"strength"`
	Dexterity int `json:"dexterity"`
	Dungeon   int `json:"dungeon"`
	Treasure  int `json:"treasure"`
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
