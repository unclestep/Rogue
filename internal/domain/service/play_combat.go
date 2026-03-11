package service

import (
	"math/rand"

	"github.com/unclestep/Rogue/internal/domain/model"
)

type Combat struct{}

func NewCombatService(session *model.GameSession, seed int64) *Combat {
	return &Combat{}
}

type AttackEvent struct {
	Attacker *model.ActorImpact
	Defender *model.ActorImpact
	Outcome  AttackOutcome
}

//go:generate stringer -type=AttackOutcome
type AttackOutcome int

const (
	AttackOutcomeSuccess AttackOutcome = iota
	AttackOutcomeMissed
	AttackOutcomeStatusCantAttack
	AttackOutcomeNoStamina
	AttackOutcomeParticipantIsAlreadyDead
)

func (c *Combat) ExecuteAttack(session *model.GameSession, attacker, defender *model.Actor, rng *rand.Rand) *AttackEvent {
	event := &AttackEvent{}

	// If one of participants is already dead
	if attacker.Vitals[model.VitalHP] <= 0 || defender.Vitals[model.VitalHP] <= 0 {
		event.Outcome = AttackOutcomeParticipantIsAlreadyDead
		return event
	}

	event.Attacker = model.NewActorImpact(attacker)
	event.Defender = model.NewActorImpact(defender)

	if !attacker.CanAttack() {
		if attacker.Kind != model.ActorPlayer {
			event.Attacker.ResetStamina()
		}
		event.Outcome = AttackOutcomeStatusCantAttack
		return event
	}

	if !attacker.HasStaminaForHit() {
		event.Outcome = AttackOutcomeNoStamina
		return event
	}

	event.Attacker.VitalsChange[model.VitalStamina] -= attacker.DerivedAttrs[model.AttrAttackStaminaCost]

	model.ResolveReactions(model.TriggerOnPreHit, event.Attacker, event.Defender, rng)
	model.ResolveReactions(model.TriggerOnPreHit, event.Defender, event.Attacker, rng)

	chance := event.calcHitChance()
	event.Attacker.DecrementAllRelatedCharges(model.TriggerOnPreHit)
	event.Defender.DecrementAllRelatedCharges(model.TriggerOnPreHit)

	if rng.Intn(100) < chance {
		event.Outcome = AttackOutcomeMissed
		return event
	}

	event.Defender.VitalsChange[model.VitalHP] -= attacker.DerivedAttrs[model.AttrStrength]

	model.ResolveReactions(model.TriggerOnHit, event.Attacker, event.Defender, rng)
	event.Attacker.DecrementAllRelatedCharges(model.TriggerOnHit)

	model.ResolveReactions(model.TriggerOnDamage, event.Defender, event.Attacker, rng)
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

func (event *AttackEvent) Perform(gs *model.GameSession, rng *rand.Rand) {
	if !event.WasPerformed() {
		return
	}

	event.Attacker.Apply(rng)
	event.Defender.Apply(rng)

	attacker := event.Attacker.Actor
	defender := event.Defender.Actor

	// Attacker can be killed by defender in different situations: effects, counter-attacks, special gear
	if attacker.Vitals[model.VitalHP] <= 0 {
		gs.RemoveActor(attacker.Id)
		// Check player's health in main loop (not here)
		if !gs.IsPlayer(attacker.Id) {
			gs.SpawnTreasure(attacker.Pos, calcTreasuresValue(attacker, rng))
		}
	}

	if defender.Vitals[model.VitalHP] <= 0 {
		gs.RemoveActor(defender.Id)
		// Check player's health in main loop (not here)
		if !gs.IsPlayer(defender.Id) {
			gs.SpawnTreasure(defender.Pos, calcTreasuresValue(defender, rng))
		}
	}
}

func calcTreasuresValue(actor *model.Actor, rng *rand.Rand) int {
	value := 0
	value += actor.DerivedAttrs[model.AttrStrength] * 10
	value += actor.DerivedAttrs[model.AttrMaxHP] * 10
	value += actor.DerivedAttrs[model.AttrDexterity] * 5
	value += actor.DerivedAttrs[model.AttrHostility] * 2
	value += rng.Intn(int(float64(value) * 0.2))
	return value
}
