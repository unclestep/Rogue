package service

import (
	"maps"

	"github.com/unclestep/Rogue/internal/domain/model"
)

type ItemUsage struct{}

func NewItemUsageService() *ItemUsage {
	return &ItemUsage{}
}

type ItemUsageEvent struct {
	Outcome       ItemUsageOutcome
	User          *model.ActorImpact
	RetrievedItem *model.Item
	ItemToEquip   *model.Item
	ItemToUnequip *model.Item
	DroppedItem   *model.Item
}

type ItemUsageOutcome int

const (
	ItemUsageOutcomeSuccess ItemUsageOutcome = iota
	ItemUsageOutcomeNotFound
	ItemUsageOutcomeCantUnequip
	ItemUsageOutcomeNoStamina
)

func NewItemUsageEvent(a *model.Actor) *ItemUsageEvent {
	return &ItemUsageEvent{
		User: model.NewActorImpact(a),
	}
}

func (event *ItemUsageEvent) Perform(ctx *model.SessionContext) {
	actor := event.User.Actor

	impactResolver := NewImpactResolverService()
	impactResolver.ApplyImpact(event.User, ctx.Rng())

	if event.RetrievedItem != nil {
		actor.Backpack.RetrieveById(event.RetrievedItem.Id)
	}

	if event.ItemToEquip != nil {
		if actor.EquippedGear == nil {
			actor.EquippedGear = make(map[model.ItemType]*model.Item)
		}
		event.ItemToEquip.Pos = model.NewInvalidPoint()
		actor.EquippedGear[event.ItemToEquip.Kind] = event.ItemToEquip
	}

	if event.ItemToUnequip != nil {
		delete(actor.EquippedGear, event.ItemToUnequip.Kind)
		actor.Backpack.Add(event.ItemToUnequip)
	}

	if event.DroppedItem != nil {
		if event.ItemToEquip != nil && actor.EquippedGear[event.DroppedItem.Kind] == event.DroppedItem {
			delete(actor.EquippedGear, event.DroppedItem.Kind)
		}

		dropPos, ok := ctx.Playthrough.Map.FindEmptyPoint(event.DroppedItem.Pos, 1)
		if ok {
			ctx.Playthrough.Map.SetItem(dropPos, int64(event.DroppedItem.Id))
			ctx.Playthrough.Items[event.DroppedItem.Id] = event.DroppedItem
		}
	}
}

func (iu *ItemUsage) ConsumeItem(playthrough *model.Playthrough, item *model.Item, a *model.Actor) *ItemUsageEvent {
	if item == nil || a == nil {
		return nil
	}

	event := NewItemUsageEvent(a)

	if !a.HasStaminaForAction() {
		event.Outcome = ItemUsageOutcomeNoStamina
		return event
	}

	event.User.VitalsChange = maps.Clone(item.VitalsChange)
	event.User.VitalsChange[model.VitalStamina] -= a.DerivedAttrs[model.AttrActionStaminaCost]
	event.User.BaseAttrsChange = maps.Clone(item.BaseAttrsChange)
	event.User.AppliedEffects = item.CloneEffects()
	event.RetrievedItem = item

	return event
}

func (iu *ItemUsage) EquipItem(playthrough *model.Playthrough, newItem *model.Item, a *model.Actor) *ItemUsageEvent {
	if newItem == nil || a == nil {
		return nil
	}

	event := NewItemUsageEvent(a)

	if !a.HasStaminaForAction() {
		event.Outcome = ItemUsageOutcomeNoStamina
		return event
	}

	oldItem, equipped := a.EquippedGear[newItem.Kind]

	if equipped {
		event.DroppedItem = oldItem
	}

	event.RetrievedItem = newItem
	event.ItemToEquip = newItem
	event.User.VitalsChange[model.VitalStamina] -= a.DerivedAttrs[model.AttrActionStaminaCost]

	return event
}

func (iu *ItemUsage) UnequipItem(item *model.Item, a *model.Actor) *ItemUsageEvent {
	if item == nil || a == nil {
		return nil
	}

	event := NewItemUsageEvent(a)

	if !a.HasStaminaForAction() {
		event.Outcome = ItemUsageOutcomeNoStamina
		return event
	}

	_, equipped := a.EquippedGear[item.Kind]
	if !equipped {
		event.Outcome = ItemUsageOutcomeCantUnequip
		return event
	}

	if !a.Backpack.CanAddItem(item.Kind) {
		event.Outcome = ItemUsageOutcomeCantUnequip
		return event
	}

	event.ItemToUnequip = item
	event.User.VitalsChange[model.VitalStamina] -= a.DerivedAttrs[model.AttrActionStaminaCost]

	return event
}
