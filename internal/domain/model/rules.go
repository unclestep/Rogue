package model

type GameRules struct {
	Id                      RulesId
	MaxDungeonCount         int // Number of dungeons to complete the game
	DungeonWidth            int
	DungeonHeight           int
	MaxHorizontalRoomCount  int                   // Max number of rooms in horizontal
	MaxVerticalRoomCount    int                   // Max number of rooms in vertical
	ExtraDungeonConnections int                   // Additional random inter-room corridors beyond the spanning tree
	ActorsConf              map[ActorLabel]*Actor // Actors configuration
	ItemsConf               map[ItemLabel]*Item   // Items configuration
	DiffCurve               *DifficultyCurve
	HpRestore               float64 // Percent of max health that will restore player's HP
	TimeForMove             int     // Time for move in seconds
}

type RulesId int

const (
	DefaultRulesId = 0
)

type DifficultyCurve struct {
	Start *DungParams
	End   *DungParams
}

// DungParams encapsulates all variables used by the generator for a specific level.
// It includes difficulty scaling, loot distribution, and door-lock mechanics.
type DungParams struct {
	// Monster Distribution: Total count and relative weights of types
	// Number and difficulty of enemies increases
	MaxMonsters            int                // Max possible number of monsters which can be spawned
	MinMonsters            int                // Min possible number of monsters which can be spawned
	MonsterWeights         map[ActorLabel]int // Probability of each monster type spawn
	MonsterStatsMultiplier float64            // Multiplier for monster HP/Stamina/Strength/Dexterity etc. to increase difficulty

	// Item Distribution: Total count and relative weights of types
	// Amount of useful items decreases.
	MaxItems                int               // Max possible number of items which can be spawned
	MinItems                int               // Min possible number of items which can be spawned
	ItemWeights             map[ItemLabel]int // Probability of each item type spawn
	TreasureValueMultiplier float64           // Value of monsters' loot; increases every level

	// Level Architecture: Controls locked door
	LockedDoorsStartDepth int // Level depth where chance of spawning key-locked doors becomes real
	MaxLockedDoors        int // Max possible number of key-locked doors which can be spawned
	MinLockedDoors        int // Min possible number of key-locked doors which can be spawned
}

