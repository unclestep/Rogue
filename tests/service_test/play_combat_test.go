package service_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func setupTestEnv() (*model.SessionContext, *service.Combat, *model.Actor, *model.Actor) {
	play := model.NewPlaythrough("", 0, 0)
	ctx := model.NewSessionContext(play)
	impactResolver := service.NewImpactResolverService()
	cs := service.NewCombatService(impactResolver)

	attacker := model.NewDefaultPlayer(1, geometry.Point{X: 0, Y: 0}, 9)
	defender := model.NewDefaultZombie(2, geometry.Point{X: 1, Y: 0})

	play.Players[attacker.Id] = attacker
	play.Monsters[defender.Id] = defender

	return ctx, cs, attacker, defender
}

func TestExecuteAttackBasicOutcomes(t *testing.T) {
	ctx, cs, attacker, defender := setupTestEnv()

	t.Run("Outcome: Success (Hit)", func(t *testing.T) {
		attacker.DerivedAttrs[model.AttrDexterity] = 100
		defender.DerivedAttrs[model.AttrDexterity] = 0

		event := cs.ExecuteAttack(ctx, attacker, defender)

		if event.Outcome != service.AttackOutcomeSuccess {
			t.Errorf("Expected Success, got %v", event.Outcome)
		}

		staminaCost := attacker.DerivedAttrs[model.AttrAttackStaminaCost]
		if event.Attacker.VitalsChange[model.VitalStamina] != -staminaCost {
			t.Errorf("Expected stamina deduction %d, got %d", -staminaCost, event.Attacker.VitalsChange[model.VitalStamina])
		}

		damage := attacker.DerivedAttrs[model.AttrStrength]
		if event.Defender.VitalsChange[model.VitalHP] != -damage {
			t.Errorf("Expected HP damage %d, got %d", -damage, event.Defender.VitalsChange[model.VitalHP])
		}
	})

	t.Run("Outcome: Missed", func(t *testing.T) {
		attacker.DerivedAttrs[model.AttrDexterity] = 0
		defender.DerivedAttrs[model.AttrDexterity] = 100

		event := cs.ExecuteAttack(ctx, attacker, defender)

		if event.Outcome != service.AttackOutcomeMissed {
			t.Errorf("Expected Missed, got %v", event.Outcome)
		}

		staminaCost := attacker.DerivedAttrs[model.AttrAttackStaminaCost]
		if event.Attacker.VitalsChange[model.VitalStamina] != -staminaCost {
			t.Errorf("Stamina should be deducted even on miss, expected %d got %d", -staminaCost, event.Attacker.VitalsChange[model.VitalStamina])
		}

		if event.Defender.VitalsChange[model.VitalHP] != 0 {
			t.Errorf("HP should not change on miss, got %d", event.Defender.VitalsChange[model.VitalHP])
		}
	})

	t.Run("Outcome: No Stamina", func(t *testing.T) {
		attacker.Vitals[model.VitalStamina] = 0

		event := cs.ExecuteAttack(ctx, attacker, defender)

		if event.Outcome != service.AttackOutcomeNoStamina {
			t.Errorf("Expected NoStamina, got %v", event.Outcome)
		}
	})

	t.Run("Outcome: Cannot Attack (Status)", func(t *testing.T) {
		attacker.Vitals[model.VitalStamina] = model.MediumAttackStaminaCost

		attacker.Statuses[model.StatusSleep] = 1

		event := cs.ExecuteAttack(ctx, attacker, defender)

		if event.Outcome != service.AttackOutcomeCantAttack {
			t.Errorf("Expected CantAttack, got %v", event.Outcome)
		}
	})

	t.Run("Outcome: Monsters Cannot Attack Monsters", func(t *testing.T) {
		delete(attacker.Statuses, model.StatusSleep)

		attacker.Kind = model.ActorOgre
		defender.Kind = model.ActorVampire

		event := cs.ExecuteAttack(ctx, attacker, defender)

		if event.Outcome != service.AttackOutcomeCantAttack {
			t.Errorf("Expected CantAttack for PvE restriction, got %v", event.Outcome)
		}
	})

	t.Run("Outcome: Already Dead", func(t *testing.T) {
		attacker.Kind = model.ActorPlayer
		defender.Vitals[model.VitalHP] = 0

		event := cs.ExecuteAttack(ctx, attacker, defender)

		if event.Outcome != service.AttackOutcomeParticipantIsAlreadyDead {
			t.Errorf("Expected ParticipantIsAlreadyDead, got %v", event.Outcome)
		}
	})
}

