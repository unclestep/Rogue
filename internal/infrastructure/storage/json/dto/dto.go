package dto

type Playthrough struct {
	PlaythroughId       int64                      `json:"playthrough_id"`
	HostId              int64                      `json:"host_id"`
	RulesId             int64                      `json:"rules_id"`
	Map                 *MapDTO                    `json:"map"`
	PlayersUuid         map[string]int64           `json:"players_uuid"`
	Players             map[int64]*ActorDTO        `json:"players"`
	PlayersStats        map[int64]*GameStatsDTO    `json:"players_stats"`
	PlayersLevelMetrics map[int64]*LevelMetricsDTO `json:"level_metrics"`
	PlayersFoW          map[int64]*VisibleAreaDTO  `json:"players_fow"`
	DeadPlayers         map[int64]*ActorDTO
	Monsters            map[int64]*ActorDTO `json:"monsters"`
	DeadMonsters        map[int64]*ActorDTO
	Items               map[int64]*ItemDTO `json:"items"`
	Depth               int                `json:"depth"`
	State               string             `json:"state"`
	DungParams          *DungParamsDTO     `json:"dung_params"`
	DynamicDifficulty   float64            `json:"dynamic_difficulty"`
	NextId              int64              `json:"next_id"`
	Seed                int64              `json:"seed"`
}

type MapDTO struct {
	Width          int                `json:"width"`            // Map width
	Height         int                `json:"height"`           // Map height
	TileGrid       [][]CellDTO        `json:"tile_grid"`        // Layer 1: Game landscape
	ItemGrid       [][]int64          `json:"item_grid"`        // Layer 2: Location of items
	ActorGrid      [][]int64          `json:"actor_grid"`       // Layer 3: Location of actors
	Rooms          []*RoomDTO         `json:"rooms"`            // Pointers to all level rooms
	Doors          []*DoorMetadataDTO `json:"doors"`            // All doors on the map
	EntranceRoomId int64              `json:"entrance_room_id"` // Pointer to room with spawn point
	ExitRoomId     int64              `json:"exit_room_id"`     // Pointer to room with exit point
	ExitPoint      PointDTO           `json:"exit_point"`       // End of level point
}

type CellDTO struct {
	Type   string `json:"type"`
	RoomId int64  `json:"room_id"`
}

type RoomDTO struct {
	Id     int64    `json:"id"`
	Pos    PointDTO `json:"pos"`    // Left-upper walkable corner
	Width  int      `json:"width"`  // Room width
	Height int      `json:"height"` // Room height
	Center PointDTO `json:"center"` // Room center
}

type DoorMetadataDTO struct {
	Pos     PointDTO `json:"pos"`     // Door position
	Locked  bool     `json:"locked"`  // Lock state
	Keyhole int      `json:"keyhole"` // Key color which opens this door
	KeyPos  PointDTO `json:"key_pos"` // Position of key which opens this door
}

type PlaythroughId int

type GameStatsDTO struct {
	// Main statistics
	TotalTreasure int `json:"total_treasure"`
	DeepestLevel  int `json:"deepest_level"`
	// Additional statistics
	MonstersDefeated int `json:"enemies_defeated"`
	FoodConsumed     int `json:"food_consumed"`
	ElixirsDrunk     int `json:"elixirs_drunk"`
	ScrollsRead      int `json:"scrolls_read"`
	HitsDealt        int `json:"hits_dealt"`
	HitsReceived     int `json:"hits_received"`
	TilesTraveled    int `json:"tiles_traveled"`
}

type LevelMetricsDTO struct {
	DamageTaken int `json:"damage_taken"`
	DamageDealt int `json:"damage_dealt"`
}

type VisibleAreaDTO struct {
	Area         [][]string
	VisibleCells []PointDTO // Swift access to visible cells
}

type ActorDTO struct {
	Id           int64                     `json:"id"`   // Unique identification of actor
	Kind         string                    `json:"kind"` // Actor type, e.g. Player, Vampire, Zombie
	Label        string                    `json:"label"`
	MovePattern  string                    `json:"move_pattern"`
	MoveDir      PointDTO                  `json:"move_dir"` // Move direction; can be changed by moving or turning view direction
	Pos          PointDTO                  `json:"pos"`
	Vitals       map[string]int            `json:"vitals"`        // Non-constant stats, e.g. current hp, stamina
	BaseAttrs    map[string]int            `json:"base_attrs"`    // Base actor's stats: strength, max health etc.
	DerivedAttrs map[string]int            `json:"derived_attrs"` // Computed actor's stats: base attributes + effects + weapon
	Statuses     map[string]int            `json:"statuses"`      // Computed actor's statuses: statuses are granted by effects
	Effects      map[string]*EffectDTO     `json:"effects"`       // Actor effects: permanent (reversible via dispelling) or temporary (expiring by turn count or usage limit). Effects of the same type stack by extending the duration; however, they do not increase or decrease the modified stats further
	EquippedGear map[string]*ItemDTO       `json:"equipped_gear"`
	Backpack     *BackpackDTO              `json:"backpack"`
	Traits       map[string][]*ReactionDTO `json:"traits"` // What actor does in different situations
	State        string                    `json:"state"`
}

