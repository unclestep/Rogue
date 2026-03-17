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

	// One of participants is already dead
	if attacker.Vitals[model.VitalHP] <= 0 || defender.Vitals[model.VitalHP] <= 0 {
		event.Outcome = AttackOutcomeParticipantIsAlreadyDead
		return event
	}

	event.Attacker = model.NewActorImpact(attacker)
	event.Defender = model.NewActorImpact(defender)

	// Attacker has statuses which prohibit him attacking other
	if !attacker.CanAttack() {
		event.Outcome = AttackOutcomeCantAttack
		return event
	}

	// Attacker has not enough stamina for hit
	if !attacker.HasStaminaForHit() {
		event.Outcome = AttackOutcomeNoStamina
		return event
	}

	// Monsters cannot attack each other
	if attacker.Kind != model.ActorPlayer && defender.Kind != model.ActorPlayer {
		event.Outcome = AttackOutcomeCantAttack
		return event
	}

	// At the moment possible outputs are only miss and hit, so deduct stamina
	event.Attacker.VitalsChange[model.VitalStamina] -= attacker.DerivedAttrs[model.AttrAttackStaminaCost]

	// Resolve reaction on pre-hit, attacker and defender can have special abilities or effects that can change combat outcome
	c.impactResolver.ResolveReactions(model.TriggerOnPreHit, event.Attacker, event.Defender, ctx.Rng())
	c.impactResolver.ResolveReactions(model.TriggerOnPreHit, event.Defender, event.Attacker, ctx.Rng())

	// Assess our chances
	chance := event.calcHitChance()

	// Now we can safely decrement effects' charges (ran out effects are not taken into account during probability calcutations)
	event.Attacker.DecrementAllRelatedCharges(model.TriggerOnPreHit)
	event.Defender.DecrementAllRelatedCharges(model.TriggerOnPreHit)

	// Check: hit or miss
	if ctx.Rng().Intn(100) >= chance {
		// Miss, so do not reduce defender's health and do not resolve reactions on hit and on damage
		event.Outcome = AttackOutcomeMissed
		return event
	}

	// Hit, so deduct damage from defender's health
	event.Defender.VitalsChange[model.VitalHP] -= attacker.DerivedAttrs[model.AttrStrength]

	// Resolve attacker's reactions on hit
	c.impactResolver.ResolveReactions(model.TriggerOnHit, event.Attacker, event.Defender, ctx.Rng())
	event.Attacker.DecrementAllRelatedCharges(model.TriggerOnHit)

	// Resolve defender's reactions on damage
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
