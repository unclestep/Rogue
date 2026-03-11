// package entity_test
//
// import (
// 	"math/rand"
// 	"testing"
// 	"time"
//
// 	"github.com/unclestep/Rogue/internal/domain/model"
// 	"github.com/unclestep/Rogue/pkg/geometry"
// )
//
// func GenSeed() int64 {
// 	return time.Now().UnixNano()
// }
//
// func GetRng(seed int64) *rand.Rand {
// 	return rand.New(rand.NewSource(seed))
// }
//
// func TestCollectActiveEffects(t *testing.T) {
// 	actor := model.NewDefaultPlayer(1, geometry.NewDefaultPoint(), 9)
//
// 	t.Run("ActorWithoutGear", func(t *testing.T) {
// 		actor.Effects[model.EffectFatigue] = &model.Effect{Kind: model.EffectFatigue}
// 		actor.Effects[model.EffectSleep] = &model.Effect{Kind: model.EffectSleep}
//
// 		effects := actor.CollectActiveEffects()
//
// 		if len(effects) != 2 {
// 			t.Errorf("Expected 2 active effects, got %v\n", len(effects))
// 		}
//
// 		foundFatigue, foundSleep := false, false
//
// 		for _, effect := range effects {
// 			switch effect.Kind {
// 			case model.EffectFatigue:
// 				foundFatigue = true
// 			case model.EffectSleep:
// 				foundSleep = true
// 			default:
// 			}
// 		}
//
// 		if !foundFatigue || !foundSleep {
// 			t.Errorf("Expected find fatigue and sleep in active effects, got foundFatigue=%v, foundSleep=%v", foundFatigue, foundSleep)
// 		}
// 	})
//
// 	t.Run("ActorWithGear", func(t *testing.T) {
// 		actor.EquippedGear[model.ItemTypeWeapon] = &model.Item{
// 			Kind: model.ItemTypeWeapon,
// 			Effects: map[model.EffectType]*model.Effect{
// 				model.EffectInfallible: {
// 					Kind: model.EffectInfallible,
// 				},
// 			},
// 		}
//
// 		effects := actor.CollectActiveEffects()
//
// 		if len(effects) != 3 {
// 			t.Errorf("Expected 3 active effects, got %v\n", len(effects))
// 		}
//
// 		foundInfallible := false
//
// 		for _, effect := range effects {
// 			if effect.Kind == model.EffectInfallible {
// 				foundInfallible = true
// 				break
// 			}
// 		}
//
// 		if !foundInfallible {
// 			t.Errorf("Expected find infallible effect from gear in active effects, got foundInfallible=%v", foundInfallible)
// 		}
// 	})
// }
//
// func TestAddEffect(t *testing.T) {
// 	actor := model.NewDefaultPlayer(1, geometry.NewDefaultPoint(), 9)
//
// 	t.Run("Stack temp and limited effects", func(t *testing.T) {
// 		eff1 := &model.Effect{
// 			Kind:     model.EffectFatigue,
// 			Duration: 2,
// 			Charges:  3,
// 		}
// 		actor.AddEffect(eff1)
//
// 		eff2 := &model.Effect{
// 			Kind:     model.EffectFatigue,
// 			Duration: 3,
// 			Charges:  2,
// 		}
// 		actor.AddEffect(eff2)
//
// 		activeEff := actor.Effects[model.EffectFatigue]
// 		if activeEff.Duration != 5 {
// 			t.Errorf("Expected duration to sum to 5, got %d", activeEff.Duration)
// 		}
// 		if activeEff.Charges != 5 {
// 			t.Errorf("Expected charges to sum to 5, got %d", activeEff.Charges)
// 		}
// 	})
//
// 	t.Run("Temp becomes infinite, limited becomes unlimited", func(t *testing.T) {
// 		infiniteEff := &model.Effect{
// 			Kind:     model.EffectFatigue,
// 			Duration: -1,
// 			Charges:  -1,
// 		}
// 		actor.AddEffect(infiniteEff)
//
// 		activeEff := actor.Effects[model.EffectFatigue]
// 		if activeEff.Duration != -1 {
// 			t.Errorf("Expected effect to become infinite (-1), got %d", activeEff.Duration)
// 		}
// 		if activeEff.Charges != -1 {
// 			t.Errorf("Expected effect to become unlimited (-1), got %d", activeEff.Charges)
// 		}
// 	})
// }
//
// func TestRecomputeStats(t *testing.T) {
// 	actor := model.NewDefaultPlayer(1, geometry.NewDefaultPoint(), 9)
// 	baseStrength := actor.BaseAttrs[model.AttrStrength]
//
// 	t.Run("Apply effect stats and statuses", func(t *testing.T) {
// 		actor.Effects["Cool effect"] = &model.Effect{
// 			Kind: "Cool effect",
// 			AttrsChange: map[model.AttrType]int{
// 				model.AttrStrength: 10,
// 			},
// 			StatusesChange: map[model.StatusType]int{
// 				model.StatusSleep: 1,
// 			},
// 		}
//
// 		actor.RecomputeStats()
//
// 		expectedStr := baseStrength + 10
// 		if actor.DerivedAttrs[model.AttrStrength] != expectedStr {
// 			t.Errorf("Expected Strength to be %d, got %d", expectedStr, actor.DerivedAttrs[model.AttrStrength])
// 		}
//
// 		if actor.Statuses[model.StatusSleep] != 1 {
// 			t.Errorf("Expected Sleep status to be 1, got %d", actor.Statuses[model.StatusSleep])
// 		}
// 	})
//
// 	t.Run("Reset stats on effect removal", func(t *testing.T) {
// 		delete(actor.Effects, "Cool effect")
// 		actor.RecomputeStats()
//
// 		if actor.DerivedAttrs[model.AttrStrength] != baseStrength {
// 			t.Errorf("Expected Strength to reset to base %d, got %d", baseStrength, actor.DerivedAttrs[model.AttrStrength])
// 		}
//
// 		if actor.Statuses[model.StatusSleep] != 0 {
// 			t.Errorf("Expected Sleep status to reset to 0, got %d", actor.Statuses[model.StatusSleep])
// 		}
// 	})
//
// 	t.Run("Not effect based stat modification should not work", func(t *testing.T) {
// 		actor.Statuses[model.StatusSleep] = 1
//
// 		actor.RecomputeStats()
//
// 		if val := actor.Statuses[model.StatusSleep]; val != 0 {
// 			t.Errorf("Expected StatusSleep = 0, got %d", val)
// 		}
// 	})
// }
//
// func TestTickEffects(t *testing.T) {
// 	seed := GenSeed()
// 	rng := GetRng(seed)
//
// 	t.Run("TriggerOnTurn applies changes", func(t *testing.T) {
// 		actor := model.NewDefaultPlayer(1, geometry.Point{}, 9)
// 		startHP := actor.Vitals[model.VitalHP]
//
// 		poisonEffect := &model.Effect{
// 			Kind:     model.EffectUnknown,
// 			Duration: 2,
// 			Procs: map[model.TriggerType][]*model.Reaction{
// 				model.TriggerEffectOnTurn: {
// 					{
// 						Trigger: model.TriggerEffectOnTurn,
// 						Target:  model.TargetSource,
// 						Chance:  model.Guaranteed,
// 						VitalsChange: map[model.VitalType]model.Change{
// 							model.VitalHP: {
// 								Holder: model.TargetSource,
// 								Vital:  model.VitalHP,
// 								Amount: -5,
// 								Scale:  0,
// 							},
// 						},
// 					},
// 				},
// 			},
// 		}
// 		actor.AddEffect(poisonEffect)
//
// 		actor.TickEffects(rng)
//
// 		if actor.Vitals[model.VitalHP] != startHP-5 {
// 			t.Errorf("Seed: %v\nExpected HP to decrease by 5 (1st tick), got %d (start: %d)", seed, actor.Vitals[model.VitalHP], startHP)
// 		}
//
// 		effect, exists := actor.Effects[model.EffectUnknown]
// 		if !exists {
// 			t.Errorf("Effect not found in effect list")
// 		}
// 		if exists && effect.Duration != 1 {
// 			t.Errorf("Seed: %v\nExpected duration to decrease to 1, got %d", seed, effect.Duration)
// 		}
//
// 		actor.TickEffects(rng)
//
// 		if actor.Vitals[model.VitalHP] != startHP-10 {
// 			t.Errorf("Seed: %v\nExpected HP to decrease by 10 (2nd tick), got %d (start: %d)", seed, actor.Vitals[model.VitalHP], startHP)
// 		}
//
// 		if _, exists := actor.Effects[model.EffectUnknown]; exists {
// 			t.Errorf("Seed: %v\nExpected effect is expired, but exists. Got effect duration %v", seed, actor.Effects[model.EffectUnknown].Duration)
// 		}
// 	})
//
// 	t.Run("Effect expire and triggerOnExpire", func(t *testing.T) {
// 		actor := model.NewDefaultPlayer(1, geometry.Point{}, 9)
//
// 		fatigue := &model.Effect{
// 			Kind:     model.EffectUnknown,
// 			Duration: 1,
// 			Procs: map[model.TriggerType][]*model.Reaction{
// 				model.TriggerEffectOnExpire: {
// 					{
// 						Trigger: model.TriggerEffectOnExpire,
// 						Target:  model.TargetSource,
// 						Chance:  model.Guaranteed,
// 						EffectsToApply: map[model.EffectType]*model.Effect{
// 							model.EffectUntouchable: {
// 								Kind: model.EffectUntouchable,
// 								StatusesChange: map[model.StatusType]int{
// 									model.StatusUntouchable: 1,
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 		}
// 		actor.AddEffect(fatigue)
//
// 		actor.TickEffects(rng)
//
// 		if _, exists := actor.Effects[model.EffectUnknown]; exists {
// 			t.Errorf("Seed: %v\nExpected effect to be removed on expire", seed)
// 		}
//
// 		if _, exists := actor.Effects[model.EffectUntouchable]; !exists {
// 			t.Errorf("Seed: %v\nExpected to find untouchable effect, but exists returns %v", seed, exists)
// 		}
//
// 		if actor.Statuses[model.StatusUntouchable] != 1 {
// 			t.Errorf("Seed: %v\nExpected OnExpire reaction to set Untouchable status to 1, got %d", seed, actor.Statuses[model.StatusUntouchable])
// 		}
// 	})
//
// 	t.Run("Charges should decrement", func(t *testing.T) {
// 		actor := model.NewDefaultPlayer(1, geometry.Point{}, 9)
//
// 		fatigue := &model.Effect{
// 			Kind:      model.EffectUnknown,
// 			Duration:  2,
// 			Charges:   1,
// 			ConsumeOn: model.TriggerEffectOnTurn,
// 			Procs: map[model.TriggerType][]*model.Reaction{
// 				model.TriggerEffectOnExpire: {
// 					{
// 						Trigger: model.TriggerEffectOnExpire,
// 						Target:  model.TargetSource,
// 						Chance:  model.Guaranteed,
// 						EffectsToApply: map[model.EffectType]*model.Effect{
// 							model.EffectUntouchable: {
// 								Kind: model.EffectUntouchable,
// 								StatusesChange: map[model.StatusType]int{
// 									model.StatusUntouchable: 1,
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 		}
// 		actor.AddEffect(fatigue)
//
// 		actor.TickEffects(rng)
//
// 		if _, exists := actor.Effects[model.EffectUnknown]; exists {
// 			t.Errorf("Seed: %v\nExpected effect to be removed on expire", seed)
// 		}
//
// 		if _, exists := actor.Effects[model.EffectUntouchable]; !exists {
// 			t.Errorf("Seed: %v\nExpected to find untouchable effect, but exists returns %v", seed, exists)
// 		}
//
// 		if actor.Statuses[model.StatusUntouchable] != 1 {
// 			t.Errorf("Seed: %v\nExpected OnExpire reaction to set Untouchable status to 1, got %d", seed, actor.Statuses[model.StatusUntouchable])
// 		}
// 	})
// }
//
// func TestActorImpactApply(t *testing.T) {
// 	seed := GenSeed()
// 	rng := GetRng(seed)
//
// 	t.Run("Apply vitals, baseAttrs and statuses", func(t *testing.T) {
// 		actor := model.NewDefaultPlayer(1, geometry.Point{}, 9)
// 		impact := model.NewActorImpact(actor)
//
// 		impact.VitalsChange[model.VitalHP] = -15
// 		impact.BaseAttrsChange[model.AttrMaxHP] = 10
// 		impact.StatusesChange[model.StatusBlindness] = 1
// 		impact.PosChange = geometry.Point{X: 1, Y: 1}
//
// 		startHP := actor.Vitals[model.VitalHP]
// 		startMaxHP := actor.BaseAttrs[model.AttrMaxHP]
// 		startPos := actor.Pos
//
// 		impact.Apply(rng)
//
// 		if actor.Vitals[model.VitalHP] != startHP-15 {
// 			t.Errorf("Seed: %v\nHP was not correctly applied", seed)
// 		}
// 		if actor.BaseAttrs[model.AttrMaxHP] != startMaxHP+10 {
// 			t.Errorf("Seed: %v\nBase MaxHealth was not correctly applied", seed)
// 		}
// 		if actor.Statuses[model.StatusBlindness] != 1 {
// 			t.Errorf("Seed: %v\nStatus Blindness was not correctly applied", seed)
// 		}
// 		if actor.Pos != startPos.Add(geometry.Point{X: 1, Y: 1}) {
// 			t.Errorf("Seed: %v\nPosition change was not correctly applied", seed)
// 		}
// 	})
//
// 	t.Run("Remove effect on zero charges and run onExpire", func(t *testing.T) {
// 		actor := model.NewDefaultPlayer(1, geometry.Point{}, 9)
//
// 		eff := &model.Effect{
// 			Kind:    model.EffectUntouchable,
// 			Charges: 1,
// 			Procs: map[model.TriggerType][]*model.Reaction{
// 				model.TriggerEffectOnExpire: {
// 					{
// 						Trigger: model.TriggerEffectOnExpire,
// 						Target:  model.TargetSource,
// 						Chance:  model.Guaranteed,
// 						BaseAttrsChange: map[model.AttrType]model.Change{
// 							model.AttrStrength: {
// 								Holder: model.TargetSource,
// 								Attr:   model.AttrStrength,
// 								Amount: 5,
// 							},
// 						},
// 					},
// 				},
// 			},
// 		}
// 		actor.AddEffect(eff)
// 		startStr := actor.BaseAttrs[model.AttrStrength]
//
// 		impact := model.NewActorImpact(actor)
// 		impact.EffectChargesChange[eff] = -1
//
// 		impact.Apply(rng)
//
// 		if _, exists := actor.Effects[model.EffectUntouchable]; exists {
// 			t.Errorf("Expected effect to be removed because charges reached 0")
// 		}
//
// 		if actor.BaseAttrs[model.AttrStrength] != startStr+5 {
// 			t.Errorf("Expected OnExpire reaction to increase Base Strength by 5, got %d", actor.BaseAttrs[model.AttrStrength])
// 		}
// 	})
// }
