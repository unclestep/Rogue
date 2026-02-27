package entities

import (
	"math/rand"
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/domain/entity"
	"github.com/unclestep/Rogue/internal/pkg/geometry"
)

func GenSeed() int64 {
	return time.Now().UnixNano()
}

func GetRng(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}

func TestCollectActiveEffects(t *testing.T) {
	actor := entity.NewDefaultPlayer(1, geometry.NewDefaultPoint())

	t.Run("ActorWithoutGear", func(t *testing.T) {
		actor.Effects[entity.FatigueEffect] = &entity.Effect{Kind: entity.FatigueEffect}
		actor.Effects[entity.SleepEffect] = &entity.Effect{Kind: entity.SleepEffect}

		effects := actor.CollectActiveEffects()

		if len(effects) != 2 {
			t.Errorf("Expected 2 active effects, got %v\n", len(effects))
		}

		foundFatigue, foundSleep := false, false

		for _, effect := range effects {
			if effect.Kind == entity.FatigueEffect {
				foundFatigue = true
			} else if effect.Kind == entity.SleepEffect {
				foundSleep = true
			}
		}

		if !foundFatigue || !foundSleep {
			t.Errorf("Expected find fatigue and sleep in active effects, got foundFatigue=%v, foundSleep=%v", foundFatigue, foundSleep)
		}
	})

	t.Run("ActorWithGear", func(t *testing.T) {
		actor.EquippedGear[entity.ItemTypeWeapon] = &entity.Item{
			Kind: entity.ItemTypeWeapon,
			Effects: map[entity.EffectType]*entity.Effect{
				entity.InfallibleEffect: {
					Kind: entity.InfallibleEffect,
				},
			},
		}

		effects := actor.CollectActiveEffects()

		if len(effects) != 3 {
			t.Errorf("Expected 3 active effects, got %v\n", len(effects))
		}

		foundInfallible := false

		for _, effect := range effects {
			if effect.Kind == entity.InfallibleEffect {
				foundInfallible = true
				break
			}
		}

		if !foundInfallible {
			t.Errorf("Expected find infallible effect from gear in active effects, got foundInfallible=%v", foundInfallible)
		}
	})
}

func TestAddEffect(t *testing.T) {
	actor := entity.NewDefaultPlayer(1, geometry.NewDefaultPoint())

	t.Run("Stack temp and limited effects", func(t *testing.T) {
		eff1 := &entity.Effect{
			Kind:     entity.FatigueEffect,
			Duration: 2,
			Charges:  3,
		}
		actor.AddEffect(eff1)

		eff2 := &entity.Effect{
			Kind:     entity.FatigueEffect,
			Duration: 3,
			Charges:  2,
		}
		actor.AddEffect(eff2)

		activeEff := actor.Effects[entity.FatigueEffect]
		if activeEff.Duration != 5 {
			t.Errorf("Expected duration to sum to 5, got %d", activeEff.Duration)
		}
		if activeEff.Charges != 5 {
			t.Errorf("Expected charges to sum to 5, got %d", activeEff.Charges)
		}
	})

	t.Run("Temp becomes infinite, limited becomes unlimited", func(t *testing.T) {
		infiniteEff := &entity.Effect{
			Kind:     entity.FatigueEffect,
			Duration: -1,
			Charges:  -1,
		}
		actor.AddEffect(infiniteEff)

		activeEff := actor.Effects[entity.FatigueEffect]
		if activeEff.Duration != -1 {
			t.Errorf("Expected effect to become infinite (-1), got %d", activeEff.Duration)
		}
		if activeEff.Charges != -1 {
			t.Errorf("Expected effect to become unlimited (-1), got %d", activeEff.Charges)
		}
	})
}

