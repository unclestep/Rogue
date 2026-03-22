package model

import (
	"time"

	"github.com/unclestep/Rogue/pkg/geometry"
)

type Playthrough struct {
	PlaythroughId       PlaythroughId
	HostId              ActorId
	RulesId             RulesId
	Map                 *Map
	PlayersUuid         map[string]ActorId
	Players             map[ActorId]*Actor
	PlayersStats        map[ActorId]*GameStats
	PlayersLevelMetrics map[ActorId]*LevelMetrics
	PlayersFoW          map[ActorId]*VisibleArea
	PlayerAimAngles     map[ActorId]float64 // flashlight direction per player (radians; not persisted)
	PlayersNicknames    map[string]string   // UUID → display nickname; not persisted
	Monsters            map[ActorId]*Actor
	DeadMonsters        map[ActorId]*Actor
	Items               map[ItemId]*Item
	Depth               int
	State               GameState
	DungParams          *DungParams
	DynamicDifficulty   float64
	NextId              int64
	PendingIntents      []*Intent
	TurnEvents          []Event
	TurnDeadline        time.Time
	Seed                int64
}

type PlaythroughId string

const InvalidPlaythroughId PlaythroughId = ""

type GameStats struct {
	// Main statistics
	TotalTreasure int
	DeepestLevel  int
	// Additional statistics
	MonstersDefeated int
	FoodConsumed     int
	ElixirsDrunk     int
	ScrollsRead      int
	HitsDealt        int
	HitsReceived     int
	TilesTraveled    int
}

type GameState int

const (
	LobbyGameState GameState = iota
	PlayingGameState
	GameOverGameState
)

type LevelMetrics struct {
	DamageTaken int
	DamageDealt int
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
		Map:                 nil,
		PlayersUuid:         make(map[string]ActorId),
		Players:             make(map[ActorId]*Actor),
		PlayersStats:        make(map[ActorId]*GameStats),
		PlayersLevelMetrics: make(map[ActorId]*LevelMetrics),
		PlayersFoW:          make(map[ActorId]*VisibleArea),
		PlayerAimAngles:     make(map[ActorId]float64),
		PlayersNicknames:    make(map[string]string),
		Monsters:            make(map[ActorId]*Actor),
		DeadMonsters:        make(map[ActorId]*Actor),
		Items:               make(map[ItemId]*Item),
		Depth:               0,
		State:               LobbyGameState,
		DungParams:          nil,
		DynamicDifficulty:   1,
		NextId:              1,
		PendingIntents:      make([]*Intent, 0),
		TurnEvents:          make([]Event, 0),
		TurnDeadline:        time.Time{},
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

func (gs *Playthrough) HasPendingIntent(actorId ActorId) bool {
	for _, intent := range gs.PendingIntents {
		if intent != nil && intent.Actor == actorId {
			return true
		}
	}
	return false
}

func (gs *Playthrough) PendingPlayerIntentCount() int {
	count := 0
	for _, intent := range gs.PendingIntents {
		if intent != nil && gs.IsPlayer(intent.Actor) {
			count++
		}
	}
	return count
}

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

func (gs *Playthrough) IsDungeonCreated() bool {
	return gs.Map != nil && gs.Map.GetRoomCount() > 0
}

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

func (gs *Playthrough) GetKey(p geometry.Point) (*Item, bool) {
	id, _ := gs.Map.GetItemID(p)

	if id == 0 {
		return nil, false
	}

	item, exists := gs.Items[ItemId(id)]
	if !exists || item.Keyhole == 0 {
		return nil, false
	}

	return item, true
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

	gs.Map.RemoveItem(item.Pos)
	item.Pos = NewInvalidPoint()
}

func (gs *Playthrough) KillActor(actor *Actor) {
	if actor == nil {
		return
	}

	if gs.IsMonster(actor.Id) {
		// NOTE: POSITION STAYS VALID FOR SPAWN TREASURES
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

	gs.Map.RemoveActor(actor.Pos)
	actor.Pos = NewInvalidPoint()
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

// HidePlayer removes the player from the map grid and marks their position as
// invalid, but keeps them alive. Safe to call before the dungeon is generated
// (Map == nil) — the map removal is skipped in that case.
func (gs *Playthrough) HidePlayer(id ActorId) {
	if player, exists := gs.Players[id]; exists {
		if gs.Map != nil {
			gs.Map.RemoveActor(player.Pos)
		}
		player.Pos = NewInvalidPoint()
	}
}
