package entity

import (
	"maps"
	"math/rand"
)

//
//
// --- EFFECT ---
//
//

type Effect struct {
	Kind     EffectType `json:"kind"`
	Duration int        `json:"duration"`
	Charges  int        `json:"charges"`

	// Passive bonuses
	AttrsChange    map[AttrType]int   // Affects on the computation of derived attributes: it never modifies base attributes
	StatusesChange map[StatusType]int // Can inflict status conditions such as Sleep, Stun, etc.

	// Active bonuses: start some iteresting logic maybe :)

	Procs     map[TriggerType][]*Reaction // Triggers the special ability logic
	ConsumeOn TriggerType                 // Situations when we need to decrement the charges
}

//
// -- EFFECT TYPES --
//

type EffectType int

const (
	//
	// -- BASE EFFECTS --
	//

	// Elixirs
	DexterityBoostElixir EffectType = iota
	StrengthBoostElixir
	MaxHealthBoostElixir

	// Weapons
	StrengthBoostWeapon

	//
	// -- COMPLEX EFFECTS --
	//

	FatigueEffect
	FirstHitProtectionEffect
	SleepEffect
	InfallibleEffect
)

//
// --- STATUSES ---
//

type StatusType int

const (
	StatusSleep StatusType = iota
	StatusFirstHitProtection
	StatusFatigue
	StatusBlindness
	StatusUntouchable
	StatusInfallible
)

// Fast checkers

func (a *Actor) CanAttack() bool {
	return a.Statuses[StatusSleep] == 0 && a.Statuses[StatusFatigue] == 0
}

func (a *Actor) CanMove() bool {
	return a.Statuses[StatusSleep] == 0
}

//
// --- REACTIONS ---
//

type TriggerType int

const (
	TriggerNone TriggerType = iota // What actors do before a combat
	TriggerOnPreHit
	TriggerOnHit    // What actor does when he deals a damage
	TriggerOnDamage // What actor does when he gets a damage
	TriggerOnMove
	TriggerEffectOnExpire
	TriggerEffectOnTurn
)

type TargetType int

const (
	TargetOpponent TargetType = iota
	TargetSource
)

type Reaction struct {
	Kind            TriggerType
	Target          TargetType
	VitalsChange    map[VitalType]Change
	BaseAttrsChange map[AttrType]Change
	StatusesChange  map[StatusType]int
	EffectsToApply  map[EffectType]*Effect
	Chance          int
}

//
// -- CHANGE --
//

type Change struct {
	AttrHolder TargetType
	Attr       AttrType
	Amount     int
	Scale      float64
}

// Calc - calculates a change based on struct parameters.
func (c *Change) Calc(source, opponent *Actor) int {
	holder := source
	if c.AttrHolder == TargetOpponent {
		holder = opponent
	}
	return c.Amount + int(float64(holder.DerivedAttrs[c.Attr])*c.Scale)
}

//
// -- REACTION RELATED METHODS --
//

func (r *Reaction) Perform(source, target *ActorImpact) {
	if target.VitalsChange == nil && r.VitalsChange != nil {
		target.VitalsChange = make(map[VitalType]int)
	}

	for vital, change := range r.VitalsChange {
		target.VitalsChange[vital] += change.Calc(source.Actor, target.Actor)
	}

	if target.BaseAttrsChange == nil && r.BaseAttrsChange != nil {
		target.BaseAttrsChange = make(map[AttrType]int)
	}

	for attr, change := range r.BaseAttrsChange {
		target.BaseAttrsChange[attr] += change.Calc(source.Actor, target.Actor)
	}

	if target.StatusesChange == nil && r.StatusesChange != nil {
		target.StatusesChange = make(map[StatusType]int)
	}

	for status, change := range r.StatusesChange {
		target.StatusesChange[status] += change
	}

	if target.AppliedEffects == nil && r.EffectsToApply != nil {
		target.AppliedEffects = make(map[EffectType]*Effect)
	}

	// Deep copy effects to avoid mutating the original templates
	for kind, effect := range r.EffectsToApply {
		target.AppliedEffects[kind] = effect.Clone()
	}

}

func ResolveReactions(trigger TriggerType, subj *ActorImpact, obj *ActorImpact, rng *rand.Rand) {
	subjReactions := subj.CollectActorReactions(trigger)

	for _, reaction := range subjReactions {
		if rng.Intn(Guaranteed) >= reaction.Chance {
			continue
		}

		var source, target *ActorImpact
		if reaction.Target == TargetSource {
			source = subj
			target = subj
		} else {
			if obj == nil {
				continue
			}
			source = subj
			target = obj
		}

		reaction.Perform(source, target)
	}

	subj.DecrementAllRelatedCharges(trigger)
}

