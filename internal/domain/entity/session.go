package entity

import (
	"math/rand"

	"github.com/unclestep/Rogue/pkg/geometry"
)

const (
	PlaceForLootSearchRad = 1
)

type GameSession struct {
	Map      *Map
	Players  map[ActorId]*Actor
	Monsters map[ActorId]*Actor
	Items    map[ItemId]*Item
}

func NewGameSession() *GameSession {
	gs := &GameSession{
		Map:      NewDefaultMap(),
		Players:  make(map[ActorId]*Actor),
		Monsters: make(map[ActorId]*Actor),
		Items:    make(map[ItemId]*Item),
	}
	return gs
}

//
//
// --- PREDICATES ---
//
//

func (gs *GameSession) IsPlayerExists(playerId ActorId) bool {
	_, exists := gs.Players[playerId]
	return exists
}

func (gs *GameSession) IsMonsterExists(monsterId ActorId) bool {
	_, exists := gs.Monsters[monsterId]
	return exists
}

func (gs *GameSession) IsLucky(chance int, rng *rand.Rand) bool {
	return rng.Intn(100) < chance
}

func (gs *GameSession) AddActor(actor *Actor) {
	gs.Map.SetActor(actor.Pos, int(actor.Id))
	gs.Monsters[actor.Id] = actor
}

func (gs *GameSession) RemoveActor(id ActorId) {
	actor, exists := gs.Monsters[id]
	if exists {
		gs.Map.RemoveActor(actor.Pos)
		delete(gs.Monsters, id)
	}
}

func (gs *GameSession) SpawnTreasures(actorPos geometry.Point, treasuresValue int) {
	itemId, itemPos, wasSpawned := gs.Map.SpawnLoot(actorPos, PlaceForLootSearchRad)
	if wasSpawned {
		treasures := NewTreasureItem(ItemId(itemId), itemPos, treasuresValue)
		gs.Items[treasures.Id] = treasures
	}
}
