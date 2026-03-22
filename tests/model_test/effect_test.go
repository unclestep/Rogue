package model_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func TestEffectClone(t *testing.T) {
	original := &model.Effect{
		Kind:     model.EffectFatigue,
		Duration: 5,
		Charges:  -1,
		AttrsChange: map[model.AttrType]int{
			model.AttrStrength: -5,
		},
		Procs: map[model.TriggerType][]*model.Reaction{
			model.TriggerEffectOnExpire: {
				{
					Target: model.TargetSource,
					Chance: model.Guaranteed,
					EffectsToApply: map[model.EffectType]*model.Effect{
						model.EffectUntouchable: {
							Kind:     model.EffectUntouchable,
							Duration: 1,
						},
					},
				},
			},
		},
	}

	clone := original.Clone()

	if clone.Kind != original.Kind || clone.Duration != original.Duration {
		t.Errorf("Base fields were not cloned properly")
	}

	original.AttrsChange[model.AttrStrength] = -10
	if clone.AttrsChange[model.AttrStrength] == -10 {
		t.Errorf("AttrsChange was not deep copied")
	}

	originalReact := original.Procs[model.TriggerEffectOnExpire][0]
	cloneReact := clone.Procs[model.TriggerEffectOnExpire][0]

	originalReact.Chance = 50
	if cloneReact.Chance == 50 {
		t.Errorf("Reaction inside Procs was not deep copied")
	}

	originalNestedEff := originalReact.EffectsToApply[model.EffectUntouchable]
	cloneNestedEff := cloneReact.EffectsToApply[model.EffectUntouchable]

	originalNestedEff.Duration = 99
	if cloneNestedEff.Duration == 99 {
		t.Errorf("Nested effect inside Reaction was not deep copied")
	}
}

func TestChangeCalc(t *testing.T) {
	source := model.NewDefaultPlayer(1, geometry.Point{X: 1, Y: 1}, 9)
	opponent := model.NewDefaultPlayer(2, geometry.Point{X: 1, Y: 1}, 9)

	t.Run("Flat amount", func(t *testing.T) {
		change := &model.Change{
			Holder: model.TargetSource,
			Vital:  model.VitalUnknown,
			Attr:   model.AttrStrength,
			Amount: 15,
			Scale:  0.0,
		}
		res := change.Calc(source, opponent)
		if res != 15 {
			t.Errorf("Expected 15, got %d", res)
		}
	})

	t.Run("Scale from source attribute", func(t *testing.T) {
		startStrength := source.DerivedAttrs[model.AttrStrength]
		change := &model.Change{
			Holder: model.TargetSource,
			Vital:  model.VitalUnknown,
			Attr:   model.AttrStrength,
			Amount: 5,
			Scale:  1,
		}
		res := change.Calc(source, opponent)
		if res != startStrength+5 {
			t.Errorf("Expected %v, got %d", startStrength+5, res)
		}
	})

	t.Run("Scale from opponent vitals", func(t *testing.T) {
		hp := opponent.Vitals[model.VitalHP]
		change := &model.Change{
			Holder: model.TargetOpponent,
			Vital:  model.VitalHP,
			Attr:   model.AttrUnknown,
			Amount: 0,
			Scale:  0.1,
		}
		expected := int(float64(hp) * 0.1)
		res := change.Calc(source, opponent)
		if res != expected {
			t.Errorf("Expected %v, got %d", expected, res)
		}
	})

	t.Run("Invalid holder returns 0", func(t *testing.T) {
		change := &model.Change{
			Holder: model.TargetNotSpecified,
			Vital:  model.VitalHP,
			Amount: 10,
			Scale:  1.0,
		}
		res := change.Calc(source, opponent)
		if res != 0 {
			t.Errorf("Expected 0 due to invalid holder, got %d", res)
		}
	})
}

