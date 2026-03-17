package service_test

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
// func TestResolveSpecificReactions(t *testing.T) {
// 	source := model.NewDefaultPlayer(1, geometry.Point{X: 1, Y: 1}, 9)
// 	opponent := model.NewDefaultPlayer(2, geometry.Point{X: 1, Y: 1}, 9)
//
// 	sourceImpact := model.NewActorImpact(source)
// 	opponentImpact := model.NewActorImpact(opponent)
//
// 	t.Run("Chance logic (Guaranteed vs 0%)", func(t *testing.T) {
// 		reactions := []*model.Reaction{
// 			{
// 				Target: model.TargetSource,
// 				Chance: 0,
// 				VitalsChange: map[model.VitalType]model.Change{
// 					model.VitalHP: {
// 						Holder: model.TargetSource,
// 						Attr:   model.AttrStrength,
// 						Amount: 10,
// 					},
// 				},
// 			},
// 			{
// 				Target: model.TargetSource,
// 				Chance: model.Guaranteed,
// 				BaseAttrsChange: map[model.AttrType]model.Change{
// 					model.AttrStrength: {
// 						Holder: model.TargetSource,
// 						Attr:   model.AttrStrength,
// 						Amount: 5,
// 					},
// 				},
// 			},
// 		}
//
// 		model.ResolveSpecificReactions(reactions, sourceImpact, opponentImpact, rng)
//
// 		if sourceImpact.VitalsChange[model.VitalHP] != 0 {
// 			t.Errorf("Reaction with 0%% chance should not trigger")
// 		}
//
// 		if res := sourceImpact.BaseAttrsChange[model.AttrStrength]; res != 5 {
// 			t.Errorf("Reaction with Guaranteed chance should trigger: expected %v, got %v", 5, res)
// 		}
// 	})
//
// 	t.Run("Target routing (Source vs Opponent)", func(t *testing.T) {
// 		sourceImpact = model.NewActorImpact(source)
// 		opponentImpact = model.NewActorImpact(opponent)
//
// 		reactions := []*model.Reaction{
// 			{
// 				Target:         model.TargetSource,
// 				Chance:         model.Guaranteed,
// 				StatusesChange: map[model.StatusType]int{model.StatusInfallible: 1},
// 			},
// 			{
// 				Target:         model.TargetOpponent,
// 				Chance:         model.Guaranteed,
// 				StatusesChange: map[model.StatusType]int{model.StatusBlindness: 1},
// 			},
// 		}
//
// 		model.ResolveSpecificReactions(reactions, sourceImpact, opponentImpact, rng)
//
// 		if sourceImpact.StatusesChange[model.StatusInfallible] != 1 {
// 			t.Errorf("Source should have received Infallible status")
// 		}
// 		if opponentImpact.StatusesChange[model.StatusBlindness] != 1 {
// 			t.Errorf("Opponent should have received Blindness status")
// 		}
// 		if sourceImpact.StatusesChange[model.StatusBlindness] == 1 {
// 			t.Errorf("Source should NOT have received Blindness status")
// 		}
// 	})
// }