func TestRecomputeStats(t *testing.T) {
	actor := entity.NewDefaultPlayer(1, geometry.NewDefaultPoint())
	baseStrength := actor.BaseAttrs[entity.Strength]

	t.Run("Apply effect stats and statuses", func(t *testing.T) {
		actor.Effects[entity.StrengthBoostElixir] = &entity.Effect{
			Kind: entity.StrengthBoostElixir,
			AttrsChange: map[entity.AttrType]int{
				entity.Strength: 10,
			},
			StatusesChange: map[entity.StatusType]int{
				entity.StatusSleep: 1,
			},
		}

		actor.RecomputeStats()

		expectedStr := baseStrength + 10
		if actor.DerivedAttrs[entity.Strength] != expectedStr {
			t.Errorf("Expected Strength to be %d, got %d", expectedStr, actor.DerivedAttrs[entity.Strength])
		}

		if actor.Statuses[entity.StatusSleep] != 1 {
			t.Errorf("Expected Sleep status to be 1, got %d", actor.Statuses[entity.StatusSleep])
		}
	})

	t.Run("Reset stats on effect removal", func(t *testing.T) {
		delete(actor.Effects, entity.StrengthBoostElixir)
		actor.RecomputeStats()

		if actor.DerivedAttrs[entity.Strength] != baseStrength {
			t.Errorf("Expected Strength to reset to base %d, got %d", baseStrength, actor.DerivedAttrs[entity.Strength])
		}

		if actor.Statuses[entity.StatusSleep] != 0 {
			t.Errorf("Expected Sleep status to reset to 0, got %d", actor.Statuses[entity.StatusSleep])
		}
	})

	t.Run("Not effect based stat modification should not work", func(t *testing.T) {
		actor.Statuses[entity.StatusSleep] = 1

		actor.RecomputeStats()

		if val := actor.Statuses[entity.StatusSleep]; val != 0 {
			t.Errorf("Expected StatusSleep = 0, got %d", val)
		}
	})
}

func TestTickEffects(t *testing.T) {
	seed := GenSeed()
	rng := GetRng(seed)

	t.Run("TriggerOnTurn applies changes", func(t *testing.T) {
		actor := entity.NewDefaultPlayer(1, geometry.Point{})
		startHP := actor.Vitals[entity.HP]

		poisonEffect := &entity.Effect{
			Kind:     entity.EffectNotSpecified,
			Duration: 2,
			Procs: map[entity.TriggerType][]*entity.Reaction{
				entity.TriggerEffectOnTurn: {
					{
						Kind:   entity.TriggerEffectOnTurn,
						Target: entity.TargetSource,
						Chance: entity.Guaranteed,
						VitalsChange: map[entity.VitalType]entity.Change{
							entity.HP: {
								Holder: entity.TargetSource,
								Vital:  entity.HP,
								Amount: -5,
								Scale:  0,
							},
						},
					},
				},
			},
		}
		actor.AddEffect(poisonEffect)

		actor.TickEffects(rng)

		if actor.Vitals[entity.HP] != startHP-5 {
			t.Errorf("Seed: %v\nExpected HP to decrease by 5 (1st tick), got %d (start: %d)", seed, actor.Vitals[entity.HP], startHP)
		}

		effect, exists := actor.Effects[entity.EffectNotSpecified]
		if !exists {
			t.Errorf("Effect not found in effect list")
		}
		if exists && effect.Duration != 1 {
			t.Errorf("Seed: %v\nExpected duration to decrease to 1, got %d", seed, effect.Duration)
		}

		actor.TickEffects(rng)

		if actor.Vitals[entity.HP] != startHP-10 {
			t.Errorf("Seed: %v\nExpected HP to decrease by 10 (2nd tick), got %d (start: %d)", seed, actor.Vitals[entity.HP], startHP)
		}

		if _, exists := actor.Effects[entity.EffectNotSpecified]; exists {
			t.Errorf("Seed: %v\nExpected effect is expired, but exists. Got effect duration %v", seed, actor.Effects[entity.EffectNotSpecified].Duration)
		}
	})

	t.Run("Effect expire and triggerOnExpire", func(t *testing.T) {
		actor := entity.NewDefaultPlayer(1, geometry.Point{})

		fatigue := &entity.Effect{
			Kind:     entity.EffectNotSpecified,
			Duration: 1,
			Procs: map[entity.TriggerType][]*entity.Reaction{
				entity.TriggerEffectOnExpire: {
					{
						Kind:   entity.TriggerEffectOnExpire,
						Target: entity.TargetSource,
						Chance: entity.Guaranteed,
						EffectsToApply: map[entity.EffectType]*entity.Effect{
							entity.UntouchableEffect: {
								Kind: entity.UntouchableEffect,
								StatusesChange: map[entity.StatusType]int{
									entity.StatusUntouchable: 1,
								},
							},
						},
					},
				},
			},
		}
		actor.AddEffect(fatigue)

		actor.TickEffects(rng)

		if _, exists := actor.Effects[entity.EffectNotSpecified]; exists {
			t.Errorf("Seed: %v\nExpected effect to be removed on expire", seed)
		}

		if _, exists := actor.Effects[entity.UntouchableEffect]; !exists {
			t.Errorf("Seed: %v\nExpected to find untouchable effect, but exists returns %v", seed, exists)
		}

		if actor.Statuses[entity.StatusUntouchable] != 1 {
			t.Errorf("Seed: %v\nExpected OnExpire reaction to set Untouchable status to 1, got %d", seed, actor.Statuses[entity.StatusUntouchable])
		}
	})

	t.Run("Charges should decrement", func(t *testing.T) {
		actor := entity.NewDefaultPlayer(1, geometry.Point{})

		fatigue := &entity.Effect{
			Kind:      entity.EffectNotSpecified,
			Duration:  2,
			Charges:   1,
			ConsumeOn: entity.TriggerEffectOnTurn,
			Procs: map[entity.TriggerType][]*entity.Reaction{
				entity.TriggerEffectOnExpire: {
					{
						Kind:   entity.TriggerEffectOnExpire,
						Target: entity.TargetSource,
						Chance: entity.Guaranteed,
						EffectsToApply: map[entity.EffectType]*entity.Effect{
							entity.UntouchableEffect: {
								Kind: entity.UntouchableEffect,
								StatusesChange: map[entity.StatusType]int{
									entity.StatusUntouchable: 1,
								},
							},
						},
					},
				},
			},
		}
		actor.AddEffect(fatigue)

		actor.TickEffects(rng)

		if _, exists := actor.Effects[entity.EffectNotSpecified]; exists {
			t.Errorf("Seed: %v\nExpected effect to be removed on expire", seed)
		}

		if _, exists := actor.Effects[entity.UntouchableEffect]; !exists {
			t.Errorf("Seed: %v\nExpected to find untouchable effect, but exists returns %v", seed, exists)
		}

		if actor.Statuses[entity.StatusUntouchable] != 1 {
			t.Errorf("Seed: %v\nExpected OnExpire reaction to set Untouchable status to 1, got %d", seed, actor.Statuses[entity.StatusUntouchable])
		}
	})
}

