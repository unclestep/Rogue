package service_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/domain/entity"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/pkg/geometry"
)

func setupTestEnv() (*entity.GameSession, *service.Combat, *entity.Actor, *entity.Actor) {
	session := entity.NewGameSession()
	session.Actors = make(map[entity.ActorId]*entity.Actor)
	session.Items = make(map[entity.ItemId]*entity.Item)

	cs := service.NewCombatService(session)

	attacker := entity.NewDefaultPlayer(1, geometry.Point{X: 0, Y: 0})
	defender := entity.NewDefaultZombie(2, geometry.Point{X: 1, Y: 0})

	session.Actors[attacker.Id] = attacker
	session.Actors[defender.Id] = defender
	session.Player = attacker

	return session, cs, attacker, defender
}

func TestExecuteAttackBasic(t *testing.T) {
	_, cs, attacker, defender := setupTestEnv()

	t.Run("Outcome: Success (Hit)", func(t *testing.T) {
		attacker.DerivedAttrs[entity.Dexterity] = 100
		defender.DerivedAttrs[entity.Dexterity] = 0

		event := cs.ExecuteAttack(attacker, defender)

		if event.Outcome != service.AttackOutcomeSuccess {
			t.Errorf("Expected Success, got %v", event.Outcome)
		}

		staminaCost := attacker.DerivedAttrs[entity.AttackStaminaCost]
		if event.Attacker.VitalsChange[entity.Stamina] != -staminaCost {
			t.Errorf("Expected stamina deduction %d, got %d", -staminaCost, event.Attacker.VitalsChange[entity.Stamina])
		}

		damage := attacker.DerivedAttrs[entity.Strength]
		if event.Defender.VitalsChange[entity.HP] != -damage {
			t.Errorf("Expected HP damage %d, got %d", -damage, event.Defender.VitalsChange[entity.HP])
		}
	})

	t.Run("Outcome: Missed", func(t *testing.T) {
		attacker.Vitals[entity.Stamina] = attacker.BaseAttrs[entity.StaminaRegen]
		attacker.DerivedAttrs[entity.Dexterity] = 0
		defender.DerivedAttrs[entity.Dexterity] = 100

		event := cs.ExecuteAttack(attacker, defender)

		if event.Outcome != service.AttackOutcomeMissed {
			t.Errorf("Expected Missed, got %v", event.Outcome)
		}

		if event.Attacker.VitalsChange[entity.Stamina] == 0 {
			t.Error("Stamina should be deducted even on miss")
		}

		if event.Defender.VitalsChange[entity.HP] != 0 {
			t.Error("HP should not change on miss")
		}
	})

	t.Run("Outcome: No Stamina", func(t *testing.T) {
		attacker.Vitals[entity.Stamina] = 0

		event := cs.ExecuteAttack(attacker, defender)

		if event.Outcome != service.AttackOutcomeNoStamina {
			t.Errorf("Expected NoStamina, got %v", event.Outcome)
		}
	})

	t.Run("Outcome: Cannot Attack", func(t *testing.T) {
		attacker.Vitals[entity.Stamina] = attacker.BaseAttrs[entity.StaminaRegen]
		attacker.Statuses[entity.StatusSleep] = 1

		event := cs.ExecuteAttack(attacker, defender)

		if event.Outcome != service.AttackOutcomeStatusCantAttack {
			t.Errorf("Expected StatusCantAttack, got %v", event.Outcome)
		}
	})

	t.Run("Outcome: Already Dead", func(t *testing.T) {
		attacker.Statuses[entity.StatusSleep] = 0
		defender.Vitals[entity.HP] = 0

		event := cs.ExecuteAttack(attacker, defender)

		if event.Outcome != service.AttackOutcomeParticipantIsAlreadyDead {
			t.Errorf("Expected ParticipantIsAlreadyDead, got %v", event.Outcome)
		}
	})
}

