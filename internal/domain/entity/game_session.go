package entity

import (
	"github.com/unclestep/Rogue/internal/pkg/geometry"
	"math/rand"
)

const (
	PlaceForLootSearchRad = 1
)

type GameSession struct {
	Player *Actor
	Map    *Map
	Actors map[ActorId]*Actor
	Items  map[ItemId]*Item
}

func NewGameSession() *GameSession {
	gs := &GameSession{
		Map: NewDefaultMap(),
	}
	return gs
}

func (gs *GameSession) IsLucky(chance int, rng *rand.Rand) bool {
	return rng.Intn(100) < chance
}

func (gs *GameSession) RemoveActor(id ActorId) {
	actor, exists := gs.Actors[id]
	if exists {
		gs.Map.RemoveActor(actor.Pos)
		delete(gs.Actors, id)
	}
}

func (gs *GameSession) SpawnTreasures(actorPos geometry.Point, treasuresValue int) {
	itemId, itemPos, wasSpawned := gs.Map.SpawnLoot(actorPos, PlaceForLootSearchRad)
	if wasSpawned {
		treasures := NewTreasureItem(ItemId(itemId), itemPos, treasuresValue)
		gs.Items[treasures.Id] = treasures
	}

}
