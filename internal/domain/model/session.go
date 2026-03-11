package model

import (
	"math/rand"
	"slices"

	"github.com/unclestep/Rogue/pkg/algorithm"
	"github.com/unclestep/Rogue/pkg/geometry"
)

const (
	PlaceForLootSearchRad = 1
)

type GameSession struct {
	SessionId           SessionId
	HostId              ActorId                   `json:"host_id"`
	Map                 *Map                      `json:"map"`
	PlayersUuid         map[string]ActorId        `json:"players_uuid"`
	Players             map[ActorId]*Actor        `json:"players"`
	PlayersStats        map[ActorId]*GameStats    `json:"players_stats"`
	PlayersLevelMetrics map[ActorId]*LevelMetrics `json:"level_metrics"`
	Monsters            map[ActorId]*Actor        `json:"monsters"`
	Items               map[ItemId]*Item          `json:"items"`
	Depth               int                       `json:"depth"`
	State               GameState                 `json:"state"`
	NextId              int64                     `json:"next_id"`
	PendingIntents      map[ActorId]Intent
	Seed                int64
	rng                 *rand.Rand
}

type SessionId int

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

func NewGameSession(id SessionId, seed int64) *GameSession {
	gs := &GameSession{
		SessionId:           id,
		Map:                 NewDefaultMap(),
		Players:             make(map[ActorId]*Actor),
		PlayersStats:        make(map[ActorId]*GameStats),
		PlayersLevelMetrics: make(map[ActorId]*LevelMetrics),
		PlayersUuid:         make(map[string]ActorId),
		Monsters:            make(map[ActorId]*Actor),
		Items:               make(map[ItemId]*Item),
		Depth:               1,
		NextId:              1,
		PendingIntents:      make(map[ActorId]Intent),
		Seed:                seed,
		rng:                 rand.New(rand.NewSource(seed)),
	}
	return gs
}

func (gs *GameSession) GetId() int64 {
	id := gs.NextId
	gs.NextId++
	return id
}

func (gs *GameSession) GetPlayer(playerUuid string) *Actor {
	playerId, exists := gs.PlayersUuid[playerUuid]
	if !exists {
		return nil
	}

	return gs.Players[playerId]
}

func (gs *GameSession) InitRng() {
	if gs.rng == nil {
		gs.rng = rand.New(rand.NewSource(gs.Seed))
	}
}

func (gs *GameSession) SafeRng() *rand.Rand {
	if gs.rng == nil {
		gs.rng = rand.New(rand.NewSource(gs.Seed))
	}
	return gs.rng
}

func (gs *GameSession) Rng() *rand.Rand {
	return gs.rng
}

func (gs *GameSession) SaveRng() {
	gs.Seed = gs.rng.Int63()
}

//
//
// --- DUNGEON CREATION ---
//
//

func (gs *GameSession) PrepareNextDungeon() {
	gs.Depth++
	gs.AdaptDifficulty()
	gs.HealPlayers()
	gs.ClearMetrics()
}

func (gs *GameSession) AddPlayer(playerUuid string) ActorId {
	if playerInternalId, exists := gs.PlayersUuid[playerUuid]; exists {
		return playerInternalId
	}

	player := gs.GameRules.ActorsConf[ActorLabelPlayerCommon].Clone()
	player.Id = ActorId(gs.GetId())
	player.Pos = NewInvalidPoint()

	gs.Players[player.Id] = player
	gs.PlayersUuid[playerUuid] = player.Id
	gs.PlayersStats[player.Id] = &GameStats{}
	gs.PlayersLevelMetrics[player.Id] = &LevelMetrics{}

	if len(gs.Players) == 0 {
		gs.HostId = player.Id
	}

	return player.Id
}

// func (gs *GameSession) AdaptDifficulty() {
// 	var avgHP float64
// 	var avgFood float64
//
// 	for _, player := range gs.Players {
// 		if player.Vitals[VitalHP] > 0 {
// 			avgHP += float64(player.Vitals[VitalHP]) / float64(player.BaseAttrs[AttrMaxHP]) / float64(len(gs.Players))
// 			avgFood += float64(len(player.Backpack.Slots[ItemTypeFood])) / float64(len(gs.Players))
// 		}
// 	}
//
// 	// TODO: Weights are not taken into account at the moment: corrections are needed
// 	if avgHP < 0.25 && avgFood < 1.0 {
// 		gs.DifficultyAdjustment = 0.8
// 	} else if avgHP > 0.8 && avgFood > 2.0 {
// 		gs.DifficultyAdjustment = 1.2
// 	}
// }