func TestExecuteAttackAdvanced(t *testing.T) {
	_, cs, attacker, defender := setupTestEnv()

	t.Run("Defender Status Untouchable", func(t *testing.T) {
		untouchableEffect := entity.NewUntouchableEffect(-1, 1)
		defender.AddEffect(untouchableEffect)
		defender.RecomputeStats()

		attacker.DerivedAttrs[entity.Dexterity] = 100
		defender.DerivedAttrs[entity.Dexterity] = 0

		event := cs.ExecuteAttack(attacker, defender)

		if event.Outcome != service.AttackOutcomeMissed {
			t.Errorf("Expected Missed due to Untouchable status, got %v", event.Outcome)
		}

		if change, exists := event.Defender.EffectChargesChange[untouchableEffect]; !exists || change != -1 {
			t.Errorf("Untouchable effect charges should be decremented by 1, got %v\n", change)
		}

		if toRemove, exists := event.Defender.EffectsToRemove[untouchableEffect]; !exists || !toRemove {
			t.Errorf("Untouchable effect should be marked as needed to remove, got %v\n", toRemove)
		}
	})

	t.Run("Attacker Status Infallible", func(t *testing.T) {
		delete(defender.Effects, entity.UntouchableEffect)

		infallibleEffect := entity.NewInfallibleEffect(-1, 1)
		attacker.AddEffect(infallibleEffect)

		attacker.DerivedAttrs[entity.Dexterity] = 0
		defender.DerivedAttrs[entity.Dexterity] = 100

		event := cs.ExecuteAttack(attacker, defender)

		if event.Outcome != service.AttackOutcomeSuccess {
			t.Errorf("Expected Success due to Infallible status, got %v", event.Outcome)
		}

		if toRemove, exists := event.Attacker.EffectsToRemove[infallibleEffect]; !exists || !toRemove {
			t.Errorf("Untouchable effect should be marked as needed to remove, got %v\n", toRemove)
		}
	})
}

func TestPerformDeathAndLoot(t *testing.T) {
	session, cs, attacker, defender := setupTestEnv()

	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))

	session.Map.GenerateTopology(1, 1, rng)
	attacker.Pos = session.Map.Rooms[0].GetPos()
	defender.Pos = attacker.Pos.Add(geometry.Point{X: 1, Y: 0})

	t.Run("Kill Enemy and Spawn Loot", func(t *testing.T) {
		attacker.AddEffect(entity.NewInfallibleEffect(1, 1))
		attacker.DerivedAttrs[entity.Strength] = 100
		defender.Vitals[entity.HP] = 1

		event := cs.ExecuteAttack(attacker, defender)
		if event.Outcome != service.AttackOutcomeSuccess {
			t.Errorf("Expected %v, got %v\n", service.AttackOutcomeSuccess, event.Outcome)
		}

		event.Perform(session, rng)

		if defender.Vitals[entity.HP] > 0 {
			t.Errorf("Expected defender HP: <=0, got: %v\n", defender.Vitals[entity.HP])
		}

		if _, exists := session.Actors[defender.Id]; exists {
			t.Error("Dead actor should be removed from session")
		}

		if session.Map.IsActor(defender.Pos) {
			t.Error("Dead actor should be removed from map grid")
		}

		if id, _ := session.Map.GetItemID(defender.Pos); id == 0 {
			t.Error("Should be an item in place of actor death")
			item, exists := session.Items[entity.ItemId(id)]
			if !exists {
				t.Error("Should exist in item map")
			}
			if item.Kind != entity.ItemTypeTreasure {
				t.Error("Should be treasures type")
			}
		}
	})
}

