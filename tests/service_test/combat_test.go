// package service_test
//
// import (
// 	"math/rand"
// 	"testing"
// 	"time"
//
// 	"github.com/unclestep/Rogue/internal/domain/model"
// 	"github.com/unclestep/Rogue/internal/domain/service"
// 	"github.com/unclestep/Rogue/pkg/geometry"
// )
//
// func setupTestEnv() (*model.GameSession, *service.Combat, *model.Actor, *model.Actor) {
// 	session := model.NewGameSession()
// 	session.Monsters = make(map[model.ActorId]*model.Actor)
// 	session.Items = make(map[model.ItemId]*model.Item)
//
// 	cs := service.NewCombatService(session, time.Now().UnixNano())
//
// 	attacker := model.NewDefaultPlayer(1, geometry.Point{X: 0, Y: 0})
// 	defender := model.NewDefaultZombie(2, geometry.Point{X: 1, Y: 0})
//
// 	session.Monsters[attacker.Id] = attacker
// 	session.Monsters[defender.Id] = defender
//
// 	return session, cs, attacker, defender
// }
//
// func TestExecuteAttackBasic(t *testing.T) {
// 	_, cs, attacker, defender := setupTestEnv()
//
// 	t.Run("Outcome: Success (Hit)", func(t *testing.T) {
// 		attacker.DerivedAttrs[model.AttrDexterity] = 100
// 		defender.DerivedAttrs[model.AttrDexterity] = 0
//
// 		event := cs.ExecuteAttack(attacker, defender)
//
// 		if event.Outcome != service.AttackOutcomeSuccess {
// 			t.Errorf("Expected Success, got %v", event.Outcome)
// 		}
//
// 		staminaCost := attacker.DerivedAttrs[model.AttrAttackStaminaCost]
// 		if event.Attacker.VitalsChange[model.VitalStamina] != -staminaCost {
// 			t.Errorf("Expected stamina deduction %d, got %d", -staminaCost, event.Attacker.VitalsChange[model.VitalStamina])
// 		}
//
// 		damage := attacker.DerivedAttrs[model.AttrStrength]
// 		if event.Defender.VitalsChange[model.VitalHP] != -damage {
// 			t.Errorf("Expected HP damage %d, got %d", -damage, event.Defender.VitalsChange[model.VitalHP])
// 		}
// 	})
//
// 	t.Run("Outcome: Missed", func(t *testing.T) {
// 		attacker.Vitals[model.VitalStamina] = attacker.BaseAttrs[model.AttrStaminaRegen]
// 		attacker.DerivedAttrs[model.AttrDexterity] = 0
// 		defender.DerivedAttrs[model.AttrDexterity] = 100
//
// 		event := cs.ExecuteAttack(attacker, defender)
//
// 		if event.Outcome != service.AttackOutcomeMissed {
// 			t.Errorf("Expected Missed, got %v", event.Outcome)
// 		}
//
// 		if event.Attacker.VitalsChange[model.VitalStamina] == 0 {
// 			t.Error("Stamina should be deducted even on miss")
// 		}
//
// 		if event.Defender.VitalsChange[model.VitalHP] != 0 {
// 			t.Error("HP should not change on miss")
// 		}
// 	})
//
// 	t.Run("Outcome: No Stamina", func(t *testing.T) {
// 		attacker.Vitals[model.VitalStamina] = 0
//
// 		event := cs.ExecuteAttack(attacker, defender)
//
// 		if event.Outcome != service.AttackOutcomeNoStamina {
// 			t.Errorf("Expected NoStamina, got %v", event.Outcome)
// 		}
// 	})
//
// 	t.Run("Outcome: Cannot Attack", func(t *testing.T) {
// 		attacker.Vitals[model.VitalStamina] = attacker.BaseAttrs[model.AttrStaminaRegen]
// 		attacker.Statuses[model.StatusSleep] = 1
//
// 		event := cs.ExecuteAttack(attacker, defender)
//
// 		if event.Outcome != service.AttackOutcomeStatusCantAttack {
// 			t.Errorf("Expected StatusCantAttack, got %v", event.Outcome)
// 		}
// 	})
//
// 	t.Run("Outcome: Already Dead", func(t *testing.T) {
// 		attacker.Statuses[model.StatusSleep] = 0
// 		defender.Vitals[model.VitalHP] = 0
//
// 		event := cs.ExecuteAttack(attacker, defender)
//
// 		if event.Outcome != service.AttackOutcomeParticipantIsAlreadyDead {
// 			t.Errorf("Expected ParticipantIsAlreadyDead, got %v", event.Outcome)
// 		}
// 	})
// }
//
// func TestExecuteAttackAdvanced(t *testing.T) {
// 	_, cs, attacker, defender := setupTestEnv()
//
// 	t.Run("Defender Status Untouchable", func(t *testing.T) {
// 		untouchableEffect := model.NewUntouchableEffect(-1, 1)
// 		defender.AddEffect(untouchableEffect)
// 		defender.RecomputeStats()
//
// 		attacker.DerivedAttrs[model.AttrDexterity] = 100
// 		defender.DerivedAttrs[model.AttrDexterity] = 0
//
// 		event := cs.ExecuteAttack(attacker, defender)
//
// 		if event.Outcome != service.AttackOutcomeMissed {
// 			t.Errorf("Expected Missed due to Untouchable status, got %v", event.Outcome)
// 		}
//
// 		if change, exists := event.Defender.EffectChargesChange[untouchableEffect]; !exists || change != -1 {
// 			t.Errorf("Untouchable effect charges should be decremented by 1, got %v\n", change)
// 		}
//
// 		if toRemove, exists := event.Defender.EffectsToRemove[untouchableEffect]; !exists || !toRemove {
// 			t.Errorf("Untouchable effect should be marked as needed to remove, got %v\n", toRemove)
// 		}
// 	})
//
// 	t.Run("Attacker Status Infallible", func(t *testing.T) {
// 		delete(defender.Effects, model.EffectUntouchable)
//
// 		infallibleEffect := model.NewInfallibleEffect(-1, 1)
// 		attacker.AddEffect(infallibleEffect)
//
// 		attacker.DerivedAttrs[model.AttrDexterity] = 0
// 		defender.DerivedAttrs[model.AttrDexterity] = 100
//
// 		event := cs.ExecuteAttack(attacker, defender)
//
// 		if event.Outcome != service.AttackOutcomeSuccess {
// 			t.Errorf("Expected Success due to Infallible status, got %v", event.Outcome)
// 		}
//
// 		if toRemove, exists := event.Attacker.EffectsToRemove[infallibleEffect]; !exists || !toRemove {
// 			t.Errorf("Untouchable effect should be marked as needed to remove, got %v\n", toRemove)
// 		}
// 	})
// }
//
// func TestPerformDeathAndLoot(t *testing.T) {
// 	session, cs, attacker, defender := setupTestEnv()
//
// 	seed := time.Now().UnixNano()
// 	rng := rand.New(rand.NewSource(seed))
//
// 	session.Map.GenerateTopology(1, 1, rng)
// 	attacker.Pos = session.Map.Rooms[0].GetPos()
// 	defender.Pos = attacker.Pos.Add(geometry.Point{X: 1, Y: 0})
//
// 	t.Run("Kill Enemy and Spawn Loot", func(t *testing.T) {
// 		attacker.AddEffect(model.NewInfallibleEffect(1, 1))
// 		attacker.DerivedAttrs[model.AttrStrength] = 100
// 		defender.Vitals[model.VitalHP] = 1
//
// 		event := cs.ExecuteAttack(attacker, defender)
// 		if event.Outcome != service.AttackOutcomeSuccess {
// 			t.Errorf("Expected %v, got %v\n", service.AttackOutcomeSuccess, event.Outcome)
// 		}
//
// 		event.Perform(session, rng)
//
// 		if defender.Vitals[model.VitalHP] > 0 {
// 			t.Errorf("Expected defender HP: <=0, got: %v\n", defender.Vitals[model.VitalHP])
// 		}
//
// 		if _, exists := session.Monsters[defender.Id]; exists {
// 			t.Error("Dead actor should be removed from session")
// 		}
//
// 		if session.Map.IsActor(defender.Pos) {
// 			t.Error("Dead actor should be removed from map grid")
// 		}
//
// 		if id, _ := session.Map.GetItemID(defender.Pos); id == 0 {
// 			t.Error("Should be an item in place of actor death")
// 			item, exists := session.Items[model.ItemId(id)]
// 			if !exists {
// 				t.Error("Should exist in item map")
// 			}
// 			if item.Kind != model.ItemTypeTreasure {
// 				t.Error("Should be treasures type")
// 			}
// 		}
// 	})
// }
//
// func TestExecuteAttackTriggers(t *testing.T) {
// 	seed := time.Now().UnixNano()
// 	rng := rand.New(rand.NewSource(seed))
//
// 	t.Run("TriggerOnPreHit: Gain Infallible", func(t *testing.T) {
// 		session, cs, attacker, defender := setupTestEnv()
// 		attacker.DerivedAttrs[model.AttrDexterity] = 0
// 		defender.DerivedAttrs[model.AttrDexterity] = 100
//
// 		attacker.Traits[model.TriggerOnPreHit] = []*model.Reaction{
// 			{
// 				Trigger: model.TriggerOnPreHit,
// 				Target:  model.TargetSource,
// 				Chance:  model.Guaranteed,
// 				EffectsToApply: map[model.EffectType]*model.Effect{
// 					model.EffectInfallible: model.NewInfallibleEffect(-1, 2),
// 				},
// 			},
// 		}
//
// 		event := cs.ExecuteAttack(attacker, defender)
// 		event.Perform(session, rng)
// 		event.Attacker.Actor.RecomputeStats()
//
// 		if event.Outcome != service.AttackOutcomeSuccess {
// 			t.Errorf("Expected Success due to PreHit Infallible buff, got %v", event.Outcome)
// 		}
//
// 		if event.Attacker.Actor.Statuses[model.StatusInfallible] != 1 {
// 			t.Error("Expected StatusInfallible to be added to Attacker changes")
// 		}
// 	})
//
// 	t.Run("TriggerOnDamage: Kill Attacker", func(t *testing.T) {
// 		_, cs, attacker, defender := setupTestEnv()
//
// 		attacker.DerivedAttrs[model.AttrDexterity] = 100
// 		defender.DerivedAttrs[model.AttrDexterity] = 0
//
// 		attackerHP := attacker.Vitals[model.VitalHP]
//
// 		defender.Traits[model.TriggerOnDamage] = []*model.Reaction{
// 			{
// 				Target: model.TargetOpponent,
// 				Chance: model.Guaranteed,
// 				VitalsChange: map[model.VitalType]model.Change{
// 					model.VitalHP: {
// 						Holder: model.TargetOpponent,
// 						Vital:  model.VitalHP,
// 						Scale:  -1,
// 					},
// 				},
// 			},
// 		}
//
// 		event := cs.ExecuteAttack(attacker, defender)
//
// 		if event.Outcome != service.AttackOutcomeSuccess {
// 			t.Errorf("Attack should succeed")
// 		}
//
// 		defenderTakenDamage := attacker.DerivedAttrs[model.AttrStrength]
// 		if event.Defender.VitalsChange[model.VitalHP] != -defenderTakenDamage {
// 			t.Errorf("Defender should take normal damage, got %d", event.Defender.VitalsChange[model.VitalHP])
// 		}
//
// 		attackerTakenDamage := -attackerHP
// 		if event.Attacker.VitalsChange[model.VitalHP] != attackerTakenDamage {
// 			t.Errorf("Attacker should be dead after attack and has %d, got %d", 0, event.Attacker.VitalsChange[model.VitalHP])
// 		}
// 	})
//
// 	t.Run("TriggerOnHit: Vampirism (Heal Self) and Sleep Opponent", func(t *testing.T) {
// 		_, cs, attacker, defender := setupTestEnv()
//
// 		attacker.Traits[model.TriggerOnHit] = []*model.Reaction{
// 			{
// 				Target: model.TargetSource,
// 				Chance: model.Guaranteed,
// 				VitalsChange: map[model.VitalType]model.Change{
// 					model.VitalHP: {
// 						Holder: model.TargetSource,
// 						Vital:  model.VitalHP,
// 						Amount: 10,
// 						Scale:  0,
// 					},
// 				},
// 			},
// 			{
// 				Target: model.TargetOpponent,
// 				Chance: model.Guaranteed,
// 				StatusesChange: map[model.StatusType]int{
// 					model.StatusSleep: 1,
// 				},
// 			},
// 		}
//
// 		attacker.DerivedAttrs[model.AttrDexterity] = 100
// 		defender.DerivedAttrs[model.AttrDexterity] = 0
//
// 		event := cs.ExecuteAttack(attacker, defender)
//
// 		if event.Outcome != service.AttackOutcomeSuccess {
// 			t.Errorf("Attack must succeed")
// 		}
//
// 		if event.Attacker.VitalsChange[model.VitalHP] != 10 {
// 			t.Errorf("Expected Vampirism heal +10, got %d", event.Attacker.VitalsChange[model.VitalHP])
// 		}
//
// 		if event.Defender.StatusesChange[model.StatusSleep] != 1 {
// 			t.Error("Expected defender got sleep status")
// 		}
// 	})
//
// 	t.Run("Trigger Interaction: Miss prevents OnHit/OnDamage", func(t *testing.T) {
// 		_, cs, attacker, defender := setupTestEnv()
// 		attacker.Traits[model.TriggerOnHit] = []*model.Reaction{
// 			{
// 				Target: model.TargetSource,
// 				VitalsChange: map[model.VitalType]model.Change{model.VitalHP: {
// 					Holder: model.TargetSource,
// 					Vital:  model.VitalHP,
// 					Amount: 100,
// 					Scale:  0,
// 				}},
// 			},
// 		}
// 		defender.Traits[model.TriggerOnDamage] = []*model.Reaction{
// 			{
// 				Target: model.TargetOpponent,
// 				VitalsChange: map[model.VitalType]model.Change{model.VitalHP: {
// 					Holder: model.TargetSource,
// 					Vital:  model.VitalHP,
// 					Amount: -50,
// 					Scale:  0,
// 				}},
// 			},
// 		}
//
// 		attacker.DerivedAttrs[model.AttrDexterity] = 0
// 		defender.DerivedAttrs[model.AttrDexterity] = 100
//
// 		event := cs.ExecuteAttack(attacker, defender)
//
// 		if event.Outcome != service.AttackOutcomeMissed {
// 			t.Errorf("Expected miss")
// 		}
//
// 		if event.Attacker.VitalsChange[model.VitalHP] != 0 {
// 			t.Error("OnHit should not trigger on miss")
// 		}
// 	})
// }
