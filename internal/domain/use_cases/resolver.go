package usecases

import (
	"log"
	"math/rand"
	"time"

	"github.com/unclestep/Rogue/internal/domain/entity"
)

type ActionResolver struct {
	session  *entity.GameSession
	combat   *CombatService
	movement *MovementService
	seed     int64
	rng      *rand.Rand
}

func NewActionResolver(session *entity.GameSession, combat *CombatService, movement *MovementService) *ActionResolver {
	resolver := &ActionResolver{
		session:  session,
		combat:   combat,
		movement: movement,
		seed:     time.Now().UnixNano(),
	}
	resolver.rng = rand.New(rand.NewSource(resolver.seed))

	return resolver
}

func (resolver *ActionResolver) ResolveMove(moveEvent *MoveEvent) {
	if !moveEvent.WasPerformed() {
		return
	}

	// Check if the actor moved
	curPos := moveEvent.Mover.Actor.Pos
	newPos := curPos.Add(moveEvent.Mover.PosChange)
	if newPos.Equal(curPos) { // Actor has no space to move
		return
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
			return
		} else {
			attackEvent := resolver.combat.ExecuteAttack(actor, target)
			// Forget about move event and perform attack event
			attackEvent.Perform(resolver.session, resolver.rng)
			return
		}
	}

	// New position is free of actors so can perform move event
	moveEvent.Perform(resolver.session, resolver.rng)

	// If there is an item in new position
	if itemId, inBounds := gameMap.GetItemID(newPos); inBounds && itemId > 0 {
		resolver.ResolveItemPickup(actor, entity.ItemId(itemId))
	}
}

func (resolver *ActionResolver) ResolveItemPickup(actor *entity.Actor, itemId entity.ItemId) {
	itemMap := resolver.session.Items
	gameMap := resolver.session.Map

	item, exists := itemMap[itemId]

	// Handle desync between gameMap and actorMap
	if !exists {
		gameMap.RemoveItem(item.Pos)
		log.Print("[INFO] Game map and item map were desync and problem was fixed, however they should be always synchronized so double-check the code")
		return
	}

	if !actor.Backpack.CanAddItem(item.Kind) {
		return
	}

	gameMap.RemoveItem(item.Pos)
	item.Pos = entity.NewInvalidPoint()
	actor.Backpack.Add(item)

	if item.Kind == entity.ItemTypeTreasure {
		delete(itemMap, item.Id)
	}
}