func TestExecuteAttackTriggers(t *testing.T) {
	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))

	t.Run("TriggerOnPreHit: Gain Infallible", func(t *testing.T) {
		session, cs, attacker, defender := setupTestEnv()
		attacker.DerivedAttrs[entity.Dexterity] = 0
		defender.DerivedAttrs[entity.Dexterity] = 100

		attacker.Traits[entity.TriggerOnPreHit] = []*entity.Reaction{
			{
				Kind:   entity.TriggerOnPreHit,
				Target: entity.TargetSource,
				Chance: entity.Guaranteed,
				EffectsToApply: map[entity.EffectType]*entity.Effect{
					entity.InfallibleEffect: entity.NewInfallibleEffect(-1, 2),
				},
			},
		}

		event := cs.ExecuteAttack(attacker, defender)
		event.Perform(session, rng)
		event.Attacker.Actor.RecomputeStats()

		if event.Outcome != service.AttackOutcomeSuccess {
			t.Errorf("Expected Success due to PreHit Infallible buff, got %v", event.Outcome)
		}

		if event.Attacker.Actor.Statuses[entity.StatusInfallible] != 1 {
			t.Error("Expected StatusInfallible to be added to Attacker changes")
		}
	})

	t.Run("TriggerOnDamage: Kill Attacker", func(t *testing.T) {
		_, cs, attacker, defender := setupTestEnv()

		attacker.DerivedAttrs[entity.Dexterity] = 100
		defender.DerivedAttrs[entity.Dexterity] = 0

		attackerHP := attacker.Vitals[entity.HP]

		defender.Traits[entity.TriggerOnDamage] = []*entity.Reaction{
			{
				Target: entity.TargetOpponent,
				Chance: entity.Guaranteed,
				VitalsChange: map[entity.VitalType]entity.Change{
					entity.HP: {
						Holder: entity.TargetOpponent,
						Vital:  entity.HP,
						Scale:  -1,
					},
				},
			},
		}

		event := cs.ExecuteAttack(attacker, defender)

		if event.Outcome != service.AttackOutcomeSuccess {
			t.Errorf("Attack should succeed")
		}

		defenderTakenDamage := attacker.DerivedAttrs[entity.Strength]
		if event.Defender.VitalsChange[entity.HP] != -defenderTakenDamage {
			t.Errorf("Defender should take normal damage, got %d", event.Defender.VitalsChange[entity.HP])
		}

		attackerTakenDamage := -attackerHP
		if event.Attacker.VitalsChange[entity.HP] != attackerTakenDamage {
			t.Errorf("Attacker should be dead after attack and has %d, got %d", 0, event.Attacker.VitalsChange[entity.HP])
		}
	})

	t.Run("TriggerOnHit: Vampirism (Heal Self) and Sleep Opponent", func(t *testing.T) {
		_, cs, attacker, defender := setupTestEnv()

		attacker.Traits[entity.TriggerOnHit] = []*entity.Reaction{
			{
				Target: entity.TargetSource,
				Chance: entity.Guaranteed,
				VitalsChange: map[entity.VitalType]entity.Change{
					entity.HP: {
						Holder: entity.TargetSource,
						Vital:  entity.HP,
						Amount: 10,
						Scale:  0,
					},
				},
			},
			{
				Target: entity.TargetOpponent,
				Chance: entity.Guaranteed,
				StatusesChange: map[entity.StatusType]int{
					entity.StatusSleep: 1,
				},
			},
		}

		attacker.DerivedAttrs[entity.Dexterity] = 100
		defender.DerivedAttrs[entity.Dexterity] = 0

		event := cs.ExecuteAttack(attacker, defender)

		if event.Outcome != service.AttackOutcomeSuccess {
			t.Errorf("Attack must succeed")
		}

		if event.Attacker.VitalsChange[entity.HP] != 10 {
			t.Errorf("Expected Vampirism heal +10, got %d", event.Attacker.VitalsChange[entity.HP])
		}

		if event.Defender.StatusesChange[entity.StatusSleep] != 1 {
			t.Error("Expected defender got sleep status")
		}
	})

	t.Run("Trigger Interaction: Miss prevents OnHit/OnDamage", func(t *testing.T) {
		_, cs, attacker, defender := setupTestEnv()
		attacker.Traits[entity.TriggerOnHit] = []*entity.Reaction{
			{
				Target: entity.TargetSource,
				VitalsChange: map[entity.VitalType]entity.Change{entity.HP: {
					Holder: entity.TargetSource,
					Vital:  entity.HP,
					Amount: 100,
					Scale:  0,
				}},
			},
		}
		defender.Traits[entity.TriggerOnDamage] = []*entity.Reaction{
			{
				Target: entity.TargetOpponent,
				VitalsChange: map[entity.VitalType]entity.Change{entity.HP: {
					Holder: entity.TargetSource,
					Vital:  entity.HP,
					Amount: -50,
					Scale:  0,
				}},
			},
		}

		attacker.DerivedAttrs[entity.Dexterity] = 0
		defender.DerivedAttrs[entity.Dexterity] = 100

		event := cs.ExecuteAttack(attacker, defender)

		if event.Outcome != service.AttackOutcomeMissed {
			t.Errorf("Expected miss")
		}

		if event.Attacker.VitalsChange[entity.HP] != 0 {
			t.Error("OnHit should not trigger on miss")
		}
	})
}
