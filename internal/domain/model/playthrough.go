package model

import (
	"time"
)

type Playthrough struct {
	PlaythroughId       PlaythroughId
	HostId              ActorId
	RulesId             RulesId
	Map                 *Map                      `json:"map"`
	PlayersUuid         map[string]ActorId        `json:"players_uuid"`
	Players             map[ActorId]*Actor        `json:"players"`
	PlayersStats        map[ActorId]*GameStats    `json:"players_stats"`
	PlayersLevelMetrics map[ActorId]*LevelMetrics `json:"level_metrics"`
	PlayersFoW          map[ActorId]*VisibleArea
	Monsters            map[ActorId]*Actor `json:"monsters"`
	DeadMonsters        map[ActorId]*Actor
	Items               map[ItemId]*Item `json:"items"`
	Depth               int              `json:"depth"`
	State               GameState        `json:"state"`
	DungParams          *DungParams
	DynamicDifficulty   float64
	NextId              int64 `json:"next_id"`
	PendingIntents      map[ActorId]*Intent
	TurnEvents          []Event
	TurnDeadline        time.Time
	Seed                int64
}

type PlaythroughId int

const (
	InvalidPlaythroughId = 0
)

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

func NewPlaythrough(playId PlaythroughId, rulesId RulesId, seed int64) *Playthrough {
	gs := &Playthrough{
		PlaythroughId:       playId,
		RulesId:             rulesId,
		PlayersUuid:         make(map[string]ActorId),
		Players:             make(map[ActorId]*Actor),
		PlayersStats:        make(map[ActorId]*GameStats),
		PlayersLevelMetrics: make(map[ActorId]*LevelMetrics),
		PlayersFoW:          make(map[ActorId]*VisibleArea),
		Monsters:            make(map[ActorId]*Actor),
		DeadMonsters:        make(map[ActorId]*Actor),
		Items:               make(map[ItemId]*Item),
		Depth:               0,
		NextId:              1,
		PendingIntents:      make(map[ActorId]*Intent),
		Seed:                seed,
	}
	return gs
}

//
//
// --- DUNGEON CREATION ---
//
//

func (gs *Playthrough) AdaptDifficulty() (map[ItemLabel]int, map[ActorLabel]int) {
	var avgHP float64
	var avgFood float64
	updateItemWeights := make(map[ItemLabel]int)
	updateActorWeights := make(map[ActorLabel]int)

	for _, player := range gs.Players {
		if player.Vitals[VitalHP] > 0 {
			avgHP += float64(player.Vitals[VitalHP]) / float64(player.BaseAttrs[AttrMaxHP]) / float64(len(gs.Players))
			avgFood += float64(len(player.Backpack.Slots[ItemTypeFood])) / float64(len(gs.Players))
		}
	}

	if avgHP < 0.25 && avgFood < 1.0 {
		for _, item := range gs.Items {
			if item.Kind == ItemTypeFood {
				updateItemWeights[item.Label] = 25
			}
		}
		gs.DynamicDifficulty = 0.8
	} else if avgHP > 0.8 && avgFood > 2.0 {
		for _, item := range gs.Items {
			if item.Kind == ItemTypeFood {
				updateItemWeights[item.Label] = -25
			}
		}
		gs.DynamicDifficulty = 1.2
	}

	return updateItemWeights, updateActorWeights
}

func (gs *Playthrough) ClearMetrics() {
	for _, metrics := range gs.PlayersLevelMetrics {
		metrics.Clear()
	}
}

//
//
// --- GETTERS ---
//
//