func TestExecuteAttackStatusEffects(t *testing.T) {
	ctx, cs, attacker, defender := setupTestEnv()

	t.Run("Defender Status Untouchable forces Miss", func(t *testing.T) {
		untouchableEffect := model.NewUntouchableEffect(-1, 1)
		defender.AddEffect(untouchableEffect)
		defender.RecomputeStats()

		attacker.DerivedAttrs[model.AttrDexterity] = 100
		defender.DerivedAttrs[model.AttrDexterity] = 0

		event := cs.ExecuteAttack(ctx, attacker, defender)

		if event.Outcome != service.AttackOutcomeMissed {
			t.Errorf("Expected Missed due to Untouchable status, got %v", event.Outcome)
		}

		if change, exists := event.Defender.EffectChargesChange[untouchableEffect]; !exists || change != -1 {
			t.Errorf("Untouchable effect charges should be decremented by 1, got %v", change)
		}
	})

	t.Run("Attacker Status Infallible forces Success", func(t *testing.T) {
		delete(defender.Effects, model.EffectUntouchable)
		infallibleEffect := model.NewInfallibleEffect(-1, 1)
		attacker.AddEffect(infallibleEffect)
		attacker.RecomputeStats()

		attacker.DerivedAttrs[model.AttrDexterity] = 0
		defender.DerivedAttrs[model.AttrDexterity] = 100

		event := cs.ExecuteAttack(ctx, attacker, defender)

		if event.Outcome != service.AttackOutcomeSuccess {
			t.Errorf("Expected Success due to Infallible status, got %v", event.Outcome)
		}

		if change, exists := event.Attacker.EffectChargesChange[infallibleEffect]; !exists || change != -1 {
			t.Errorf("Infallible effect charges should be decremented by 1, got %v", change)
		}
	})
}

func TestPerformDeathAndCleanup(t *testing.T) {
	ctx, cs, attacker, defender := setupTestEnv()

	t.Run("Kill Enemy and Move to DeadMonsters", func(t *testing.T) {
		attacker.AddEffect(model.NewInfallibleEffect(1, 1))
		attacker.DerivedAttrs[model.AttrStrength] = 100

		defender.Vitals[model.VitalHP] = 1
		defender.Pos = geometry.Point{X: 1, Y: 1}

		event := cs.ExecuteAttack(ctx, attacker, defender)
		if event.Outcome != service.AttackOutcomeSuccess {
			t.Fatalf("Expected %v, got %v", service.AttackOutcomeSuccess, event.Outcome)
		}

		event.Perform(ctx)

		if defender.Vitals[model.VitalHP] > 0 {
			t.Errorf("Expected defender HP to be <=0, got: %v", defender.Vitals[model.VitalHP])
		}

		if _, exists := ctx.Playthrough.Monsters[defender.Id]; exists {
			t.Error("Dead monster should be removed from Playthrough.Monsters")
		}

		if _, exists := ctx.Playthrough.DeadMonsters[defender.Id]; !exists {
			t.Error("Dead monster should be moved to Playthrough.DeadMonsters")
		}

		if defender.Pos == model.NewInvalidPoint() {
			t.Errorf("Dead monster position should be InvalidPoint, got %v", defender.Pos)
		}
	})
}

func TestPerformDeathAndHide(t *testing.T) {
	ctx, cs, defender, attacker := setupTestEnv()

	t.Run("Kill Player and Hide", func(t *testing.T) {
		attacker.AddEffect(model.NewInfallibleEffect(1, 1))
		attacker.DerivedAttrs[model.AttrStrength] = 100

		defender.Vitals[model.VitalHP] = 1
		defender.Pos = geometry.Point{X: 1, Y: 1}

		event := cs.ExecuteAttack(ctx, attacker, defender)
		if event.Outcome != service.AttackOutcomeSuccess {
			t.Fatalf("Expected %v, got %v", service.AttackOutcomeSuccess, event.Outcome)
		}

		event.Perform(ctx)

		if defender.Vitals[model.VitalHP] > 0 {
			t.Errorf("Expected defender HP to be <=0, got: %v", defender.Vitals[model.VitalHP])
		}

		if _, exists := ctx.Playthrough.Players[defender.Id]; !exists {
			t.Error("Dead player should rest in Playthrough.Players")
		}

		if _, exists := ctx.Playthrough.DeadMonsters[defender.Id]; exists {
			t.Error("Dead player should not be moved to Playthrough.DeadMonsters")
		}

		if defender.Pos != model.NewInvalidPoint() {
			t.Errorf("Dead player position should be InvalidPoint, got %v", defender.Pos)
		}
	})
}