func (impact *ActorImpact) CollectActorReactions(trigger TriggerType) []*Reaction {
	actor := impact.Actor
	reactions := make([]*Reaction, 0)

	// Innate traits
	reactions = append(reactions, actor.Traits[trigger]...)

	// Gear procs
	for _, gear := range actor.EquippedGear {
		for _, effect := range gear.Effects {
			reactions = append(reactions, effect.Procs[trigger]...)
		}
		reactions = append(reactions, gear.Procs[trigger]...)
	}

	// Effect procs
	for _, effect := range actor.Effects {
		if _, toRemove := impact.EffectsToRemove[effect]; !toRemove {
			reactions = append(reactions, effect.Procs[trigger]...)
		}
	}

	return reactions

}

func (impact *ActorImpact) DecrementAllRelatedCharges(trigger TriggerType) {
	impact.DecrementEffectsCharges(trigger, impact.Actor.Effects)
	for _, gear := range impact.Actor.EquippedGear {
		impact.DecrementEffectsCharges(trigger, gear.Effects)

	}
}

func (impact *ActorImpact) DecrementEffectsCharges(trigger TriggerType, effects map[EffectType]*Effect) {
	for _, effect := range effects {
		if effect.ConsumeOn == trigger && effect.Charges != -1 {
			impact.EffectChargesChange[effect] -= 1
			if effect.Charges+impact.EffectChargesChange[effect] <= 0 {
				impact.EffectsToRemove[effect] = true
			}
		}
	}
}

//
// -- CONSTRUCTORS --
//

func NewFirstHitProtectionEffect() *Effect {
	effect := &Effect{
		Kind:           FirstHitProtectionEffect,
		Duration:       -1,
		Charges:        1,
		StatusesChange: map[StatusType]int{StatusUntouchable: 1},
		Procs:          make(map[TriggerType][]*Reaction),
	}

	return effect
}

func NewSleepEffect(duration int) *Effect {
	effect := &Effect{
		Kind:           SleepEffect,
		Duration:       duration,
		StatusesChange: make(map[StatusType]int),
	}
	effect.StatusesChange[StatusSleep] = 1
	return effect
}

func NewFatigueEffect(duration int) *Effect {
	effect := &Effect{
		Kind:           FatigueEffect,
		Duration:       duration,
		StatusesChange: make(map[StatusType]int),
	}
	effect.StatusesChange[StatusFatigue] = 1
	return effect
}

func NewInfallibleEffect(duration int) *Effect {
	effect := &Effect{
		Kind:        InfallibleEffect,
		Duration:    duration,
		AttrsChange: make(map[AttrType]int),
	}

	effect.AttrsChange[CounterAttackChance] = Guaranteed

	return effect
}

//
// -- PREDICATES --
//

func (e *Effect) IsTemp() bool {
	return e.Duration != -1
}

func (e *Effect) IsLimited() bool {
	return e.Charges != -1
}

func (e *Effect) IsExpired() bool {
	return e.Duration == 0
}

func (e *Effect) IsRunOut() bool {
	return e.Charges == 0
}

//
// -- SETTERS --
//

func (e *Effect) MakeInfinite() {
	e.Duration = -1
}

func (e *Effect) MakeUnlimited() {
	e.Charges = -1
}

func (e *Effect) ExtendDuration(d int) {
	if e.IsTemp() {
		e.Duration += d
	}
}

func (e *Effect) AddCharges(c int) {
	if e.IsLimited() {
		e.Charges += c
	}
}

//
// -- UTILITIES --
//

func (e *Effect) Clone() *Effect {
	clone := &Effect{
		Kind:           e.Kind,
		Duration:       e.Duration,
		Charges:        e.Charges,
		AttrsChange:    maps.Clone(e.AttrsChange),
		StatusesChange: maps.Clone(e.StatusesChange),
		ConsumeOn:      e.ConsumeOn,
	}

	if e.Procs != nil {
		clone.Procs = make(map[TriggerType][]*Reaction)
		for trigger, reactions := range e.Procs {
			clonedReactions := make([]*Reaction, len(reactions))
			for i, r := range reactions {
				clonedReactions[i] = &Reaction{
					Kind:            r.Kind,
					Target:          r.Target,
					Chance:          r.Chance,
					VitalsChange:    maps.Clone(r.VitalsChange),
					BaseAttrsChange: maps.Clone(r.BaseAttrsChange),
					StatusesChange:  maps.Clone(r.StatusesChange),
				}
				if r.EffectsToApply != nil {
					clonedReactions[i].EffectsToApply = make(map[EffectType]*Effect, len(r.EffectsToApply))
					for kind, eff := range r.EffectsToApply {
						clonedReactions[i].EffectsToApply[kind] = eff.Clone()
					}
				}
			}
			e.Procs[trigger] = clonedReactions
		}
	}

	return clone
}
