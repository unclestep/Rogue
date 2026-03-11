// package service
//
// import (
// 	"log"
// 	"math/rand"
//
// 	"github.com/unclestep/Rogue/internal/domain/model"
// 	"github.com/unclestep/Rogue/pkg/geometry"
// )
//
// type Resolver struct {
// 	Session *model.GameSession
// 	Combat  *Combat
// 	Seed    int64
// 	Rng     *rand.Rand
// }
//
// func NewResolverService(session *model.GameSession, seed int64) *Resolver {
// 	return &Resolver{
// 		Session: session,
// 		Combat:  NewCombatService(session, seed),
// 		Seed:    seed,
// 		Rng:     rand.New(rand.NewSource(seed)),
// 	}
// }
//
// func (r *Resolver) SetSeed(seed int64) {
// 	r.Seed = seed
// 	r.Rng = rand.New(rand.NewSource(seed))
// 	r.Combat.SetSeed(seed)
// }
//
// func (r *Resolver) ResolveMove(event *MoveEvent) []Event {
// 	if event == nil {
// 		return nil
// 	}
//
// 	actor := event.Mover.Actor
// 	gameMap := r.Session.Map
// 	actorMap := r.Session.Monsters
//
// 	// Check if the actor moved
// 	curPos := actor.Pos
// 	newPos := curPos.Add(event.Mover.PosChange)
// 	if !gameMap.InBounds(newPos) {
// 		event.Mover.PosChange = geometry.Point{}
// 		event.Outcome = MoveOutcomeCantMove
// 		return []Event{event}
//
// 	}
// 	if newPos.Equal(curPos) { // Actor has no space to move
// 		// Reset stamina to escape infinite loops
// 		// No need to check if it is a player, because players can move to monsters
// 		event.Mover.ResetStamina()
// 		event.Outcome = MoveOutcomeNoSpace
// 		return []Event{event}
// 	}
//
// 	// If actor's new position is other actor's position, it is an attack
// 	if actorId := gameMap.ActorGrid[newPos.Y][newPos.X]; actorId > 0 {
// 		target, alive := actorMap[model.ActorId(actorId)]
//
// 		// Handle desync between gameMap and actorMap
// 		if !alive {
// 			gameMap.RemoveActor(newPos)
// 			log.Print("[INFO] Game map and actor map were desync and problem was fixed, however they should be always synchronized so double-check the code")
// 		} else if actor.Kind != model.ActorPlayer && target.Kind != model.ActorPlayer {
// 			// Monsters can't attack each other
// 			event.Mover.PosChange = geometry.NewDefaultPoint()
// 			// Reset stamina to escape infinite loops
// 			event.Mover.ResetStamina()
// 			event.Outcome = MoveOutcomeNoSpace
// 			return []Event{event}
// 		} else {
// 			attackEvent := r.Combat.ExecuteAttack(actor, target)
// 			// Forget about move event
// 			return []Event{attackEvent}
// 		}
// 	}
//
// 	// So, that is just a move after all
// 	// But need to check if we can move
// 	// or have enough stamina for move
// 	if !actor.CanMove() {
// 		if actor.Kind != model.ActorPlayer {
// 			event.Mover.ResetStamina()
// 		}
// 		event.Mover.PosChange = geometry.NewDefaultPoint()
// 		event.Outcome = MoveOutcomeCantMove
// 		return []Event{event}
// 	}
// 	if !actor.HasStaminaForMove() {
// 		event.Mover.PosChange = geometry.NewDefaultPoint()
// 		event.Outcome = MoveOutcomeNoStamina
// 		return []Event{event}
// 	}
//
// 	event.Mover.VitalsChange[model.VitalStamina] -= actor.DerivedAttrs[model.AttrMoveStaminaCost]
// 	event.Mover.Actor.MoveDir = event.Mover.PosChange.Normalize()
// 	model.ResolveReactions(model.TriggerOnMove, event.Mover, event.Mover, r.Rng)
// 	event.Mover.DecrementAllRelatedCharges(model.TriggerOnMove)
//
// 	events := []Event{event}
//
// 	// If there is an item in new position
// 	if itemId := gameMap.ItemGrid[newPos.Y][newPos.X]; itemId > 0 && actor.Kind == model.ActorPlayer {
// 		events = append(events, r.ResolveItemPickup(actor, model.ItemId(itemId)))
// 	}
//
// 	return events
// }
//
// type ItemPickupEvent struct {
// 	Actor *model.Actor
// 	Item  *model.Item
// }
//
// func NewItemPickupEvent(actor *model.Actor) *ItemPickupEvent {
// 	return &ItemPickupEvent{
// 		Actor: actor,
// 	}
// }
//
// func (r *Resolver) ResolveItemPickup(actor *model.Actor, itemId model.ItemId) *ItemPickupEvent {
// 	event := NewItemPickupEvent(actor)
//
// 	itemMap := r.Session.Items
// 	gameMap := r.Session.Map
// 	item, exists := itemMap[itemId]
//
// 	// Handle desync between gameMap and actorMap
// 	if !exists {
// 		gameMap.RemoveItem(item.Pos)
// 		log.Print("[INFO] Game map and item map were desync and problem was fixed, however they should be always synchronized so double-check the code")
// 		return event
// 	}
//
// 	if !actor.Backpack.CanAddItem(item.Kind) {
// 		return event
// 	}
//
// 	event.Item = item
// 	return event
// }
//
// func (event *ItemPickupEvent) Perform(gs *model.GameSession, rng *rand.Rand) {
// 	_ = rng
//
// 	if event.Actor == nil || event.Item == nil {
// 		return
// 	}
//
// 	gameMap := gs.Map
// 	itemMap := gs.Items
// 	actor := event.Actor
// 	item := event.Item
//
// 	gameMap.RemoveItem(item.Pos)
// 	item.Pos = model.NewInvalidPoint()
// 	actor.Backpack.Add(item)
//
// 	if item.Kind == model.ItemTypeTreasure {
// 		delete(itemMap, item.Id)
// 	}
// }