func TestDecrementEffectsCharges(t *testing.T) {
	actor := model.NewDefaultPlayer(1, geometry.Point{X: 0, Y: 0}, 9)
	impact := model.NewActorImpact(actor)

	dexBoost := &model.Effect{
		Kind:      model.EffectElixirDexterity,
		Charges:   1,
		ConsumeOn: model.TriggerNotSpecified,
	}

	fatigue := &model.Effect{
		Kind:      model.EffectFatigue,
		Charges:   1,
		ConsumeOn: model.TriggerNotSpecified,
	}

	sleep := &model.Effect{
		Kind:      model.EffectSleep,
		Charges:   2,
		ConsumeOn: model.TriggerNotSpecified,
	}

	actor.Effects[model.EffectElixirDexterity] = dexBoost
	actor.Effects[model.EffectFatigue] = fatigue
	actor.EquippedGear[model.ItemTypeWeapon] = &model.Item{
		Kind: model.ItemTypeWeapon,
		Effects: map[model.EffectType]*model.Effect{
			model.EffectSleep: sleep,
		},
	}

	impact.DecrementAllRelatedCharges(model.TriggerNotSpecified)

	if change := impact.EffectChargesChange[dexBoost]; change != -1 {
		t.Errorf("Dexterity boost elixir charges change should be %v, got %v\n", -1, change)
	}

	if change := impact.EffectChargesChange[fatigue]; change != -1 {
		t.Errorf("Fatigue charges change should be %v, got %v\n", -1, change)
	}

	if change := impact.EffectChargesChange[sleep]; change != -1 {
		t.Errorf("Weapon's sleep effect charges change should be %v, got %v\n", -1, change)
	}

	if len(impact.EffectsToRemove) != 2 {
		t.Errorf("Length of effects to remove should be %v, got %v", 2, len(impact.EffectsToRemove))
	}

	impact.DecrementAllRelatedCharges(model.TriggerNotSpecified)

	if change := impact.EffectChargesChange[dexBoost]; change != -1 {
		t.Errorf("Dexterity boost elixir charges change should be %v, got %v\n", -1, change)
	}

	if change := impact.EffectChargesChange[fatigue]; change != -1 {
		t.Errorf("Fatigue charges change should be %v, got %v\n", -1, change)
	}

	if change := impact.EffectChargesChange[sleep]; change != -2 {
		t.Errorf("Weapon's sleep effect charges change should be %v, got %v\n", -2, change)
	}

	if len(impact.EffectsToRemove) != 3 {
		t.Errorf("Length of effects to remove should be %v, got %v", 3, len(impact.EffectsToRemove))
	}

	impact.RemoveEffects()

	if effect := actor.Effects[model.EffectElixirDexterity]; effect != nil {
		t.Errorf("Dexterity boost elixir should be deleted")
	}

	if effect := actor.Effects[model.EffectFatigue]; effect != nil {
		t.Errorf("Fatigue should be deleted")
	}

	if effect := actor.EquippedGear[model.ItemTypeWeapon].Effects[model.EffectSleep]; effect != nil {
		t.Errorf("Weapon's sleep effect should be deleted")
	}
}

func TestReactionPerform(t *testing.T) {
	source := model.NewDefaultPlayer(1, geometry.Point{X: 1, Y: 1}, 9)
	opponent := model.NewDefaultPlayer(2, geometry.Point{X: 1, Y: 1}, 9)

	sourceImpact := model.NewActorImpact(source)
	opponentImpact := model.NewActorImpact(opponent)

	reaction := &model.Reaction{
		Trigger: model.TriggerOnHit,
		Target:  model.TargetOpponent,
		VitalsChange: map[model.VitalType]model.Change{
			model.VitalHP: {
				Holder: model.TargetSource,
				Vital:  model.VitalUnknown,
				Attr:   model.AttrStrength,
				Amount: +5,
				Scale:  -1.0,
			},
		},
		StatusesChange: map[model.StatusType]int{
			model.StatusBlindness: 1,
		},
		EffectsToApply: map[model.EffectType]*model.Effect{
			model.EffectSleep: model.NewSleepEffect(2),
		},
	}

	damage := -source.DerivedAttrs[model.AttrStrength] + 5

	reaction.Perform(sourceImpact, opponentImpact)

	if opponentImpact.VitalsChange[model.VitalHP] != damage {
		t.Errorf("Expected %v VitalsChange, got %d", damage, opponentImpact.VitalsChange[model.VitalHP])
	}

	if opponentImpact.StatusesChange[model.StatusBlindness] != 1 {
		t.Errorf("Expected Blindness status to be 1")
	}

	appliedEff, exists := opponentImpact.AppliedEffects[model.EffectSleep]
	if !exists {
		t.Fatalf("Expected SleepEffect to be applied")
	}

	// Modify original effect to be sure that applied effect is cloned
	reaction.EffectsToApply[model.EffectSleep].Duration = 999
	if appliedEff.Duration == 999 {
		t.Errorf("Effect was not deep copied! Duration changed to 999")
	}
}

