package model_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func TestCollectActiveEffects(t *testing.T) {
	actor := model.NewDefaultPlayer(1, geometry.NewDefaultPoint(), 9)

	t.Run("ActorWithoutGear", func(t *testing.T) {
		actor.Effects[model.EffectFatigue] = &model.Effect{Kind: model.EffectFatigue}
		actor.Effects[model.EffectSleep] = &model.Effect{Kind: model.EffectSleep}

		effects := actor.CollectActiveEffects()

		if len(effects) != 2 {
			t.Errorf("Expected 2 active effects, got %v\n", len(effects))
		}

		foundFatigue, foundSleep := false, false

		for _, effect := range effects {
			switch effect.Kind {
			case model.EffectFatigue:
				foundFatigue = true
			case model.EffectSleep:
				foundSleep = true
			default:
			}
		}

		if !foundFatigue || !foundSleep {
			t.Errorf("Expected find fatigue and sleep in active effects, got foundFatigue=%v, foundSleep=%v", foundFatigue, foundSleep)
		}
	})

	t.Run("ActorWithGear", func(t *testing.T) {
		actor.EquippedGear[model.ItemTypeWeapon] = &model.Item{
			Kind: model.ItemTypeWeapon,
			Effects: map[model.EffectType]*model.Effect{
				model.EffectInfallible: {
					Kind: model.EffectInfallible,
				},
			},
		}

		effects := actor.CollectActiveEffects()

		if len(effects) != 3 {
			t.Errorf("Expected 3 active effects, got %v\n", len(effects))
		}

		foundInfallible := false

		for _, effect := range effects {
			if effect.Kind == model.EffectInfallible {
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
	actor := model.NewDefaultPlayer(1, geometry.NewDefaultPoint(), 9)

	t.Run("Stack temp and limited effects", func(t *testing.T) {
		eff1 := &model.Effect{
			Kind:     model.EffectFatigue,
			Duration: 2,
			Charges:  3,
		}
		actor.AddEffect(eff1)

		eff2 := &model.Effect{
			Kind:     model.EffectFatigue,
			Duration: 3,
			Charges:  2,
		}
		actor.AddEffect(eff2)

		activeEff := actor.Effects[model.EffectFatigue]
		if activeEff.Duration != 5 {
			t.Errorf("Expected duration to sum to 5, got %d", activeEff.Duration)
		}
		if activeEff.Charges != 5 {
			t.Errorf("Expected charges to sum to 5, got %d", activeEff.Charges)
		}
	})

	t.Run("Temp becomes infinite, limited becomes unlimited", func(t *testing.T) {
		infiniteEff := &model.Effect{
			Kind:     model.EffectFatigue,
			Duration: -1,
			Charges:  -1,
		}
		actor.AddEffect(infiniteEff)

		activeEff := actor.Effects[model.EffectFatigue]
		if activeEff.Duration != -1 {
			t.Errorf("Expected effect to become infinite (-1), got %d", activeEff.Duration)
		}
		if activeEff.Charges != -1 {
			t.Errorf("Expected effect to become unlimited (-1), got %d", activeEff.Charges)
		}
	})
}

func TestRecomputeStats(t *testing.T) {
	actor := model.NewDefaultPlayer(1, geometry.NewDefaultPoint(), 9)
	baseStrength := actor.BaseAttrs[model.AttrStrength]

	t.Run("Apply effect stats and statuses", func(t *testing.T) {
		actor.Effects["Cool effect"] = &model.Effect{
			Kind: "Cool effect",
			AttrsChange: map[model.AttrType]int{
				model.AttrStrength: 10,
			},
			StatusesChange: map[model.StatusType]int{
				model.StatusSleep: 1,
			},
		}

		actor.RecomputeStats()

		expectedStr := baseStrength + 10
		if actor.DerivedAttrs[model.AttrStrength] != expectedStr {
			t.Errorf("Expected Strength to be %d, got %d", expectedStr, actor.DerivedAttrs[model.AttrStrength])
		}

		if actor.Statuses[model.StatusSleep] != 1 {
			t.Errorf("Expected Sleep status to be 1, got %d", actor.Statuses[model.StatusSleep])
		}
	})

	t.Run("Reset stats on effect removal", func(t *testing.T) {
		delete(actor.Effects, "Cool effect")
		actor.RecomputeStats()

		if actor.DerivedAttrs[model.AttrStrength] != baseStrength {
			t.Errorf("Expected Strength to reset to base %d, got %d", baseStrength, actor.DerivedAttrs[model.AttrStrength])
		}

		if actor.Statuses[model.StatusSleep] != 0 {
			t.Errorf("Expected Sleep status to reset to 0, got %d", actor.Statuses[model.StatusSleep])
		}
	})

	t.Run("Not effect based stat modification should not work", func(t *testing.T) {
		actor.Statuses[model.StatusSleep] = 1

		actor.RecomputeStats()

		if val := actor.Statuses[model.StatusSleep]; val != 0 {
			t.Errorf("Expected StatusSleep = 0, got %d", val)
		}
	})
}

func TestActorClone(t *testing.T) {
	original := &model.Actor{
		Id:   1,
		Kind: model.ActorPlayer,
		Pos:  geometry.Point{X: 10, Y: 20},
		Vitals: map[model.VitalType]int{
			model.VitalHP: 100,
		},
		BaseAttrs: map[model.AttrType]int{
			model.AttrStrength: 15,
		},
		DerivedAttrs: map[model.AttrType]int{
			model.AttrStrength: 15,
		},
		Statuses: make(map[model.StatusType]int),
	}

	cloned := original.Clone()

	if cloned == original {
		t.Errorf("Expected cloned actor to be a different pointer")
	}

	if cloned.Id != original.Id || cloned.Kind != original.Kind || cloned.Pos != original.Pos {
		t.Errorf("Expected scalar fields to be copied identically")
	}

	// Modify cloned maps to ensure they don't affect the original (deep copy check)
	cloned.Vitals[model.VitalHP] = 50
	if original.Vitals[model.VitalHP] == 50 {
		t.Errorf("Expected deep copy of Vitals, changing clone altered original")
	}

	cloned.BaseAttrs[model.AttrStrength] = 99
	if original.BaseAttrs[model.AttrStrength] == 99 {
		t.Errorf("Expected deep copy of BaseAttrs, changing clone altered original")
	}
}

func TestActorCloneNilActor(t *testing.T) {
	var original *model.Actor = nil
	cloned := original.Clone()

	if cloned != nil {
		t.Errorf("Expected nil when cloning a nil actor, got %v", cloned)
	}
}