func (d *DifficultyCurve) At(curDepth, totalDepth int, dynamicDifficulty float64) *DungParams {
	progress := float64(curDepth-1) / float64(totalDepth-1)

	start := d.Start
	end := d.End
	cur := DungParams{}

	cur.MinMonsters = int(float64(interpolate(start.MinMonsters, end.MinMonsters, progress)) * dynamicDifficulty)
	cur.MaxMonsters = int(float64(interpolate(start.MaxMonsters, end.MaxMonsters, progress)) * dynamicDifficulty)

	cur.MinItems = int(float64(interpolate(start.MinItems, end.MinItems, progress)) / dynamicDifficulty)
	cur.MaxItems = int(float64(interpolate(start.MaxItems, end.MaxItems, progress)) / dynamicDifficulty)

	cur.MonsterStatsMultiplier = interpolate(start.MonsterStatsMultiplier, end.MonsterStatsMultiplier, progress) * dynamicDifficulty
	cur.TreasureValueMultiplier = interpolate(start.TreasureValueMultiplier, end.TreasureValueMultiplier, progress) * dynamicDifficulty

	cur.MonsterWeights = interpolateWeights(start.MonsterWeights, end.MonsterWeights, progress)
	cur.ItemWeights = interpolateWeights(start.ItemWeights, end.ItemWeights, progress)

	cur.LockedDoorsStartDepth = start.LockedDoorsStartDepth
	if curDepth >= start.LockedDoorsStartDepth {
		cur.MinLockedDoors = int(float64(interpolate(start.MinLockedDoors, end.MinLockedDoors, progress)) * dynamicDifficulty)
		cur.MaxLockedDoors = int(float64(interpolate(start.MaxLockedDoors, end.MaxLockedDoors, progress)) * dynamicDifficulty)
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

//
//
// --- DEFAULT CONSTRUCTORS ---
//
//

func NewDefaultGameRules() *GameRules {
	return &GameRules{
		Id:                      0,
		MaxDungeonCount:         21,
		DungeonWidth:            80,
		DungeonHeight:           24,
		MaxHorizontalRoomCount:  4,
		MaxVerticalRoomCount:    4,
		ExtraDungeonConnections: 2,
		ActorsConf:              createDefaultActorsConf(),
		ItemsConf:               createDefaultItemsConf(),
		DiffCurve:               createDefaultDiffCurve(),
		HpRestore:               0.25,
		TimeForMove:             0,
	}
}

func createDefaultActorsConf() map[ActorLabel]*Actor {
	return map[ActorLabel]*Actor{
		ActorLabelPlayerCommon:    NewDefaultPlayer(0, NewInvalidPoint(), 9),
		ActorLabelZombieCommon:    NewDefaultZombie(0, NewInvalidPoint()),
		ActorLabelVampireCommon:   NewDefaultVampire(0, NewInvalidPoint()),
		ActorLabelGhostCommon:     NewDefaultGhost(0, NewInvalidPoint()),
		ActorLabelOgreCommon:      NewDefaultOgre(0, NewInvalidPoint()),
		ActorLabelSnakeMageCommon: NewDefaultSnakeMage(0, NewInvalidPoint()),
		ActorLabelMimicCommon:     NewDefaultMimic(0, NewInvalidPoint()),
		ActorLabelPursuerCommon:   NewDefaultPursuer(0, NewInvalidPoint()),
	}
}

func createDefaultItemsConf() map[ItemLabel]*Item {
	return map[ItemLabel]*Item{
		ItemLabelDefaultFood:     NewDefaultFood(0, NewInvalidPoint()),
		ItemLabelDexterityElixir: NewDefaultDexterityElixir(0, NewInvalidPoint()),
		ItemLabelStrengthElixir:  NewDefaultStrengthElixir(0, NewInvalidPoint()),
		ItemLabelMaxHpElixir:     NewDefaultMaxHpElixir(0, NewInvalidPoint()),
		ItemLabelDexterityScroll: NewDefaultDexterityScroll(0, NewInvalidPoint()),
		ItemLabelStrengthScroll:  NewDefaultStrengthScroll(0, NewInvalidPoint()),
		ItemLabelMaxHpScroll:     NewDefaultMaxHpScroll(0, NewInvalidPoint()),
		ItemLabelDefaultWeapon:   NewDefaultWeapon(0, NewInvalidPoint()),
	}
}

func createDefaultDiffCurve() *DifficultyCurve {
	return &DifficultyCurve{
		Start: createDefaultStartGenParams(),
		End:   createDefaultEndGenParams(),
	}
}

func createDefaultStartGenParams() *DungParams {
	return &DungParams{
		MaxMonsters: 3,
		MinMonsters: 1,
		MonsterWeights: map[ActorLabel]int{
			ActorLabelZombieCommon:    40,
			ActorLabelVampireCommon:   15,
			ActorLabelGhostCommon:     10,
			ActorLabelOgreCommon:      15,
			ActorLabelSnakeMageCommon: 15,
			ActorLabelMimicCommon:     5,
			ActorLabelPursuerCommon:   10,
		},
		MonsterStatsMultiplier: 1.0,
		MaxItems:               7,
		MinItems:               3,
		ItemWeights: map[ItemLabel]int{
			ItemLabelDefaultFood:     40,
			ItemLabelDexterityElixir: 10,
			ItemLabelStrengthElixir:  10,
			ItemLabelMaxHpElixir:     10,
			ItemLabelDexterityScroll: 5,
			ItemLabelStrengthScroll:  5,
			ItemLabelMaxHpScroll:     5,
			ItemLabelDefaultWeapon:   15,
		},
		TreasureValueMultiplier: 1.0,
		LockedDoorsStartDepth:   5,
		MaxLockedDoors:          2,
		MinLockedDoors:          1,
	}
}

func createDefaultEndGenParams() *DungParams {
	return &DungParams{
		MaxMonsters: 15,
		MinMonsters: 12,
		MonsterWeights: map[ActorLabel]int{
			ActorLabelZombieCommon:    10,
			ActorLabelVampireCommon:   25,
			ActorLabelGhostCommon:     15,
			ActorLabelOgreCommon:      25,
			ActorLabelSnakeMageCommon: 20,
			ActorLabelMimicCommon:     5,
			ActorLabelPursuerCommon:   20,
		},
		MonsterStatsMultiplier: 2.0,
		MaxItems:               3,
		MinItems:               0,
		ItemWeights: map[ItemLabel]int{
			ItemLabelDefaultFood:     10,
			ItemLabelDexterityElixir: 20,
			ItemLabelStrengthElixir:  20,
			ItemLabelMaxHpElixir:     20,
			ItemLabelDexterityScroll: 5,
			ItemLabelStrengthScroll:  5,
			ItemLabelMaxHpScroll:     5,
			ItemLabelDefaultWeapon:   15,
		},
		TreasureValueMultiplier: 2.0,
		LockedDoorsStartDepth:   5,
		MaxLockedDoors:          9,
		MinLockedDoors:          7,
	}
}
