package service

import (
	"log"
	"math/rand"
	"time"

	"github.com/unclestep/Rogue/internal/domain/entity"
	"github.com/unclestep/Rogue/internal/pkg/geometry"
)

type Resolver struct {
	session *entity.GameSession
	combat  *Combat
	move    *Move
	seed    int64
	rng     *rand.Rand
}

func (resolver *Resolver) SetSeed(seed int64) {
	resolver.seed = seed
	resolver.rng = rand.New(rand.NewSource(seed))
	resolver.combat.SetSeed(resolver.seed)
	resolver.move.SetSeed(resolver.combat.seed)
}

func NewResolver(session *entity.GameSession, combat *Combat, movement *Move) *Resolver {
	resolver := &Resolver{
		session: session,
		combat:  combat,
		move:    movement,
		seed:    time.Now().UnixNano(),
	}
	resolver.rng = rand.New(rand.NewSource(resolver.seed))
	combat.SetSeed(resolver.seed)
	movement.SetSeed(resolver.combat.seed)

	return resolver
}

type ItemPickupEvent struct {
	Actor *entity.Actor
	Item  *entity.Item
}

func NewItemPickupEvent(actor *entity.Actor) *ItemPickupEvent {
	return &ItemPickupEvent{
		Actor: actor,
	}
}

func (event *ItemPickupEvent) Perform(gs *entity.GameSession, rng *rand.Rand) {
	_ = rng

	if event.Actor == nil || event.Item == nil {
		return
	}

	gameMap := gs.Map
	itemMap := gs.Items
	actor := event.Actor
	item := event.Item

	gameMap.RemoveItem(item.Pos)
	item.Pos = entity.NewInvalidPoint()
	actor.Backpack.Add(item)

	if item.Kind == entity.ItemTypeTreasure {
		delete(itemMap, item.Id)
	}
}

func (resolver *Resolver) ResolveMove(moveEvent *MoveEvent) []Event {
	events := make([]Event, 2, 2)

	if !moveEvent.WasPerformed() {
		events[0] = moveEvent
		return events
	}

	// Check if the actor moved
	curPos := moveEvent.Mover.Actor.Pos
	newPos := curPos.Add(moveEvent.Mover.PosChange)
	if newPos.Equal(curPos) { // Actor has no space to move
		events[0] = moveEvent
		return events
	}

	actor := moveEvent.Mover.Actor
	gameMap := resolver.session.Map
	actorMap := resolver.session.Actors

	// If actor's new position is other actor's position, it is an attack
	if actorId, inBounds := gameMap.GetActorID(newPos); inBounds && actorId > 0 {
		target, alive := actorMap[entity.ActorId(actorId)]

		// Handle desync between gameMap and actorMap
		if !alive {
			gameMap.RemoveActor(newPos)
			log.Print("[INFO] Game map and actor map were desync and problem was fixed, however they should be always synchronized so double-check the code")
		} else if actor.Kind != entity.PlayerType && target.Kind != entity.PlayerType {
			// Monsters can't attack each other
			moveEvent.Mover.PosChange = geometry.NewDefaultPoint()
			events[0] = moveEvent
			return events
		} else {
			attackEvent := resolver.combat.ExecuteAttack(actor, target)
			// Forget about move event
			events[0] = attackEvent
			return events
		}
	}

	events[0] = moveEvent

	// If there is an item in new position
	if itemId, inBounds := gameMap.GetItemID(newPos); inBounds && itemId > 0 {
		events[1] = resolver.ResolveItemPickup(actor, entity.ItemId(itemId))
	}

	return events
}

func (resolver *Resolver) ResolveItemPickup(actor *entity.Actor, itemId entity.ItemId) *ItemPickupEvent {
	event := NewItemPickupEvent(actor)

	itemMap := resolver.session.Items
	gameMap := resolver.session.Map
	item, exists := itemMap[itemId]

	// Handle desync between gameMap and actorMap
	if !exists {
		gameMap.RemoveItem(item.Pos)
		log.Print("[INFO] Game map and item map were desync and problem was fixed, however they should be always synchronized so double-check the code")
		return event
	}

	if !actor.Backpack.CanAddItem(item.Kind) {
		return event
	}

	event.Item = item
	return event
}
