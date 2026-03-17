package model_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
)

func TestActorImpactGetEffectRespectsRemoval(t *testing.T) {
	actor := &model.Actor{
		Effects: make(map[model.EffectType]*model.Effect),
	}

	eff := &model.Effect{Kind: model.EffectFatigue}
	actor.Effects[model.EffectFatigue] = eff
	impact := model.NewActorImpact(actor)

	// Case 1: Effect exists and not removed
	if got := impact.GetEffect(model.EffectFatigue); got != eff {
		t.Errorf("expected to find effect, got nil")
	}

	// Case 2: Effect marked for removal should not be returned
	impact.EffectsToRemove[eff] = true
	if got := impact.GetEffect(model.EffectFatigue); got != nil {
		t.Errorf("expected nil for effect marked for removal, got %v", got)
	}
}

func TestActorImpactGetStatusAggregation(t *testing.T) {
	actor := &model.Actor{
		Effects: map[model.EffectType]*model.Effect{
			model.EffectSleep: {
				Kind:           model.EffectSleep,
				StatusesChange: map[model.StatusType]int{model.StatusSleep: 1},
			},
		},
	}
	impact := model.NewActorImpact(actor)

	newEff := &model.Effect{
		Kind:           model.EffectUnknown,
		StatusesChange: map[model.StatusType]int{model.StatusSleep: 1},
	}
	impact.AppliedEffects[model.EffectUnknown] = newEff

	val, _ := impact.GetStatus(model.StatusSleep)
	if val <= 0 {
		t.Errorf("expected status value > 0, got %d", val)
	}

	impact.EffectsToRemove[actor.Effects[model.EffectSleep]] = true
	val, origin := impact.GetStatus(model.StatusSleep)
	if origin != newEff {
		t.Errorf("expected status origin to be from applied effects after removal mark")
	}
}

func TestActorImpactConsumeEffect(t *testing.T) {
	eff := &model.Effect{Kind: model.EffectInfallible, Charges: 1}
	impact := model.NewActorImpact(&model.Actor{})

	impact.ConsumeEffect(eff)
	if impact.EffectChargesChange[eff] != -1 {
		t.Errorf("expected charges change -1, got %d", impact.EffectChargesChange[eff])
	}

	if !impact.EffectsToRemove[eff] {
		t.Errorf("effect should be marked for removal when charges reach 0")
	}
}

func TestActorImpactRemoveEffectsFromGear(t *testing.T) {
	gearEff := &model.Effect{Kind: model.EffectWeaponDefault}
	weapon := &model.Item{
		Kind:    model.ItemTypeWeapon,
		Effects: map[model.EffectType]*model.Effect{model.EffectWeaponDefault: gearEff},
	}

	actor := &model.Actor{
		EquippedGear: map[model.ItemType]*model.Item{model.ItemTypeWeapon: weapon},
		Effects:      make(map[model.EffectType]*model.Effect),
	}

	impact := model.NewActorImpact(actor)
	impact.EffectsToRemove[gearEff] = true

	removed := impact.RemoveEffects()
	if !removed {
		t.Errorf("expected RemoveEffects to return true")
	}

	if _, exists := weapon.Effects[model.EffectWeaponDefault]; exists {
		t.Errorf("effect was not removed from gear")
	}
}

func TestActorImpactCollectActorReactions(t *testing.T) {
	trigger := model.TriggerOnHit
	reaction := &model.Reaction{Trigger: trigger}

	actor := &model.Actor{
		Traits: map[model.TriggerType][]*model.Reaction{
			trigger: {reaction},
		},
		EquippedGear: make(map[model.ItemType]*model.Item),
		Effects:      make(map[model.EffectType]*model.Effect),
	}

	impact := model.NewActorImpact(actor)

	res := impact.CollectActorReactions(trigger)
	if len(res) != 1 {
		t.Errorf("expected 1 reaction from traits, got %d", len(res))
	}

	eff := &model.Effect{
		Procs: map[model.TriggerType][]*model.Reaction{trigger: {reaction}},
	}
	actor.Effects[model.EffectInfallible] = eff

	res = impact.CollectActorReactions(trigger)
	if len(res) != 2 {
		t.Errorf("expected 2 reactions (trait + effect), got %d", len(res))
	}

	impact.EffectsToRemove[eff] = true
	res = impact.CollectActorReactions(trigger)
	if len(res) != 1 {
		t.Errorf("expected reaction from removed effect to be ignored")
	}
}

// TestDeepNestedClone tests recursive deep copying of effects within reactions.
// This is a "sophisticated" case where an effect triggers a reaction that applies another effect.
func TestDeepNestedClone(t *testing.T) {
	// Level 3: The most nested effect
	innerEffect := &model.Effect{
		Kind:     model.EffectInfallible,
		Duration: 1,
	}

	// Level 2: Reaction that applies the Level 3 effect
	reaction := &model.Reaction{
		Trigger: model.TriggerEffectOnExpire,
		Chance:  model.Guaranteed,
		EffectsToApply: map[model.EffectType]*model.Effect{
			model.EffectInfallible: innerEffect,
		},
	}

	// Level 1: Outer effect that contains the reaction
	outerEffect := &model.Effect{
		Kind: model.EffectFatigue,
		Procs: map[model.TriggerType][]*model.Reaction{
			model.TriggerEffectOnExpire: {reaction},
		},
	}

	clone := outerEffect.Clone()

	// 1. Check if the Level 3 effect is actually cloned and not just pointed to
	clonedInner := clone.Procs[model.TriggerEffectOnExpire][0].EffectsToApply[model.EffectInfallible]
	if clonedInner == innerEffect {
		t.Errorf("Level 3 nested effect was not deep-copied (pointers are identical)")
	}

	// 2. Modify the original nested effect and ensure clone remains unchanged
	innerEffect.Duration = 999
	if clonedInner.Duration == 999 {
		t.Errorf("Modification of original nested effect leaked into the clone")
	}
}

