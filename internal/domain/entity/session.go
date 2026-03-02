package entity

import (
	"cmp"
	"log"
	"math/rand"
	"slices"

	"github.com/unclestep/Rogue/pkg/conv"
	"github.com/unclestep/Rogue/pkg/geometry"
	"github.com/unclestep/Rogue/pkg/idgen"
)

const (
	PlaceForLootSearchRad = 1
)

type GameSession struct {
	HostId              ActorId                   `json:"host_id"`
	Map                 *Map                      `json:"map"`
	Players             map[ActorId]*Actor        `json:"players"`
	PlayersStats        map[ActorId]*GameStats    `json:"players_stats"`
	PlayersLevelMetrics map[ActorId]*LevelMetrics `json:"level_metrics"`
	Monsters            map[ActorId]*Actor        `json:"monsters"`
	Items               map[ItemId]*Item          `json:"items"`
	CurDepth            int                       `json:"cur_depth"`
	CurState            GameState                 `json:"cur_state"`
	DungeonGenManager   *DungeonGenManager        `json:"dungeon_gen_manager"`
	CurDungeonGenParams *DungeonGenParams         `json:"cur_dungeon_gen_params"`
	GameRules           *GameRules                `json:"game_rules"`
}

type GameStats struct {
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

type GameState int

const (
	LobbyGameState GameState = iota
	PlayingGameState
	GameOverGameState
)

// DungeonGenParams encapsulates all variables used by the generator for a specific level.
// It includes difficulty scaling, loot distribution, and door-lock mechanics.
type DungeonGenParams struct {
	// Monster Distribution: Total count and relative weights of types
	// Number and difficulty of enemies increases
	MaxMonsters            int               `json:"max_monsters"`             // Max possible number of monsters which can be spawned
	MinMonsters            int               `json:"min_monsters"`             // Min possible number of monsters which can be spawned
	MonsterWeigths         map[ActorType]int `json:"monster_weights"`          // Probability of each monster type spawn
	MonsterStatsMultiplier float64           `json:"monster_stats_multiplier"` // Multiplier for moster HP/Stamina/Strength/Dexterity etc. to increase difficulty

	// Item Distribution: Total count and relative weights of types
	// Amount of useful items decreases.
	MaxItems                int              `json:"max_items"`                 // Max possible number of items which can be spawned
	MinItems                int              `json:"min_items"`                 // Min possible number of items which can be spawned
	ItemWeights             map[ItemType]int `json:"item_weights"`              // Probability of each item type spawn
	TreasureValueMultiplier float64          `json:"treasure_value_multiplier"` // Value of monsters' loot; increases every level

	// Level Architecture: Controls locked door
	LockedDoorsStartDepth int `json:"locked_door_start_depth"` // Level depth where chance of spawning key-locked doors becomes real
	MaxLockedDoors        int `json:"max_locked_doors"`        // Max possible number of key-locked doors which can be spawned
	MinLockedDoors        int `json:"min_locked_doors"`        // Min possible number of key-locked doors which can be spawned

	// Dynamic difficulty
	DiffucltyAdjustment float64 // Multiplier changes game balance: if players are struggling difficulty will decrease and vice versa
}

type DungeonGenManager struct {
	StartGenParams *DungeonGenParams `json:"start_gen_params"`
	EndGenParams   *DungeonGenParams `json:"end_gen_params"`
}

func (d *DungeonGenManager) GetParamsForLevel(session *GameSession) *DungeonGenParams {
	maxDungeonCount := session.GameRules.MaxDungeonCount
	depth := session.CurDepth
	chosenDifficulty := session.GameRules.GameDifficulty
	dynamicDifficulty := session.CurDungeonGenParams.DiffucltyAdjustment
	if dynamicDifficulty == 0 {
		dynamicDifficulty = d.StartGenParams.DiffucltyAdjustment
	}

	progress := float64(depth-1) / float64(maxDungeonCount-1)

	start := d.StartGenParams
	end := d.EndGenParams
	cur := DungeonGenParams{}

	cur.MinMonsters = int(float64(interpolate(start.MinMonsters, end.MinMonsters, progress)) * chosenDifficulty * dynamicDifficulty)
	cur.MaxMonsters = int(float64(interpolate(start.MaxMonsters, end.MaxMonsters, progress)) * chosenDifficulty * dynamicDifficulty)

	cur.MinItems = int(float64(interpolate(start.MinItems, end.MinItems, progress)) / (chosenDifficulty * dynamicDifficulty))
	cur.MaxItems = int(float64(interpolate(start.MaxItems, end.MaxItems, progress)) / (chosenDifficulty * dynamicDifficulty))

	cur.MonsterStatsMultiplier = float64(interpolate(start.MonsterStatsMultiplier, end.MonsterStatsMultiplier, progress)) * chosenDifficulty * dynamicDifficulty
	cur.TreasureValueMultiplier = float64(interpolate(start.TreasureValueMultiplier, end.TreasureValueMultiplier, progress)) * chosenDifficulty * dynamicDifficulty

	cur.MonsterWeigths = interpolateWeigths(start.MonsterWeigths, end.MonsterWeigths, progress)
	cur.ItemWeights = interpolateWeigths(start.ItemWeights, end.ItemWeights, progress)

	if depth >= start.LockedDoorsStartDepth {
		cur.MinLockedDoors = int(float64(interpolate(start.MinLockedDoors, end.MaxLockedDoors, progress)) * chosenDifficulty * dynamicDifficulty)
		cur.MaxLockedDoors = int(float64(interpolate(start.MaxLockedDoors, end.MaxLockedDoors, progress)) * chosenDifficulty * dynamicDifficulty)
	}

	return &cur
}

func interpolate[T interface{ ~int | float64 }](start, end T, step float64) T {
	return T(float64(start) + (float64(end-start) * step))
}

func interpolateWeigths[T comparable](start, end map[T]int, step float64) map[T]int {
	res := make(map[T]int)

	for key := range start {
		res[key] = interpolate(start[key], end[key], step)
	}

	return res
}

type GameRules struct {
	PlayerCount            int                  `json:"player_count"`              // Number of players
	MaxDungeonCount        int                  `json:"max_dungeon_count"`         // Number of dungeons to complete the game
	MaxHorizontalRoomCount int                  `json:"max_horizontal_room_count"` // Max number of rooms in horizontal
	MaxVerticalRoomCount   int                  `json:"max_vertical_room_count"`   // Max number of rooms in vertical
	ActorsConf             map[ActorType]*Actor `json:"actors_conf"`               // Actors configuration
	ItemsConf              map[ItemType]*Item   `json:"items_conf"`                // Items configuration
	GameDifficulty         float64              `json:"game_difficulty"`           // Chosen game difficulty (stable, not adjustable)
	HPRestore              float64              `json:"hp_restore"`                // Procent of max health that will restore player's HP
}

type LevelMetrics struct {
	DamageTaken int `json:"damage_taken"`
	DamageDealt int `json:"damage_dealt"`
}

func (lm *LevelMetrics) Clear() {
	lm.DamageTaken = 0
	lm.DamageDealt = 0
}

//
//
// --- CONSTRUCTORS ---
//
//

func NewGameSession(gameRules *GameRules, dungGenManager *DungeonGenManager) *GameSession {
	gs := &GameSession{
		Map:                 NewDefaultMap(),
		Players:             make(map[ActorId]*Actor),
		PlayersStats:        make(map[ActorId]*GameStats),
		PlayersLevelMetrics: make(map[ActorId]*LevelMetrics),
		Monsters:            make(map[ActorId]*Actor),
		Items:               make(map[ItemId]*Item),
		CurDepth:            0,
		DungeonGenManager:   dungGenManager,
		GameRules:           gameRules,
	}
	return gs
}

//
//
// --- DUNGEON CREATION ---
//
//

func (gs *GameSession) NextLevel(rng *rand.Rand) {
	gs.CurDepth++
	gs.CurDungeonGenParams = gs.DungeonGenManager.GetParamsForLevel(gs)
	gs.CreateDungeon(rng)
	gs.AdaptDifficulty()
	gs.HealPlayers()
	gs.ClearMetrics()
}

func (gs *GameSession) CreateDungeon(rng *rand.Rand) {
	m := gs.Map

	// Clear previous level
	m.ClearLevel()
	idgen.SetStartID(0)

	// Generate new one
	m.GenerateTopology(gs.GameRules.MaxHorizontalRoomCount, gs.GameRules.MaxVerticalRoomCount, rng)
	gs.CreateKeys(rng)
	gs.CreateItems(rng)
	gs.ShowPlayers(rng)
	gs.CreateMonsters(rng)
}

func (gs *GameSession) CreateKeys(rng *rand.Rand) {
	genParams := gs.CurDungeonGenParams
	doorCount := genParams.MinLockedDoors + rng.Intn(genParams.MaxLockedDoors-genParams.MinLockedDoors)
	keys := gs.Map.GenerateKeysAndDoors(doorCount, doorCount, rng)

	for id, pos := range keys {
		item := NewKeyItem(ItemId(id), pos)
		gs.AddItem(item)
	}
}

func (gs *GameSession) CreateItems(rng *rand.Rand) {
	genParams := gs.CurDungeonGenParams
	itemCount := genParams.MinItems + rng.Intn(genParams.MaxItems-genParams.MinItems)
	items := gs.Map.GenerateItems(itemCount, rng)
	itemWeightsKV := conv.MapToKV(genParams.ItemWeights)
	slices.SortFunc(itemWeightsKV, func(a, b conv.KV[ItemType, int]) int {
		return cmp.Compare(b.Val, a.Val)
	})

	for id, pos := range items {
		itemType := getWeightedResult(itemWeightsKV, rng)
		item := gs.GameRules.ItemsConf[itemType].Clone()
		item.Id = ItemId(id)
		item.Pos = pos
		gs.AddItem(item)

	}
}

func (gs *GameSession) AddPlayer(playerStats *GameStats) {
	player := gs.GameRules.ActorsConf[PlayerType].Clone()
	player.Id = ActorId(idgen.Next())
	player.Pos = NewInvalidPoint()

	gs.Players[player.Id] = player

	if playerStats == nil {
		gs.PlayersStats[player.Id] = &GameStats{}
	} else {
		gs.PlayersStats[player.Id] = playerStats
	}

	gs.PlayersLevelMetrics[player.Id] = &LevelMetrics{}

	if len(gs.Players) == 0 {
		gs.HostId = player.Id
	}

}

func (gs *GameSession) CreateMonsters(rng *rand.Rand) {
	genParams := gs.CurDungeonGenParams
	monsterCount := genParams.MinMonsters + rng.Intn(genParams.MaxMonsters-genParams.MinMonsters)
	monsters := gs.Map.GenerateActors(monsterCount, rng)
	monsterWeightsKV := conv.MapToKV(genParams.MonsterWeigths)
	slices.SortFunc(monsterWeightsKV, func(a, b conv.KV[ActorType, int]) int {
		return cmp.Compare(b.Val, a.Val)
	})

	for id, pos := range monsters {
		monsterType := getWeightedResult(monsterWeightsKV, rng)
		monster := gs.GameRules.ActorsConf[monsterType].Clone()
		monster.Id = ActorId(id)
		monster.Pos = pos
		gs.AddMonster(monster)
	}
}

func getWeightedResult[K interface{ ActorType | ItemType }, V ~int](weights []conv.KV[K, V], rng *rand.Rand) K {
	var totalWeight V = 0
	for _, w := range weights {
		totalWeight += w.Val
	}

	r := V(rng.Intn(int(totalWeight)))

	for _, weight := range weights {
		if r < weight.Val {
			return weight.Key
		}
		r -= weight.Val
	}

	return weights[0].Key
}

func (gs *GameSession) AdaptDifficulty() {
	var avgHP float64
	var avgFood float64

	for _, player := range gs.Players {
		if player.Vitals[HP] > 0 {
			avgHP += float64(player.Vitals[HP]) / float64(player.BaseAttrs[MaxHealth]) / float64(len(gs.Players))
			avgFood += float64(len(player.Backpack.Slots[ItemTypeFood])) / float64(len(gs.Players))
		}
	}

	// TODO: Weights are not taken into account at the moment: corrections are needed
	if avgHP < 0.25 && avgFood < 1.0 {
		gs.CurDungeonGenParams.DiffucltyAdjustment = 0.8
		gs.CurDungeonGenParams.ItemWeights[ItemTypeFood] += 50
	} else if avgHP > 0.8 && avgFood > 2.0 {
		gs.CurDungeonGenParams.DiffucltyAdjustment = 1.2
		gs.CurDungeonGenParams.ItemWeights[ItemTypeFood] = max(1, gs.CurDungeonGenParams.ItemWeights[ItemTypeFood]-20)
	}

}

func (gs *GameSession) HealPlayers() {
	for _, player := range gs.Players {
		if player.Vitals[HP] <= 0 {
			player.Vitals[HP] = int(float64(player.BaseAttrs[MaxHealth]) * gs.GameRules.HPRestore)
		} else {
			player.Vitals[HP] += int(min(float64(player.BaseAttrs[MaxHealth]), float64(player.BaseAttrs[MaxHealth])*gs.GameRules.HPRestore))
		}
	}
}

func (gs *GameSession) ClearMetrics() {
	for _, metrics := range gs.PlayersLevelMetrics {
		metrics.Clear()
	}
}

//
//
// --- PREDICATES ---
//
//

func (gs *GameSession) IsPlayer(actorId ActorId) bool {
	_, exists := gs.Players[actorId]
	return exists
}

func (gs *GameSession) IsPlayerDead(actorId ActorId) bool {
	if player, exists := gs.Players[actorId]; exists {
		return player.Vitals[HP] <= 0
	}

	return true
}

func (gs *GameSession) IsPlayerEscaped(actorId ActorId) bool {
	if player, exists := gs.Players[actorId]; exists {
		return player.Pos == NewInvalidPoint() && player.Vitals[HP] > 0
	}

	return false
}

func (gs *GameSession) AreAllPlayersDead() bool {
	deads := 0
	for _, player := range gs.Players {
		if player.Vitals[HP] <= 0 {
			deads++
		}
	}

	return deads == len(gs.Players)
}

func (gs *GameSession) AreAllAlivePlayersEscaped() bool {
	deads := 0
	escaped := 0
	for _, player := range gs.Players {
		if player.Pos == NewInvalidPoint() && player.Vitals[HP] > 0 {
			escaped++
		} else if player.Vitals[HP] <= 0 {
			deads++
		}
	}

	return escaped+deads == len(gs.Players)
}

func (gs *GameSession) IsMonster(actorId ActorId) bool {
	_, exists := gs.Monsters[actorId]
	return exists
}

func (gs *GameSession) IsLucky(chance int, rng *rand.Rand) bool {
	return rng.Intn(100) < chance
}

//
//
// --- SETTERS ---
//
//

func (gs *GameSession) AddItem(item *Item) {
	if oldItem, exists := gs.Items[item.Id]; exists {
		gs.Map.RemoveItem(item.Pos)
		delete(gs.Items, item.Id)
		log.Printf("Item with ID %v already exists. Old item was replaced by given one: pos %v, kind %v\n", oldItem.Id, item.Pos, item.Kind)
	}
	gs.Map.SetItem(item.Pos, int(item.Id))
	gs.Items[item.Id] = item
}

func (gs *GameSession) AddMonster(monster *Actor) {
	if oldMonster, exists := gs.Monsters[monster.Id]; exists {
		gs.Map.RemoveActor(oldMonster.Pos)
		delete(gs.Monsters, oldMonster.Id)
		log.Printf("Monster with ID %v already exists. Old monster was replaced by given one: pos %v, kind %v\n", oldMonster.Id, monster.Pos, monster.Kind)
	}
	gs.Map.SetActor(monster.Pos, int(monster.Id))
	gs.Monsters[monster.Id] = monster
}

// RemoveActor - deletes monster if monster's id was given and hides player if player's.
func (gs *GameSession) RemoveActor(id ActorId) {
	monster, exists := gs.Monsters[id]
	if exists {
		gs.Map.RemoveActor(monster.Pos)
		delete(gs.Monsters, id)
		return
	}

	player, exists := gs.Players[id]
	if exists {
		gs.Map.RemoveActor(player.Pos)
		gs.HidePlayer(player.Id)
	}
}

// HidePlayer - removes player from the map, but keeps it alive.
// After this function player will not be shown on the map.
func (gs *GameSession) HidePlayer(id ActorId) {
	if player, exists := gs.Players[id]; exists {
		gs.Map.RemoveActor(player.Pos)
		player.Pos = NewInvalidPoint()
	}
}

// ShowPlayers - shows all players in new dungeon level.
// Use this function when move players to the new dungeon level.
func (gs *GameSession) ShowPlayers(rng *rand.Rand) {
	m := gs.Map
	rooms := slices.Clone(m.Rooms)

	for _, player := range gs.Players {
		if player.Pos == NewInvalidPoint() {
			for len(rooms) > 0 {
				room := rooms[0]
				p, ok := m.TakeRandomActorPoint(room, rng)

				if ok {
					player.Pos = p
					m.SetActor(p, int(player.Id))
					break
				}
				rooms = rooms[1:]
			}
		}
	}
}

func (gs *GameSession) SpawnTreasures(actorPos geometry.Point, treasuresValue int) {
	itemId, itemPos, wasSpawned := gs.Map.SpawnLoot(actorPos, PlaceForLootSearchRad)
	if wasSpawned {
		treasures := NewTreasureItem(ItemId(itemId), itemPos, treasuresValue)
		gs.Items[treasures.Id] = treasures
	}
}
