package usecases

import (
	"math/rand"
	"time"

	"github.com/unclestep/Rogue/internal/domain/entity"
)

type CombatService struct {
	session *entity.GameSession
	seed    int64
	rng     *rand.Rand
}

func NewCombatService(session *entity.GameSession) *CombatService {
	cs := &CombatService{session: session, seed: time.Now().UnixNano()}
	cs.rng = rand.New(rand.NewSource(cs.seed))
	return cs
}

func (cs *CombatService) SetSeed(seed int64) {
	cs.seed = seed
	cs.rng = rand.New(rand.NewSource(cs.seed))
}

type AttackEvent struct {
	Attacker *entity.ActorImpact
	Defender *entity.ActorImpact
	Outcome  AttackOutcome
}

type AttackOutcome int

const (
	AttackOutcomeSuccess AttackOutcome = iota
	AttackOutcomeMissed
	AttackOutcomeStatusCantAttack
	AttackOutcomeNoStamina
	AttackOutcomeParticipantIsAlreadyDead
)

func (cs *CombatService) ExecuteAttack(attackerId, defenderId entity.ActorId) *AttackEvent {
	attacker, attackerAlive := cs.session.Actors[attackerId]
	defender, defenderAlive := cs.session.Actors[defenderId]

	event := &AttackEvent{}

	// If one of participants is already dead
	if !attackerAlive || !defenderAlive {
		event.Outcome = AttackOutcomeParticipantIsAlreadyDead
		return event
	}

	event.Attacker = &entity.ActorImpact{Actor: attacker}
	event.Defender = &entity.ActorImpact{Actor: defender}

	if !attacker.CanAttack() {
		event.Outcome = AttackOutcomeStatusCantAttack
		return event
	}

	if attacker.Vitals[entity.Stamina] < attacker.BaseAttrs[entity.AttackStaminaCost] {
		event.Outcome = AttackOutcomeNoStamina
		return event
	}

	event.Attacker.VitalsChange[entity.Stamina] -= attacker.DerivedAttrs[entity.AttackStaminaCost]

	entity.ResolveReactions(entity.TriggerOnPreHit, event.Attacker, event.Defender, cs.rng)
	entity.ResolveReactions(entity.TriggerOnPreHit, event.Defender, event.Attacker, cs.rng)

	chance := calcHitChance(attacker, defender)
	if !cs.session.IsLucky(chance, cs.rng) {
		event.Outcome = AttackOutcomeMissed
		return event
	}

	damage := attacker.DerivedAttrs[entity.Strength]
	if damage > 0 {
		event.Defender.VitalsChange[entity.HP] -= damage
	}

	entity.ResolveReactions(entity.TriggerOnHit, event.Attacker, event.Defender, cs.rng)
	entity.ResolveReactions(entity.TriggerOnDamage, event.Defender, event.Attacker, cs.rng)

	return event
}

func calcHitChance(attacker, defender *entity.Actor) int {
	if defender.Statuses[entity.StatusUntouchable] > 0 {
		return 0
	}

	if attacker.Statuses[entity.StatusInfallible] > 0 {
		return 100
	}

	attackerChance := attacker.DerivedAttrs[entity.Dexterity]
	defenderChance := defender.DerivedAttrs[entity.Dexterity]

	return attackerChance / (attackerChance + defenderChance) * 100
}

func (event *AttackEvent) WasPerformed() bool {
	return event.Outcome == AttackOutcomeSuccess || event.Outcome == AttackOutcomeMissed
}

func (event *AttackEvent) Perform(gs *entity.GameSession) {
	if !event.WasPerformed() {
		return
	}

	event.Attacker.Apply()
	event.Defender.Apply()

	attacker := event.Attacker.Actor
	defender := event.Defender.Actor

	// Attacker can be killed by defender in different situations: effects, counter attacks, special gear
	if attacker.Vitals[entity.HP] <= 0 {
		gs.RemoveActor(attacker.Id)
		// Check player's health in main loop (not here)
		if attacker != gs.Player {
			gs.SpawnTreasures(attacker.Pos, calcTreasuresValue(attacker))

		}
	}

	if defender.Vitals[entity.HP] <= 0 {
		gs.RemoveActor(defender.Id)
		// Check player's health in main loop (not here)
		if attacker != gs.Player {
			gs.SpawnTreasures(defender.Pos, calcTreasuresValue(defender))
		}
	}
}

func calcTreasuresValue(actor *entity.Actor) int {
	value := 0
	value += actor.DerivedAttrs[entity.Strength] * 10
	value += actor.DerivedAttrs[entity.MaxHealth] * 10
	value += actor.DerivedAttrs[entity.Dexterity] * 5
	value += actor.DerivedAttrs[entity.Hostility] * 2
	// value += cs.rng.Intn(int(float64(value) * 0.2))
	return value
}