func TestActorImpactApply(t *testing.T) {
	seed := GenSeed()
	rng := GetRng(seed)

	t.Run("Apply vitals, baseAttrs and statuses", func(t *testing.T) {
		actor := entity.NewDefaultPlayer(1, geometry.Point{})
		impact := entity.NewActorImpact(actor)

		impact.VitalsChange[entity.HP] = -15
		impact.BaseAttrsChange[entity.MaxHealth] = 10
		impact.StatusesChange[entity.StatusBlindness] = 1
		impact.PosChange = geometry.Point{X: 1, Y: 1}

		startHP := actor.Vitals[entity.HP]
		startMaxHP := actor.BaseAttrs[entity.MaxHealth]
		startPos := actor.Pos

		impact.Apply(rng)

		if actor.Vitals[entity.HP] != startHP-15 {
			t.Errorf("Seed: %v\nHP was not correctly applied", seed)
		}
		if actor.BaseAttrs[entity.MaxHealth] != startMaxHP+10 {
			t.Errorf("Seed: %v\nBase MaxHealth was not correctly applied", seed)
		}
		if actor.Statuses[entity.StatusBlindness] != 1 {
			t.Errorf("Seed: %v\nStatus Blindness was not correctly applied", seed)
		}
		if actor.Pos != startPos.Add(geometry.Point{X: 1, Y: 1}) {
			t.Errorf("Seed: %v\nPosition change was not correctly applied", seed)
		}
	})

	t.Run("Remove effect on zero charges and run onExpire", func(t *testing.T) {
		actor := entity.NewDefaultPlayer(1, geometry.Point{})

		eff := &entity.Effect{
			Kind:    entity.UntouchableEffect,
			Charges: 1,
			Procs: map[entity.TriggerType][]*entity.Reaction{
				entity.TriggerEffectOnExpire: {
					{
						Kind:   entity.TriggerEffectOnExpire,
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
				},
			},
		}
		actor.AddEffect(eff)
		startStr := actor.BaseAttrs[entity.Strength]

		impact := entity.NewActorImpact(actor)
		impact.EffectChargesChange[eff] = -1

		impact.Apply(rng)

		if _, exists := actor.Effects[entity.UntouchableEffect]; exists {
			t.Errorf("Expected effect to be removed because charges reached 0")
		}

		if actor.BaseAttrs[entity.Strength] != startStr+5 {
			t.Errorf("Expected OnExpire reaction to increase Base Strength by 5, got %d", actor.BaseAttrs[entity.Strength])
		}
	})
}
