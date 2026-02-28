package entity_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/entity"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func TestEffectClone(t *testing.T) {
	original := &entity.Effect{
		Kind:     entity.FatigueEffect,
		Duration: 5,
		Charges:  -1,
		AttrsChange: map[entity.AttrType]int{
			entity.Strength: -5,
		},
		Procs: map[entity.TriggerType][]*entity.Reaction{
			entity.TriggerEffectOnExpire: {
				{
					Target: entity.TargetSource,
					Chance: entity.Guaranteed,
					EffectsToApply: map[entity.EffectType]*entity.Effect{
						entity.UntouchableEffect: {
							Kind:     entity.UntouchableEffect,
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

	original.AttrsChange[entity.Strength] = -10
	if clone.AttrsChange[entity.Strength] == -10 {
		t.Errorf("AttrsChange was not deep copied")
	}

	originalReact := original.Procs[entity.TriggerEffectOnExpire][0]
	cloneReact := clone.Procs[entity.TriggerEffectOnExpire][0]

	originalReact.Chance = 50
	if cloneReact.Chance == 50 {
		t.Errorf("Reaction inside Procs was not deep copied")
	}

	originalNestedEff := originalReact.EffectsToApply[entity.UntouchableEffect]
	cloneNestedEff := cloneReact.EffectsToApply[entity.UntouchableEffect]

	originalNestedEff.Duration = 99
	if cloneNestedEff.Duration == 99 {
		t.Errorf("Nested effect inside Reaction was not deep copied")
	}
}

func TestChangeCalc(t *testing.T) {
	source := entity.NewDefaultPlayer(1, geometry.Point{X: 1, Y: 1})
	opponent := entity.NewDefaultPlayer(2, geometry.Point{X: 1, Y: 1})

	t.Run("Flat amount", func(t *testing.T) {
		change := &entity.Change{
			Holder: entity.TargetSource,
			Vital:  entity.NoVital,
			Attr:   entity.Strength,
			Amount: 15,
			Scale:  0.0,
		}
		res := change.Calc(source, opponent)
		if res != 15 {
			t.Errorf("Expected 15, got %d", res)
		}
	})

	t.Run("Scale from source attribute", func(t *testing.T) {
		startStrength := source.DerivedAttrs[entity.Strength]
		change := &entity.Change{
			Holder: entity.TargetSource,
			Vital:  entity.NoVital,
			Attr:   entity.Strength,
			Amount: 5,
			Scale:  1,
		}
		res := change.Calc(source, opponent)
		if res != startStrength+5 {
			t.Errorf("Expected %v, got %d", startStrength+5, res)
		}
	})

	t.Run("Scale from opponent vitals", func(t *testing.T) {
		hp := opponent.Vitals[entity.HP]
		change := &entity.Change{
			Holder: entity.TargetOpponent,
			Vital:  entity.HP,
			Attr:   entity.NoAttr,
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
		change := &entity.Change{
			Holder: entity.TargetNotSpecified,
			Vital:  entity.HP,
			Amount: 10,
			Scale:  1.0,
		}
		res := change.Calc(source, opponent)
		if res != 0 {
			t.Errorf("Expected 0 due to invalid holder, got %d", res)
		}
	})
}

func TestReactionPerform(t *testing.T) {
	source := entity.NewDefaultPlayer(1, geometry.Point{X: 1, Y: 1})
	opponent := entity.NewDefaultPlayer(2, geometry.Point{X: 1, Y: 1})

	sourceImpact := entity.NewActorImpact(source)
	opponentImpact := entity.NewActorImpact(opponent)

	reaction := &entity.Reaction{
		Kind:   entity.TriggerOnHit,
		Target: entity.TargetOpponent,
		VitalsChange: map[entity.VitalType]entity.Change{
			entity.HP: {
				Holder: entity.TargetSource,
				Vital:  entity.NoVital,
				Attr:   entity.Strength,
				Amount: +5,
				Scale:  -1.0,
			},
		},
		StatusesChange: map[entity.StatusType]int{
			entity.StatusBlindness: 1,
		},
		EffectsToApply: map[entity.EffectType]*entity.Effect{
			entity.SleepEffect: entity.NewSleepEffect(2),
		},
	}

	damage := -source.DerivedAttrs[entity.Strength] + 5

	reaction.Perform(sourceImpact, opponentImpact)

	if opponentImpact.VitalsChange[entity.HP] != damage {
		t.Errorf("Expected %v VitalsChange, got %d", damage, opponentImpact.VitalsChange[entity.HP])
	}

	if opponentImpact.StatusesChange[entity.StatusBlindness] != 1 {
		t.Errorf("Expected Blindness status to be 1")
	}

	appliedEff, exists := opponentImpact.AppliedEffects[entity.SleepEffect]
	if !exists {
		t.Fatalf("Expected SleepEffect to be applied")
	}

	// Modify original effect to be sure that applied effect is cloned
	reaction.EffectsToApply[entity.SleepEffect].Duration = 999
	if appliedEff.Duration == 999 {
		t.Errorf("Effect was not deep copied! Duration changed to 999")
	}
}

func TestResolveSpecificReactions(t *testing.T) {
	seed := GenSeed()
	rng := GetRng(seed)

	source := entity.NewDefaultPlayer(1, geometry.Point{X: 1, Y: 1})
	opponent := entity.NewDefaultPlayer(2, geometry.Point{X: 1, Y: 1})

	sourceImpact := entity.NewActorImpact(source)
	opponentImpact := entity.NewActorImpact(opponent)

	t.Run("Chance logic (Guaranteed vs 0%)", func(t *testing.T) {
		reactions := []*entity.Reaction{
			{
				Target: entity.TargetSource,
				Chance: 0,
				VitalsChange: map[entity.VitalType]entity.Change{
					entity.HP: {
						Holder: entity.TargetSource,
						Attr:   entity.Strength,
						Amount: 10,
					},
				},
			},
			{
				Target: entity.TargetSource,
				Chance: entity.Guaranteed,
				BaseAttrsChange: map[entity.AttrType]entity.Change{
					entity.Strength: {
						Holder: entity.TargetSource,
						Attr:   entity.Strength,
						Amount: 5,
					},
				},
			},
		}

		entity.ResolveSpecificReactions(reactions, sourceImpact, opponentImpact, rng)

		if sourceImpact.VitalsChange[entity.HP] != 0 {
			t.Errorf("Reaction with 0%% chance should not trigger")
		}

		if res := sourceImpact.BaseAttrsChange[entity.Strength]; res != 5 {
			t.Errorf("Reaction with Guaranteed chance should trigger: expected %v, got %v", 5, res)
		}
	})

	t.Run("Target routing (Source vs Opponent)", func(t *testing.T) {
		sourceImpact = entity.NewActorImpact(source)
		opponentImpact = entity.NewActorImpact(opponent)

		reactions := []*entity.Reaction{
			{
				Target:         entity.TargetSource,
				Chance:         entity.Guaranteed,
				StatusesChange: map[entity.StatusType]int{entity.StatusInfallible: 1},
			},
			{
				Target:         entity.TargetOpponent,
				Chance:         entity.Guaranteed,
				StatusesChange: map[entity.StatusType]int{entity.StatusBlindness: 1},
			},
		}

		entity.ResolveSpecificReactions(reactions, sourceImpact, opponentImpact, rng)

		if sourceImpact.StatusesChange[entity.StatusInfallible] != 1 {
			t.Errorf("Source should have received Infallible status")
		}
		if opponentImpact.StatusesChange[entity.StatusBlindness] != 1 {
			t.Errorf("Opponent should have received Blindness status")
		}
		if sourceImpact.StatusesChange[entity.StatusBlindness] == 1 {
			t.Errorf("Source should NOT have received Blindness status")
		}
	})
}

func TestDecrementEffectsCharges(t *testing.T) {
	actor := entity.NewDefaultPlayer(1, geometry.Point{X: 0, Y: 0})
	impact := entity.NewActorImpact(actor)

	dexBoost := &entity.Effect{
		Kind:      entity.DexterityBoostElixir,
		Charges:   1,
		ConsumeOn: entity.TriggerNotSpecified,
	}

	fatigue := &entity.Effect{
		Kind:      entity.FatigueEffect,
		Charges:   1,
		ConsumeOn: entity.TriggerNotSpecified,
	}

	sleep := &entity.Effect{
		Kind:      entity.SleepEffect,
		Charges:   2,
		ConsumeOn: entity.TriggerNotSpecified,
	}

	actor.Effects[entity.DexterityBoostElixir] = dexBoost
	actor.Effects[entity.FatigueEffect] = fatigue
	actor.EquippedGear[entity.ItemTypeWeapon] = &entity.Item{
		Kind: entity.ItemTypeWeapon,
		Effects: map[entity.EffectType]*entity.Effect{
			entity.SleepEffect: sleep,
		},
	}

	impact.DecrementAllRelatedCharges(entity.TriggerNotSpecified)

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

	impact.DecrementAllRelatedCharges(entity.TriggerNotSpecified)

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

	if effect := actor.Effects[entity.DexterityBoostElixir]; effect != nil {
		t.Errorf("Dexterity boost elixir should be deleted")
	}

	if effect := actor.Effects[entity.FatigueEffect]; effect != nil {
		t.Errorf("Fatigue should be deleted")
	}

	if effect := actor.EquippedGear[entity.ItemTypeWeapon].Effects[entity.SleepEffect]; effect != nil {
		t.Errorf("Weapon's sleep effect should be deleted")
	}
}
