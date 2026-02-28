package service

import (
	"log"
	"math/rand"

	"github.com/unclestep/Rogue/internal/domain/entity"
	"github.com/unclestep/Rogue/pkg/geometry"
)

type Resolver struct {
	Session *entity.GameSession
	Combat  *Combat
	Seed    int64
	Rng     *rand.Rand
}

func NewResolverService(session *entity.GameSession, seed int64) *Resolver {
	return &Resolver{
		Session: session,
		Combat:  NewCombatService(session, seed),
		Seed:    seed,
		Rng:     rand.New(rand.NewSource(seed)),
	}
}

func (r *Resolver) SetSeed(seed int64) {
	r.Seed = seed
	r.Rng = rand.New(rand.NewSource(seed))
}

func (r *Resolver) ResolveMove(event *MoveEvent) []Event {
	// Check if the actor moved
	curPos := event.Mover.Actor.Pos
	newPos := curPos.Add(event.Mover.PosChange)
	if newPos.Equal(curPos) { // Actor has no space to move
		// Reset stamina to escape infinite loops
		// No need to check if it is a player, because players can move to monsters
		event.Mover.ResetStamina()
		event.Outcome = MoveOutcomeNoSpace
		return []Event{event}
	}

	actor := event.Mover.Actor
	gameMap := r.Session.Map
	actorMap := r.Session.Monsters

	// If actor's new position is other actor's position, it is an attack
	if actorId, inBounds := gameMap.GetActorID(newPos); inBounds && actorId > 0 {
		target, alive := actorMap[entity.ActorId(actorId)]

		// Handle desync between gameMap and actorMap
		if !alive {
			gameMap.RemoveActor(newPos)
			log.Print("[INFO] Game map and actor map were desync and problem was fixed, however they should be always synchronized so double-check the code")
		} else if actor.Kind != entity.PlayerType && target.Kind != entity.PlayerType {
			// Monsters can't attack each other
			event.Mover.PosChange = geometry.NewDefaultPoint()
			// Reset stamina to escape infinite loops
			event.Mover.ResetStamina()
			event.Outcome = MoveOutcomeNoSpace
			return []Event{event}
		} else {
			attackEvent := r.Combat.ExecuteAttack(actor, target)
			// Forget about move event
			return []Event{attackEvent}
		}
	}

	// So, that is just a move after all
	// But need to check if we can move
	// or have enough stamina for move
	if !actor.CanMove() {
		if actor.Kind != entity.PlayerType {
			event.Mover.ResetStamina()
		}
		event.Outcome = MoveOutcomeCantMove
		return []Event{event}
	}
	if !actor.HasStaminaForMove() {
		event.Outcome = MoveOutcomeNoStamina
		return []Event{event}
	}

	event.Mover.VitalsChange[entity.Stamina] -= actor.DerivedAttrs[entity.MoveStaminaCost]
	entity.ResolveReactions(entity.TriggerOnMove, event.Mover, event.Mover, r.Rng)
	event.Mover.DecrementAllRelatedCharges(entity.TriggerOnMove)

	events := []Event{event}

	// If there is an item in new position
	if itemId, inBounds := gameMap.GetItemID(newPos); inBounds && itemId > 0 && actor.Kind == entity.PlayerType {
		events = append(events, r.ResolveItemPickup(actor, entity.ItemId(itemId)))
	}

	return events
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

func (r *Resolver) ResolveItemPickup(actor *entity.Actor, itemId entity.ItemId) *ItemPickupEvent {
	event := NewItemPickupEvent(actor)

	itemMap := r.Session.Items
	gameMap := r.Session.Map
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