func (gs *Playthrough) GetAlivePlayers() []ActorId {
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

func (gs *Playthrough) GetItem(id ItemId) *Item {
	item, _ := gs.Items[id]
	return item
}

func (gs *Playthrough) GetActor(id ActorId) *Actor {
	if player, exists := gs.Players[id]; exists {
		return player
	}

	if monster, exists := gs.Monsters[id]; exists {
		return monster
	}

	return nil
}

func (gs *Playthrough) GetPlayer(playerUuid string) *Actor {
	playerId, exists := gs.PlayersUuid[playerUuid]
	if !exists {
		return nil
	}

	player, exists := gs.Players[playerId]
	if exists {
		return player
	}

	return nil
}

func (gs *Playthrough) GetId() int64 {
	id := gs.NextId
	gs.NextId++
	return id
}

//
//
// --- PREDICATES ---
//
//

func (gs *Playthrough) IsPlayer(actorId ActorId) bool {
	_, exists := gs.Players[actorId]
	return exists
}

func (gs *Playthrough) IsMonster(actorId ActorId) bool {
	_, exists := gs.Monsters[actorId]
	return exists
}

func (gs *Playthrough) IsHost(playerUuid string) bool {
	internalId, _ := gs.PlayersUuid[playerUuid]
	return internalId == gs.HostId
}

func (gs *Playthrough) IsPlayerDead(actorId ActorId) bool {
	if player, exists := gs.Players[actorId]; exists {
		return player.Vitals[VitalHP] <= 0
	}

	return true
}

func (gs *Playthrough) IsPlayerEscaped(actorId ActorId) bool {
	if player, exists := gs.Players[actorId]; exists {
		return player.Pos == NewInvalidPoint() && player.Vitals[VitalHP] > 0
	}

	return false
}

func (gs *Playthrough) AreAllPlayersDead() bool {
	dead := 0
	for _, player := range gs.Players {
		if player.Vitals[VitalHP] <= 0 {
			player.Pos = NewInvalidPoint()
			dead++
		}
	}

	return dead == len(gs.Players)
}

func (gs *Playthrough) AreAllAlivePlayersEscaped() bool {
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

//
//
// --- SETTERS ---
//
//

func (gs *Playthrough) AddItem(item *Item) {
	if item == nil {
		return
	}

	oldItem, exists := gs.Items[item.Id]
	if exists && oldItem.Pos != NewInvalidPoint() {
		gs.Map.RemoveItem(oldItem.Pos)
	}

	gs.Map.SetItem(item.Pos, int64(item.Id))
	gs.Items[item.Id] = item
}

func (gs *Playthrough) AddMonster(monster *Actor) {
	if monster == nil {
		return
	}

	oldMonster, exists := gs.Monsters[monster.Id]
	if exists && oldMonster.Pos != NewInvalidPoint() {
		gs.Map.RemoveActor(oldMonster.Pos)
	}

	gs.Map.SetActor(monster.Pos, int64(monster.Id))
	gs.Monsters[monster.Id] = monster
}

func (gs *Playthrough) RemoveItem(item *Item) {
	if item == nil {
		return
	}

	item.Pos = NewInvalidPoint()
	gs.Map.RemoveItem(item.Pos)
}

func (gs *Playthrough) KillActor(actor *Actor) {
	if actor == nil {
		return
	}

	if gs.IsMonster(actor.Id) {
		actor.Pos = NewInvalidPoint()
		gs.Map.RemoveActor(actor.Pos)
		gs.DeadMonsters[actor.Id] = actor
		delete(gs.Monsters, actor.Id)
	}

	if gs.IsPlayer(actor.Id) {
		gs.HidePlayer(actor.Id)
	}
}

// RemoveActor - deletes completely actor.
func (gs *Playthrough) RemoveActor(actor *Actor) {
	if actor == nil {
		return
	}

	actor.Pos = NewInvalidPoint()
	gs.Map.RemoveActor(actor.Pos)
	delete(gs.Monsters, actor.Id)
	delete(gs.Players, actor.Id)
}

func (gs *Playthrough) DisconnectAll() {
	for _, playerId := range gs.PlayersUuid {
		gs.HidePlayer(playerId)
	}
}

func (gs *Playthrough) DisconnectPlayer(playerUuid string) {
	inId, exists := gs.PlayersUuid[playerUuid]
	if !exists {
		return
	}

	player, exists := gs.Players[inId]
	if !exists {
		return
	}

	gs.HidePlayer(player.Id)
}

// HidePlayer - removes player from the map, but keeps it alive.
// After this function player will not be shown on the map.
func (gs *Playthrough) HidePlayer(id ActorId) {
	if player, exists := gs.Players[id]; exists {
		gs.Map.RemoveActor(player.Pos)
		player.Pos = NewInvalidPoint()
	}
}
