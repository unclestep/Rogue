package entity

import (
	"log"
	"maps"
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
	AttrsChange    map[AttrType]int   `json:"attrs_change"`    // Affects on the computation of derived attributes: it never modifies base attributes
	StatusesChange map[StatusType]int `json:"statuses_change"` // Can inflict status conditions such as Sleep, Stun, etc.

	// Active bonuses

	Procs     map[TriggerType][]*Reaction `json:"procs"`      // Triggers the special ability logic
	ConsumeOn TriggerType                 `json:"consume_on"` // Situations when we need to decrement the charges
}

//
// -- EFFECT TYPES --
//

//go:generate stringer -type=EffectType
type EffectType int

const (
	EffectNotSpecified EffectType = iota

	//
	// -- BASE EFFECTS --
	//

	// Elixirs
	DexterityBoostElixir
	StrengthBoostElixir
	MaxHealthBoostElixir

	// Weapons
	StrengthBoostWeapon

	//
	// -- COMPLEX EFFECTS --
	//

	FatigueEffect
	UntouchableEffect
	SleepEffect
	InfallibleEffect
)

//
// --- STATUSES ---
//

//go:generate stringer -type=StatusType
type StatusType int

const (
	StatusNotSpecified StatusType = iota
	StatusSleep
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
// -- GETTERS --
//

func (e *Effect) Clone() *Effect {
	if e == nil {
		return nil
	}

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
				clonedReactions[i] = r.Clone()
			}
			clone.Procs[trigger] = clonedReactions
		}
	}

	return clone
}

//
// --- REACTIONS ---
//

//go:generate stringer -type=TriggerType
type TriggerType int

const (
	TriggerNotSpecified TriggerType = iota // What actors do before a combat
	TriggerOnPreHit
	TriggerOnHit    // What actor does when he deals a damage
	TriggerOnDamage // What actor does when he gets a damage
	TriggerOnMove
	TriggerEffectOnExpire
	TriggerEffectOnTurn
)

//go:generate stringer -type=TargetType
type TargetType int

const (
	TargetNotSpecified TargetType = iota
	TargetOpponent
	TargetSource
)

type Reaction struct {
	Kind            TriggerType            `json:"kind"`
	Target          TargetType             `json:"target"`
	VitalsChange    map[VitalType]Change   `json:"vitals_change"`
	BaseAttrsChange map[AttrType]Change    `json:"base_attrs_change"`
	StatusesChange  map[StatusType]int     `json:"statuses_change"`
	EffectsToApply  map[EffectType]*Effect `json:"effects_to_apply"`
	Chance          int                    `json:"chance"`
}

//
// -- CHANGE --
//

type Change struct {
	Holder TargetType `json:"holder"`
	Vital  VitalType  `json:"vital"`
	Attr   AttrType   `json:"attr"`
	Amount int        `json:"amount"`
	Scale  float64    `json:"scale"`
}

// Calc - calculates a change based on struct parameters.
func (c *Change) Calc(source, opponent *Actor) int {
	if c.Vital == NoVital && c.Attr == NoAttr {
		log.Printf("[INFO] Cannot calculate change: Vital and/or Attr is not specified. Change structure:\n%v\n", c)
		return 0
	}

	if c.Holder == TargetNotSpecified {
		log.Printf("[INFO] Cannot calculate change: attribute holder is not specified. Change structure:\n%v\n", c)
		return 0
	}

	holder := source
	if c.Holder == TargetOpponent {
		holder = opponent
	}

	var val int
	if c.Vital == NoVital {
		val = holder.DerivedAttrs[c.Attr]
	} else {
		val = holder.Vitals[c.Vital]
	}

	return c.Amount + int(float64(val)*c.Scale)
}

//
//
// --- REACTION GETTERS ---
//
//

func (r *Reaction) Clone() *Reaction {
	if r == nil {
		return nil
	}

	clone := &Reaction{
		Kind:            r.Kind,
		Target:          r.Target,
		Chance:          r.Chance,
		VitalsChange:    maps.Clone(r.VitalsChange),
		BaseAttrsChange: maps.Clone(r.BaseAttrsChange),
		StatusesChange:  maps.Clone(r.StatusesChange),
	}

	if r.EffectsToApply != nil {
		clone.EffectsToApply = make(map[EffectType]*Effect, len(r.EffectsToApply))
		for kind, eff := range r.EffectsToApply {
			clone.EffectsToApply[kind] = eff.Clone()
		}
	}

	return clone
}

//
//
// --- SETTERS ---
//
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

//
// -- CONSTRUCTORS --
//

func NewUntouchableEffect(duration, charges int) *Effect {
	effect := &Effect{
		Kind:           UntouchableEffect,
		Duration:       duration,
		Charges:        charges,
		StatusesChange: map[StatusType]int{StatusUntouchable: 1},
		ConsumeOn:      TriggerOnPreHit,
	}
	return effect
}

func NewSleepEffect(duration int) *Effect {
	effect := &Effect{
		Kind:     SleepEffect,
		Duration: duration,
		StatusesChange: map[StatusType]int{
			StatusSleep: 1,
		},
	}
	return effect
}

func NewFatigueEffect(duration int) *Effect {
	effect := &Effect{
		Kind:     FatigueEffect,
		Duration: duration,
		StatusesChange: map[StatusType]int{
			StatusFatigue: 1,
		},
	}
	return effect
}

func NewInfallibleEffect(duration, charges int) *Effect {
	effect := &Effect{
		Kind:     InfallibleEffect,
		Duration: duration,
		Charges:  charges,
		AttrsChange: map[AttrType]int{
			CounterAttackChance: Guaranteed,
		},
		StatusesChange: map[StatusType]int{
			StatusInfallible: 1,
		},
		ConsumeOn: TriggerOnPreHit,
	}
	return effect
}