// TestImpactRobustnessWithNilFields tests how ActorImpact handles partially initialized Actors.
// This simulates "impossible" states if an Actor is created bypassing constructors.
func TestImpactRobustnessWithNilFields(t *testing.T) {
	// Actor with nil maps (danger zone for panics)
	brokenActor := &model.Actor{
		Id:           666,
		Vitals:       nil,
		BaseAttrs:    nil,
		EquippedGear: nil,
		Effects:      nil,
	}

	impact := model.NewActorImpact(brokenActor)

	// Test 1: GetEffect on nil maps
	if eff := impact.GetEffect(model.EffectSleep); eff != nil {
		t.Errorf("Expected nil effect from nil maps, got %v", eff)
	}

	// Test 2: CollectReactions on nil maps
	// Should return empty slice, not panic
	reactions := impact.CollectActorReactions(model.TriggerOnHit)
	if len(reactions) != 0 {
		t.Errorf("Expected 0 reactions from broken actor, got %d", len(reactions))
	}
}

// TestChangeCalc_ExtremeScaling tests very high scale values and mixed holders.
func TestChangeCalcExtremeScaling(t *testing.T) {
	source := &model.Actor{
		DerivedAttrs: map[model.AttrType]int{model.AttrStrength: 1_000_000}, // Extreme value
	}
	opponent := &model.Actor{
		Vitals: map[model.VitalType]int{model.VitalHP: 10}, // Very low value
	}

	t.Run("Scaling from zero attribute", func(t *testing.T) {
		change := &model.Change{
			Holder: model.TargetOpponent,
			Attr:   model.AttrDexterity, // Opponent doesn't have this key in map
			Scale:  10.0,
			Amount: 5,
		}
		// Go maps return 0 for missing keys. 0 * 10.0 + 5 = 5.
		res := change.Calc(source, opponent)
		if res != 5 {
			t.Errorf("Expected 5 when attribute is missing (0), got %d", res)
		}
	})

	t.Run("Negative amount with positive scale", func(t *testing.T) {
		change := &model.Change{
			Holder: model.TargetSource,
			Attr:   model.AttrStrength,
			Amount: -1_000_000,
			Scale:  1.0,
		}
		// 1M + (-1M) = 0
		res := change.Calc(source, opponent)
		if res != 0 {
			t.Errorf("Expected 0 for offsetting extreme values, got %d", res)
		}
	})
}

// TestStatusConflictAndRemoval tests what happens when multiple sources provide the same status.
func TestStatusConflictAndRemoval(t *testing.T) {
	actor := &model.Actor{
		Effects: make(map[model.EffectType]*model.Effect),
	}

	// Source 1: Innate effect
	eff1 := &model.Effect{
		Kind:           model.EffectSleep,
		StatusesChange: map[model.StatusType]int{model.StatusSleep: 1},
	}
	actor.Effects[model.EffectSleep] = eff1

	impact := model.NewActorImpact(actor)

	// Source 2: Applied temporary effect
	eff2 := &model.Effect{
		Kind:           model.EffectFatigue, // Using different kind to allow coexistence
		StatusesChange: map[model.StatusType]int{model.StatusSleep: 1},
	}
	impact.AppliedEffects[model.EffectFatigue] = eff2

	// Both provide Sleep. Remove the first one.
	impact.EffectsToRemove[eff1] = true

	val, origin := impact.GetStatus(model.StatusSleep)
	if val != 1 {
		t.Errorf("Expected status Sleep to still be 1 from the second source")
	}
	if origin != eff2 {
		t.Errorf("Expected status origin to switch to eff2 after eff1 removal")
	}

	// Remove the second one too
	impact.EffectsToRemove[eff2] = true
	val, _ = impact.GetStatus(model.StatusSleep)
	if val != 0 {
		t.Errorf("Expected status Sleep to be 0 after all sources removed")
	}
}

// TestMassEffectRemovalCollision tests removing the same effect multiple times in one tick.
func TestMassEffectRemovalCollision(t *testing.T) {
	eff := &model.Effect{Kind: model.EffectFatigue}
	actor := &model.Actor{
		Effects: map[model.EffectType]*model.Effect{model.EffectFatigue: eff},
	}
	impact := model.NewActorImpact(actor)

	// Mark for removal 3 times
	impact.EffectsToRemove[eff] = true
	impact.EffectsToRemove[eff] = true

	// Add to applied effects and mark for removal there too
	impact.AppliedEffects[model.EffectFatigue] = eff

	// Run removal
	removed := impact.RemoveEffects()
	if !removed {
		t.Errorf("RemoveEffects failed to report successful removal")
	}

	if len(actor.Effects) != 0 || len(impact.AppliedEffects) != 0 {
		t.Errorf("Effect was not fully purged from all sources")
	}
}