func TestExecuteAttackTriggers(t *testing.T) {
	t.Run("TriggerOnPreHit: Gain Infallible", func(t *testing.T) {
		ctx, cs, attacker, defender := setupTestEnv()
		attacker.DerivedAttrs[model.AttrDexterity] = 0
		defender.DerivedAttrs[model.AttrDexterity] = 100

		attacker.Traits[model.TriggerOnPreHit] = []*model.Reaction{
			{
				Trigger: model.TriggerOnPreHit,
				Target:  model.TargetSource,
				Chance:  model.Guaranteed,
				EffectsToApply: map[model.EffectType]*model.Effect{
					model.EffectInfallible: model.NewInfallibleEffect(1, 1),
				},
			},
		}

		event := cs.ExecuteAttack(ctx, attacker, defender)
		event.Perform(ctx)

		if event.Outcome != service.AttackOutcomeSuccess {
			t.Errorf("Expected Success due to PreHit Infallible buff, got %v", event.Outcome)
		}

		if _, exists := event.Attacker.AppliedEffects[model.EffectInfallible]; exists {
			t.Error("EffectInfallible should be applied and removed at the same turn")
		}
	})

	t.Run("TriggerOnDamage: Reflect Damage to Attacker", func(t *testing.T) {
		ctx, cs, attacker, defender := setupTestEnv()

		attacker.DerivedAttrs[model.AttrDexterity] = 100
		defender.DerivedAttrs[model.AttrDexterity] = 0
		attacker.DerivedAttrs[model.AttrStrength] = 50

		defender.Traits[model.TriggerOnDamage] = []*model.Reaction{
			{
				Target: model.TargetOpponent,
				Chance: model.Guaranteed,
				VitalsChange: map[model.VitalType]model.Change{
					model.VitalHP: {
						Holder: model.TargetOpponent,
						Vital:  model.VitalHP,
						Scale:  -1,
					},
				},
			},
		}

		event := cs.ExecuteAttack(ctx, attacker, defender)

		if event.Outcome != service.AttackOutcomeSuccess {
			t.Fatalf("Attack should succeed")
		}

		defenderTakenDamage := attacker.DerivedAttrs[model.AttrStrength]
		if event.Defender.VitalsChange[model.VitalHP] != -defenderTakenDamage {
			t.Errorf("Defender should take normal damage, got %d", event.Defender.VitalsChange[model.VitalHP])
		}

		if event.Attacker.VitalsChange[model.VitalHP] > 0 {
			t.Errorf("Attacker should have taken damage from TriggerOnDamage reflection, got change: %d", event.Attacker.VitalsChange[model.VitalHP])
		}
	})

	t.Run("TriggerOnHit: Vampirism (Heal Self) and Sleep Opponent", func(t *testing.T) {
		ctx, cs, attacker, defender := setupTestEnv()

		attacker.Traits[model.TriggerOnHit] = []*model.Reaction{
			{
				Target: model.TargetSource,
				Chance: model.Guaranteed,
				VitalsChange: map[model.VitalType]model.Change{
					model.VitalHP: {
						Holder: model.TargetSource,
						Vital:  model.VitalHP,
						Amount: 10,
						Scale:  0,
					},
				},
			},
			{
				Target: model.TargetOpponent,
				Chance: model.Guaranteed,
				StatusesChange: map[model.StatusType]int{
					model.StatusSleep: 1,
				},
			},
		}

		attacker.DerivedAttrs[model.AttrDexterity] = 100
		defender.DerivedAttrs[model.AttrDexterity] = 0

		event := cs.ExecuteAttack(ctx, attacker, defender)

		if event.Outcome != service.AttackOutcomeSuccess {
			t.Fatalf("Attack must succeed")
		}

		if event.Attacker.VitalsChange[model.VitalHP] != 10 {
			t.Errorf("Expected Vampirism heal +10, got %d", event.Attacker.VitalsChange[model.VitalHP])
		}

		if event.Defender.StatusesChange[model.StatusSleep] != 1 {
			t.Error("Expected defender to get sleep status change +1")
		}
	})

	t.Run("Trigger Interaction: Miss prevents OnHit and OnDamage", func(t *testing.T) {
		ctx, cs, attacker, defender := setupTestEnv()

		attacker.Traits[model.TriggerOnHit] = []*model.Reaction{
			{
				Target:       model.TargetSource,
				Chance:       model.Guaranteed,
				VitalsChange: map[model.VitalType]model.Change{model.VitalHP: {Amount: 100}},
			},
		}
		defender.Traits[model.TriggerOnDamage] = []*model.Reaction{
			{
				Target:       model.TargetOpponent,
				Chance:       model.Guaranteed,
				VitalsChange: map[model.VitalType]model.Change{model.VitalHP: {Amount: -50}},
			},
		}

		attacker.DerivedAttrs[model.AttrDexterity] = 0
		defender.DerivedAttrs[model.AttrDexterity] = 100

		event := cs.ExecuteAttack(ctx, attacker, defender)

		if event.Outcome != service.AttackOutcomeMissed {
			t.Fatalf("Expected miss")
		}

		if event.Attacker.VitalsChange[model.VitalHP] != 0 {
			t.Error("TriggerOnHit should not trigger on miss")
		}

		if event.Defender.VitalsChange[model.VitalHP] != 0 {
			t.Error("TriggerOnDamage should not trigger on miss")
		}
	})
}