func TestEffectCloneVitalsChange(t *testing.T) {
	t.Parallel()

	original := &model.Effect{
		Kind: model.EffectFatigue,
		VitalsChange: map[model.VitalType]int{
			model.VitalHP: 50,
		},
	}

	clone := original.Clone()

	if clone.VitalsChange == nil || clone.VitalsChange[model.VitalHP] != 50 {
		t.Errorf("VitalsChange was not cloned properly")
	}

	original.VitalsChange[model.VitalHP] = 100
	if clone.VitalsChange[model.VitalHP] == 100 {
		t.Errorf("VitalsChange was not deep copied")
	}
}

func TestEffectGetBuffsAndDebuffs(t *testing.T) {
	t.Parallel()

	effect := &model.Effect{
		AttrsChange: map[model.AttrType]int{
			model.AttrStrength:  10, // Buff
			model.AttrDexterity: -5, // Debuff
			model.AttrMaxHP:     0,  // Neutral (should be ignored)
		},
	}

	buffs := effect.GetBuffs()
	debuffs := effect.GetDebuffs()

	if len(buffs) != 1 || buffs[model.AttrStrength] != 10 {
		t.Errorf("Expected buffs to contain only Strength: 10, got %v", buffs)
	}

	if len(debuffs) != 1 || debuffs[model.AttrDexterity] != -5 {
		t.Errorf("Expected debuffs to contain only Dexterity: -5, got %v", debuffs)
	}
}

func TestEffectModifiersAndPredicates(t *testing.T) {
	t.Parallel()

	effect := &model.Effect{
		Duration: 5,
		Charges:  2,
	}

	if !effect.IsTemp() || !effect.IsLimited() {
		t.Errorf("Expected effect to be temporary and limited")
	}

	effect.ExtendDuration(3)
	if effect.Duration != 8 {
		t.Errorf("Expected duration to be 8, got %d", effect.Duration)
	}

	effect.AddCharges(1)
	if effect.Charges != 3 {
		t.Errorf("Expected charges to be 3, got %d", effect.Charges)
	}

	effect.MakeInfinite()
	effect.MakeUnlimited()

	if effect.IsTemp() || effect.IsLimited() {
		t.Errorf("Expected effect to be infinite and unlimited")
	}

	effect.ExtendDuration(5)
	if effect.Duration != -1 {
		t.Errorf("Expected duration to remain -1, got %d", effect.Duration)
	}
}

func TestActorFastCheckers(t *testing.T) {
	t.Parallel()

	actor := &model.Actor{
		Statuses: make(map[model.StatusType]int),
	}

	if !actor.CanMove() || !actor.CanAttack() {
		t.Errorf("Clean actor should be able to move and attack")
	}

	actor.Statuses[model.StatusSleep] = 1
	if actor.CanMove() || actor.CanAttack() {
		t.Errorf("Sleeping actor should NOT be able to move or attack")
	}

	actor.Statuses[model.StatusSleep] = 0
	actor.Statuses[model.StatusFatigue] = 1
	if !actor.CanMove() {
		t.Errorf("Fatigued actor SHOULD be able to move")
	}
	if actor.CanAttack() {
		t.Errorf("Fatigued actor should NOT be able to attack")
	}
}