// func (gs *GameSession) HealPlayers() {
// 	for _, player := range gs.Players {
// 		if player.Vitals[VitalHP] <= 0 {
// 			player.Vitals[VitalHP] = int(float64(player.BaseAttrs[AttrMaxHP]) * gs.GameRules.HpRestore)
// 		} else {
// 			player.Vitals[VitalHP] += int(min(float64(player.BaseAttrs[AttrMaxHP]), float64(player.BaseAttrs[AttrMaxHP])*gs.GameRules.HpRestore))
// 		}
// 	}
// }

func (gs *GameSession) ClearMetrics() {
	for _, metrics := range gs.PlayersLevelMetrics {
		metrics.Clear()
	}
}

//
//
// --- GETTERS ---
//
//

func (gs *GameSession) GetAlivePlayers() []ActorId {
	alive := make([]ActorId, 0, len(gs.Players))

	for id, player := range gs.Players {
		if player.Vitals[VitalHP] > 0 {
			alive = append(alive, id)
		} else {
			player.Pos = NewInvalidPoint()
		}
	}

	return alive
}

func (gs *GameSession) GetActor(id ActorId) *Actor {
	if player, exists := gs.Players[id]; exists {
		return player
	}

	if monster, exists := gs.Monsters[id]; exists {
		return monster
	}

	return nil
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
		return player.Vitals[VitalHP] <= 0
	}

	return true
}

func (gs *GameSession) IsPlayerEscaped(actorId ActorId) bool {
	if player, exists := gs.Players[actorId]; exists {
		return player.Pos == NewInvalidPoint() && player.Vitals[VitalHP] > 0
	}

	return false
}

func (gs *GameSession) AreAllPlayersDead() bool {
	dead := 0
	for _, player := range gs.Players {
		if player.Vitals[VitalHP] <= 0 {
			player.Pos = NewInvalidPoint()
			dead++
		}
	}

	return dead == len(gs.Players)
}

func (gs *GameSession) AreAllAlivePlayersEscaped() bool {
	dead := 0
	escaped := 0
	for _, player := range gs.Players {
		if player.Pos == NewInvalidPoint() && player.Vitals[VitalHP] > 0 {
			escaped++
		} else if player.Vitals[VitalHP] <= 0 {
			dead++
		}
	}

	return escaped+dead == len(gs.Players)
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
	oldItem, exists := gs.Items[item.Id]
	if exists {
		gs.Map.RemoveItem(oldItem.Pos)
	}

	gs.Map.SetItem(item.Pos, int64(item.Id))
	gs.Items[item.Id] = item
}

func (gs *GameSession) AddMonster(monster *Actor) {
	oldMonster, exists := gs.Monsters[monster.Id]
	if exists {
		gs.Map.RemoveActor(oldMonster.Pos)
	}

	gs.Map.SetActor(monster.Pos, int64(monster.Id))
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
	rooms := slices.Clone(m.rooms)
	i := 0

	for r, room := range rooms {
		if room.Id == m.EntranceRoomId {
			i = r
			break
		}
	}

	for _, player := range gs.Players {
		if player.Pos == NewInvalidPoint() {
			for len(rooms) > 0 {
				room := rooms[i]
				p, ok := m.TakeRandomActorPoint(room, rng)

				if ok {
					player.Pos = p
					m.SetActor(p, int64(player.Id))
					break
				}

				rooms = algorithm.Remove(rooms, i)
				i = 0
			}
		}
	}
}

// // SpawnTreasure TODO: remove from session
// func (gs *GameSession) SpawnTreasure(actorPos geometry.Point, treasureValue int) {
// 	itemPos, ok := gs.Map.FindEmptyPoint(actorPos, PlaceForLootSearchRad)
// 	if ok {
// 		id := gs.GetId()
// 		gs.Map.SetItem(itemPos, id)
// 		treasure := NewTreasureItem(ItemId(id), itemPos, treasureValue)
// 		gs.Items[treasure.Id] = treasure
// 	}
// }
