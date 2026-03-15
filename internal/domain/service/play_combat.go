package service

import (
	"github.com/unclestep/Rogue/internal/domain/model"
)

type Combat struct {
	impactResolver *ImpactResolver
}

func NewCombatService(impactResolver *ImpactResolver) *Combat {
	return &Combat{
		impactResolver: impactResolver,
	}
}

type AttackEvent struct {
	Attacker       *model.ActorImpact
	Defender       *model.ActorImpact
	Outcome        AttackOutcome
	impactResolver *ImpactResolver
}

//go:generate stringer -type=AttackOutcome
type AttackOutcome int

const (
	AttackOutcomeSuccess AttackOutcome = iota
	AttackOutcomeMissed
	AttackOutcomeCantAttack
	AttackOutcomeNoStamina
	AttackOutcomeParticipantIsAlreadyDead
)

func (c *Combat) ExecuteAttack(ctx *model.SessionContext, attacker, defender *model.Actor) *AttackEvent {
	if attacker == nil || defender == nil {
		return nil
	}

	event := &AttackEvent{impactResolver: c.impactResolver}

	// If one of participants is already dead
	if attacker.Vitals[model.VitalHP] <= 0 || defender.Vitals[model.VitalHP] <= 0 {
		event.Outcome = AttackOutcomeParticipantIsAlreadyDead
		return event
	}

	event.Attacker = model.NewActorImpact(attacker)
	event.Defender = model.NewActorImpact(defender)

	if !attacker.CanAttack() {
		event.Outcome = AttackOutcomeCantAttack
		return event
	}

	if !attacker.HasStaminaForHit() {
		event.Outcome = AttackOutcomeNoStamina
		return event
	}

	if attacker.Kind != model.ActorPlayer && defender.Kind != model.ActorPlayer {
		event.Outcome = AttackOutcomeCantAttack
		return event
	}

	event.Attacker.VitalsChange[model.VitalStamina] -= attacker.DerivedAttrs[model.AttrAttackStaminaCost]

	c.impactResolver.ResolveReactions(model.TriggerOnPreHit, event.Attacker, event.Defender, ctx.Rng())
	c.impactResolver.ResolveReactions(model.TriggerOnPreHit, event.Defender, event.Attacker, ctx.Rng())

	chance := event.calcHitChance()
	event.Attacker.DecrementAllRelatedCharges(model.TriggerOnPreHit)
	event.Defender.DecrementAllRelatedCharges(model.TriggerOnPreHit)

	if ctx.Rng().Intn(model.Guaranteed) < chance {
		event.Outcome = AttackOutcomeMissed
		return event
	}

	event.Defender.VitalsChange[model.VitalHP] -= attacker.DerivedAttrs[model.AttrStrength]

	c.impactResolver.ResolveReactions(model.TriggerOnHit, event.Attacker, event.Defender, ctx.Rng())
	event.Attacker.DecrementAllRelatedCharges(model.TriggerOnHit)

	c.impactResolver.ResolveReactions(model.TriggerOnDamage, event.Defender, event.Attacker, ctx.Rng())
	event.Defender.DecrementAllRelatedCharges(model.TriggerOnDamage)

	return event
}

func (event *AttackEvent) calcHitChance() int {
	defender := event.Defender
	attacker := event.Attacker

	if status, effect := defender.GetStatus(model.StatusUntouchable); status > 0 {
		defender.ConsumeEffect(effect)
		return 0
	}

	if status, effect := attacker.GetStatus(model.StatusInfallible); status > 0 {
		attacker.ConsumeEffect(effect)
		return 100
	}

	attackerChance := attacker.Actor.DerivedAttrs[model.AttrDexterity]
	defenderChance := defender.Actor.DerivedAttrs[model.AttrDexterity]

	return attackerChance / (attackerChance + defenderChance) * 100
}

func (event *AttackEvent) WasPerformed() bool {
	return event.Outcome == AttackOutcomeSuccess || event.Outcome == AttackOutcomeMissed
}

func (event *AttackEvent) Perform(ctx *model.SessionContext) {
	if !event.WasPerformed() {
		return
	}

	event.impactResolver.ApplyImpact(event.Attacker, ctx.Rng())
	event.impactResolver.ApplyImpact(event.Defender, ctx.Rng())

	attacker := event.Attacker.Actor
	defender := event.Defender.Actor

	// Attacker can be killed by defender in different situations: effects, counter-attacks, special gear
	if attacker.Vitals[model.VitalHP] <= 0 {
		ctx.Playthrough.KillActor(attacker)
	}

	if defender.Vitals[model.VitalHP] <= 0 {
		ctx.Playthrough.KillActor(defender)
	}
}
