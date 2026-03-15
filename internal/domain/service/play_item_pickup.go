package service

import (
	"github.com/unclestep/Rogue/internal/domain/model"
)

//
//
// --- ITEM PICKUP SERVICE ---
//
//

type Pickup struct{}

//
// -- CONSTRUCTOR --
//

func NewPickupService() *Pickup {
	return &Pickup{}
}

//
// -- PICKUP EVENT --
//

type ItemPickupEvent struct {
	Actor        *model.ActorImpact
	PickupedItem *model.Item
	Outcome      PickupOutcome
}

type PickupOutcome int

const (
	PickupOutcomeSuccess PickupOutcome = iota
	PickupOutcomeBackpackNoFreeSpace
)

func NewItemPickupEvent(actor *model.Actor) *ItemPickupEvent {
	return &ItemPickupEvent{
		Actor: model.NewActorImpact(actor),
	}
}

func (p *Pickup) Pickup(playthrough *model.Playthrough, actor *model.Actor, item *model.Item) *ItemPickupEvent {
	if actor == nil || item == nil {
		return nil
	}

	event := NewItemPickupEvent(actor)

	if !actor.Backpack.CanAddItem(item.Kind) {
		event.Outcome = PickupOutcomeBackpackNoFreeSpace
		return event
	}

	event.PickupedItem = item
	return event
}

func (event *ItemPickupEvent) Perform(ctx *model.SessionContext) {
	if event.Actor == nil || event.PickupedItem == nil {
		return
	}

	ctx.Playthrough.RemoveItem(event.PickupedItem)
	event.Actor.Actor.Backpack.Add(event.PickupedItem)
}