type ItemDTO struct {
	Id              int64                     `json:"id"`
	Kind            string                    `json:"kind"`
	Label           string                    `json:"label"`
	Pos             PointDTO                  `json:"pos"`
	Keyhole         int                       `json:"keyhole"`
	Value           int                       `json:"value"`
	VitalsChange    map[string]int            `json:"vitals_change"`
	BaseAttrsChange map[string]int            `json:"base_attrs_change"`
	Effects         map[string]*EffectDTO     `json:"effects"` // Ongoing stat modifiers
	Procs           map[string][]*ReactionDTO `json:"procs"`   // Event-driven triggers
}

type BackpackDTO struct {
	Slots          map[string][]*ItemDTO `json:"slots"`
	TreasuresValue int                   `json:"treasures_values"`
	SlotsCapacity  int                   `json:"slots_capacity"`
}

type EffectDTO struct {
	Kind     string `json:"kind"`
	Duration int    `json:"duration"`
	Charges  int    `json:"charges"`

	// Passive bonuses
	VitalsChange   map[string]int `json:"vitals_change"`   // Temporary HP or stamina boost
	AttrsChange    map[string]int `json:"attrs_change"`    // Affects on the computation of derived attributes: it never modifies base attributes
	StatusesChange map[string]int `json:"statuses_change"` // Can inflict status conditions such as Sleep, Stun, etc.

	// Active bonuses

	Procs     map[string][]*ReactionDTO `json:"procs"`      // Triggers the special ability logic
	ConsumeOn string                    `json:"consume_on"` // Situations when we need to decrement the charges
}

type ReactionDTO struct {
	Trigger         string                `json:"trigger"`
	Target          string                `json:"target"`
	VitalsChange    map[string]ChangeDTO  `json:"vitals_change"`
	BaseAttrsChange map[string]ChangeDTO  `json:"base_attrs_change"`
	StatusesChange  map[string]int        `json:"statuses_change"`
	EffectsToApply  map[string]*EffectDTO `json:"effects_to_apply"`
	Chance          int                   `json:"chance"`
}

type ChangeDTO struct {
	Holder string  `json:"holder"`
	Vital  string  `json:"vital"`
	Attr   string  `json:"attr"`
	Amount int     `json:"amount"`
	Scale  float64 `json:"scale"`
}

type PointDTO struct {
	X int `json:"X"`
	Y int `json:"Y"`
}

type DifficultyCurveDTO struct {
	Start *DungParamsDTO `json:"start_gen_params"`
	End   *DungParamsDTO `json:"end_gen_params"`
}

type GameRulesDTO struct {
	Id                     int64
	MaxDungeonCount        int                  `json:"max_dungeon_count"` // Number of dungeons to complete the game
	DungeonWidth           int                  `json:"dungeon_width"`
	DungeonHeight          int                  `json:"dungeon_height"`
	MaxHorizontalRoomCount int                  `json:"max_horizontal_room_count"` // Max number of rooms in horizontal
	MaxVerticalRoomCount   int                  `json:"max_vertical_room_count"`   // Max number of rooms in vertical
	ActorsConf             map[string]*ActorDTO `json:"actors_conf"`               // Actors configuration
	ItemsConf              map[string]*ItemDTO  `json:"items_conf"`                // Items configuration
	DiffCurve              *DifficultyCurveDTO  `json:"diff_curve"`
	HpRestore              float64              `json:"hp_restore"`    // Percent of max health that will restore player's HP
	TimeForMove            int                  `json:"time_for_move"` // Time for move in seconds
}

// DungParamsDTO encapsulates all variables used by the generator for a specific level.
// It includes difficulty scaling, loot distribution, and door-lock mechanics.
type DungParamsDTO struct {
	// Monster Distribution: Total count and relative weights of types
	// Number and difficulty of enemies increases
	MaxMonsters            int            `json:"max_monsters"`             // Max possible number of monsters which can be spawned
	MinMonsters            int            `json:"min_monsters"`             // Min possible number of monsters which can be spawned
	MonsterWeights         map[string]int `json:"monster_weights"`          // Probability of each monster type spawn
	MonsterStatsMultiplier float64        `json:"monster_stats_multiplier"` // Multiplier for monster HP/Stamina/Strength/Dexterity etc. to increase difficulty

	// Item Distribution: Total count and relative weights of types
	// Amount of useful items decreases.
	MaxItems                int            `json:"max_items"`                 // Max possible number of items which can be spawned
	MinItems                int            `json:"min_items"`                 // Min possible number of items which can be spawned
	ItemWeights             map[string]int `json:"item_weights"`              // Probability of each item type spawn
	TreasureValueMultiplier float64        `json:"treasure_value_multiplier"` // Value of monsters' loot; increases every level

	// Level Architecture: Controls locked door
	LockedDoorsStartDepth int `json:"locked_door_start_depth"` // Level depth where chance of spawning key-locked doors becomes real
	MaxLockedDoors        int `json:"max_locked_doors"`        // Max possible number of key-locked doors which can be spawned
	MinLockedDoors        int `json:"min_locked_doors"`        // Min possible number of key-locked doors which can be spawned
}
