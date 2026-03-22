package model

import (
	"github.com/unclestep/Rogue/pkg/geometry"
)

type ActorImpact struct {
	Actor               *Actor
	PosChange           geometry.Point
	VitalsChange        map[VitalType]int
	BaseAttrsChange     map[AttrType]int
	StatusesChange      map[StatusType]int
	AppliedEffects      map[EffectType]*Effect
	EffectChargesChange map[*Effect]int
	EffectsToRemove     map[*Effect]bool
}

func NewActorImpact(actor *Actor) *ActorImpact {
	return &ActorImpact{
		Actor:               actor,
		VitalsChange:        make(map[VitalType]int),
		BaseAttrsChange:     make(map[AttrType]int),
		StatusesChange:      make(map[StatusType]int),
		AppliedEffects:      make(map[EffectType]*Effect),
		EffectChargesChange: make(map[*Effect]int),
		EffectsToRemove:     make(map[*Effect]bool),
	}
}

//
// -- GETTERS --
//

func (impact *ActorImpact) GetEffect(effectType EffectType) *Effect {
	personalEffect, hasInPersonal := impact.Actor.Effects[effectType]
	toRemove := impact.EffectsToRemove[personalEffect]
	if hasInPersonal && !toRemove {
		return personalEffect
	}

	appliedEffect, hasInApplied := impact.AppliedEffects[effectType]
	toRemove = impact.EffectsToRemove[appliedEffect]
	if hasInApplied && !toRemove {
		return appliedEffect
	}

	return nil
}

func (impact *ActorImpact) GetStatus(statusType StatusType) (int, *Effect) {
	for _, effect := range impact.Actor.Effects {
		statusChange := effect.StatusesChange[statusType]
		toRemove := impact.EffectsToRemove[effect]
		if statusChange > 0 && !toRemove {
			return statusChange, effect
		}
	}

	for _, effect := range impact.AppliedEffects {
		statusChange, statusExists := effect.StatusesChange[statusType]
		toRemove := impact.EffectsToRemove[effect]
		if statusExists && statusChange > 0 && !toRemove {
			return statusChange, effect
		}
	}

	return 0, nil
}

//
// -- SETTERS --
//

func (impact *ActorImpact) ResetStamina() {
	impact.VitalsChange[VitalStamina] -= impact.Actor.Vitals[VitalStamina]
}

//
// -- MUTATORS --
//

func (impact *ActorImpact) ConsumeEffect(effect *Effect) {
	if effect.IsLimited() && !impact.EffectsToRemove[effect] {
		impact.EffectChargesChange[effect] -= 1
		if effect.Charges+impact.EffectChargesChange[effect] <= 0 {
			impact.EffectsToRemove[effect] = true
		}
	}
}

// RemoveEffects - removes all effects which were marked as needed to remove.
// These effects do not necessarily expire or run out.
// NOTE: Need to manually recompute actor's stats after this.
func (impact *ActorImpact) RemoveEffects() {
	for effect := range impact.EffectsToRemove {
		effectOrigins := impact.whereEffectFrom(effect)
		for _, origin := range effectOrigins {
			impact.removeEffect(origin, effect)
		}
	}
	clear(impact.EffectsToRemove)
}

// whereEffectFrom - finds effect's origin.
// Very likely it is straight from actor's active effects, but it also could be from actor's equipped gear.
func (impact *ActorImpact) whereEffectFrom(effect *Effect) []map[EffectType]*Effect {
	origins := make([]map[EffectType]*Effect, 0) // If the same effect (in terms of pointers) is in multiple origins (rare case)
	if eff, exists := impact.Actor.Effects[effect.Kind]; exists {
		if eff == effect {
			origins = append(origins, impact.Actor.Effects)
		}
	}

	for _, gear := range impact.Actor.EquippedGear {
		if eff, exists := gear.Effects[effect.Kind]; exists {
			if eff == effect {
				origins = append(origins, gear.Effects)
			}
		}
	}

	if appliedEffect, exists := impact.AppliedEffects[effect.Kind]; exists {
		if appliedEffect == effect {
			origins = append(origins, impact.AppliedEffects)
		}
	}

	return origins
}

// removeEffect - removes effect from its origin.
// NOTE: Need to manually recompute actor's stats after this.
func (impact *ActorImpact) removeEffect(origin map[EffectType]*Effect, effect *Effect) bool {
	if origin == nil {
		return false
	}

	removed := false
	if _, exists := origin[effect.Kind]; exists {
		delete(origin, effect.Kind)
		delete(impact.EffectsToRemove, effect)
		removed = true
	}
	return removed
}

//
//
// --- REACTION RELATED METHODS ---
//
//

func (impact *ActorImpact) CollectActorReactions(trigger TriggerType) []*Reaction {
	actor := impact.Actor
	reactions := make([]*Reaction, 0)

	// Innate traits
	reactions = append(reactions, actor.Traits[trigger]...)

	// Gear procs
	for _, gear := range actor.EquippedGear {
		for _, effect := range gear.Effects {
			if _, toRemove := impact.EffectsToRemove[effect]; !toRemove {
				reactions = append(reactions, effect.Procs[trigger]...)
			}
		}
		reactions = append(reactions, gear.Procs[trigger]...)
	}

	// Effect procs
	for _, effect := range actor.Effects {
		if _, toRemove := impact.EffectsToRemove[effect]; !toRemove {
			reactions = append(reactions, effect.Procs[trigger]...)
		}
	}

	return reactions
}

func (impact *ActorImpact) DecrementAllRelatedCharges(trigger TriggerType) {
	impact.DecrementEffectsCharges(trigger, impact.Actor.Effects)
	for _, gear := range impact.Actor.EquippedGear {
		impact.DecrementEffectsCharges(trigger, gear.Effects)

	}
}

func (impact *ActorImpact) DecrementEffectsCharges(trigger TriggerType, effects map[EffectType]*Effect) {
	for _, effect := range effects {
		if _, exists := impact.EffectsToRemove[effect]; exists {
			continue
		}

		if effect.ConsumeOn == trigger && effect.Charges > 0 {
			impact.EffectChargesChange[effect] -= 1
			if effect.Charges+impact.EffectChargesChange[effect] == 0 {
				impact.EffectsToRemove[effect] = true
			}
		}
	}
}
