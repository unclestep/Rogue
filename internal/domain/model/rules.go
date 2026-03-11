package model

type GameRules struct {
	MaxDungeonCount        int                   `json:"max_dungeon_count"`         // Number of dungeons to complete the game
	MaxHorizontalRoomCount int                   `json:"max_horizontal_room_count"` // Max number of rooms in horizontal
	MaxVerticalRoomCount   int                   `json:"max_vertical_room_count"`   // Max number of rooms in vertical
	ActorsConf             map[ActorLabel]*Actor `json:"actors_conf"`               // Actors configuration
	ItemsConf              map[ItemLabel]*Item   `json:"items_conf"`                // Items configuration
	GameDifficulty         float64               `json:"game_difficulty"`           // Chosen game difficulty (stable, not adjustable)
	HpRestore              float64               `json:"hp_restore"`                // Percent of max health that will restore player's HP
	TimeForMove            int                   `json:"time_for_move"`             // Time for move in seconds
}

// DungGenParams encapsulates all variables used by the generator for a specific level.
// It includes difficulty scaling, loot distribution, and door-lock mechanics.
type DungGenParams struct {
	// Monster Distribution: Total count and relative weights of types
	// Number and difficulty of enemies increases
	MaxMonsters            int                `json:"max_monsters"`             // Max possible number of monsters which can be spawned
	MinMonsters            int                `json:"min_monsters"`             // Min possible number of monsters which can be spawned
	MonsterWeights         map[ActorLabel]int `json:"monster_weights"`          // Probability of each monster type spawn
	MonsterStatsMultiplier float64            `json:"monster_stats_multiplier"` // Multiplier for monster HP/Stamina/Strength/Dexterity etc. to increase difficulty

	// Item Distribution: Total count and relative weights of types
	// Amount of useful items decreases.
	MaxItems                int               `json:"max_items"`                 // Max possible number of items which can be spawned
	MinItems                int               `json:"min_items"`                 // Min possible number of items which can be spawned
	ItemWeights             map[ItemLabel]int `json:"item_weights"`              // Probability of each item type spawn
	TreasureValueMultiplier float64           `json:"treasure_value_multiplier"` // Value of monsters' loot; increases every level

	// Level Architecture: Controls locked door
	LockedDoorsStartDepth int `json:"locked_door_start_depth"` // Level depth where chance of spawning key-locked doors becomes real
	MaxLockedDoors        int `json:"max_locked_doors"`        // Max possible number of key-locked doors which can be spawned
	MinLockedDoors        int `json:"min_locked_doors"`        // Min possible number of key-locked doors which can be spawned
}

type DifficultyCurve struct {
	Start *DungGenParams `json:"start_gen_params"`
	End   *DungGenParams `json:"end_gen_params"`
}

func (d *DifficultyCurve) At(curDepth, totalDepth int, initDifficulty, dynamicDifficulty float64) *DungGenParams {
	progress := float64(curDepth-1) / float64(totalDepth-1)

	start := d.Start
	end := d.End
	cur := DungGenParams{}

	cur.MinMonsters = int(float64(interpolate(start.MinMonsters, end.MinMonsters, progress)) * initDifficulty * dynamicDifficulty)
	cur.MaxMonsters = int(float64(interpolate(start.MaxMonsters, end.MaxMonsters, progress)) * initDifficulty * dynamicDifficulty)

	cur.MinItems = int(float64(interpolate(start.MinItems, end.MinItems, progress)) / (initDifficulty * dynamicDifficulty))
	cur.MaxItems = int(float64(interpolate(start.MaxItems, end.MaxItems, progress)) / (initDifficulty * dynamicDifficulty))

	cur.MonsterStatsMultiplier = interpolate(start.MonsterStatsMultiplier, end.MonsterStatsMultiplier, progress) * initDifficulty * dynamicDifficulty
	cur.TreasureValueMultiplier = interpolate(start.TreasureValueMultiplier, end.TreasureValueMultiplier, progress) * initDifficulty * dynamicDifficulty

	cur.MonsterWeights = interpolateWeights(start.MonsterWeights, end.MonsterWeights, progress)
	cur.ItemWeights = interpolateWeights(start.ItemWeights, end.ItemWeights, progress)

	if curDepth >= start.LockedDoorsStartDepth {
		cur.MinLockedDoors = int(float64(interpolate(start.MinLockedDoors, end.MinLockedDoors, progress)) * initDifficulty * dynamicDifficulty)
		cur.MaxLockedDoors = int(float64(interpolate(start.MaxLockedDoors, end.MaxLockedDoors, progress)) * initDifficulty * dynamicDifficulty)
	}

	return &cur
}

func interpolate[T interface{ ~int | float64 }](start, end T, step float64) T {
	return T(float64(start) + (float64(end-start) * step))
}

func interpolateWeights[T comparable](start, end map[T]int, step float64) map[T]int {
	res := make(map[T]int)

	for key := range start {
		res[key] = interpolate(start[key], end[key], step)
	}
	for key := range end {
		if _, exists := res[key]; !exists {
			res[key] = interpolate(start[key], end[key], step)
		}
	}

	return res
}
