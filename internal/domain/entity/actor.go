package entity

import (
	"github.com/unclestep/Rogue/internal/pkg/geometry"
	"math/rand"
)

type Actor struct {
	Id           ActorId   // Unique identification of actor
	Kind         ActorType // Actor type, e.g. Player, Vampire, Zombie
	MovePattern  MovePatternType
	Pos          geometry.Point
	Vitals       map[VitalType]int      // Non-constant stats, e.g. current hp, stamina
	BaseAttrs    map[AttrType]int       // Base actor's stats: strength, max health etc.
	DerivedAttrs map[AttrType]int       // Computed actor's stats: base attributes + effects + weapon
	Statuses     map[StatusType]int     // Computed actor's statuses: statuses are granted by effects
	Effects      map[EffectType]*Effect // Actor effects: permanent (reversible via dispelling) or temporary (expiring by turn count or usage limit). Effects of the same type stack by extending the duration; however, they do not increase or decrease the modified stats further
	EquippedGear map[ItemType]*Item
	Backpack     *Backpack
	Traits       map[TriggerType][]*Reaction // What actor does in different situations
}

type ActorId int

type ActorType int

const (
	PlayerType ActorType = iota
	ZombieType
	VampireType
	GhostType
	OgreType
	SnakeMageType
	MimicType
)

type AttackPatternType int

const (
	DefaultAttackPattern AttackPatternType = iota
)

type MovePatternType int

const (
	DefaultMovePattern MovePatternType = iota
	TeleportMovePattern
	DiagonalMovePattern
	StaticMovePattern
)

type VitalType int

const (
	HP VitalType = iota
	Stamina
)

func NewVitals(hp, stamina int) map[VitalType]int {
	vitals := make(map[VitalType]int)
	vitals[HP] = hp
	vitals[Stamina] = stamina
	return vitals
}

type AttrType int

const (
	MaxHealth AttrType = iota
	MaxStamina
	AttackStaminaCost
	MoveStaminaCost
	StaminaRegen
	Strength
	Dexterity
	Hostility
	CounterAttackChance
)

func NewAttrs(attrConf *AttrConf) map[AttrType]int {
	attrs := make(map[AttrType]int)
	attrs[MaxHealth] = attrConf.MaxHealth
	attrs[MaxStamina] = attrConf.MaxStamina
	attrs[AttackStaminaCost] = attrConf.AttackStaminaCost
	attrs[MoveStaminaCost] = attrConf.MoveStaminaCost
	attrs[StaminaRegen] = attrConf.StaminaRegen
	attrs[Strength] = attrConf.Strength
	attrs[Dexterity] = attrConf.Dexterity
	attrs[Hostility] = attrConf.Hostility
	attrs[CounterAttackChance] = attrConf.CounterAttackChance
	return attrs
}

type AttrConf struct {
	MaxHealth           int
	MaxStamina          int
	AttackStaminaCost   int
	MoveStaminaCost     int
	StaminaRegen        int
	Strength            int
	Dexterity           int
	Hostility           int
	CounterAttackChance int
}

// Health consts
const (
	LowHealth      = 75
	MediumHealth   = 100
	HighHealth     = 125
	VeryHighHealth = 150
)

// Stamina consts
const (
	MediumStamina = 100
	HighStamina   = 200
)

// Strength consts
const (
	LowStrength      = 10
	MediumStrength   = 20
	HighStrength     = 30
	VeryHighStrength = 40
)

// Hostility consts
const (
	Friendly          = 0
	VeryLowHostility  = 1
	LowHostility      = 2
	MediumHostility   = 3
	HighHostility     = 4
	VeryHighHostility = 5
)

// Chance consts
const (
	NoChance       = 0
	LowChance      = 25
	MediumChance   = 50
	HighChance     = 75
	VeryHighChance = 90
	Guaranteed     = 100
)

// Attack stamina cost consts
const (
	MediumAttackStaminaCost = 100
)

// Move stamina cost consts
const (
	MediumMoveStaminaCost = 100
)

//
//
// --- ACTOR IMPACT ---
//
//

type ActorImpact struct {
	Actor               *Actor
	PosChange           geometry.Point
	VitalsChange        map[VitalType]int
	BaseAttrsChange     map[AttrType]int
	StatusesChange      map[StatusType]int
	AppliedEffects      map[EffectType]*Effect
	EffectChargesChange map[*Effect]int
	EffectsToRemove     map[*Effect]bool
}

func NewActorImpact(actor *Actor) *ActorImpact {
	return &ActorImpact{
		Actor:               actor,
		VitalsChange:        make(map[VitalType]int),
		BaseAttrsChange:     make(map[AttrType]int),
		StatusesChange:      make(map[StatusType]int),
		AppliedEffects:      make(map[EffectType]*Effect),
		EffectChargesChange: make(map[*Effect]int),
		EffectsToRemove:     make(map[*Effect]bool),
	}
}

// Apply - applies all changes and removes expired, ran out or marked as needed to remove effects.
func (impact *ActorImpact) Apply() {
	actor := impact.Actor

	for vital, change := range impact.VitalsChange {
		actor.Vitals[vital] += change
	}

	for attr, change := range impact.BaseAttrsChange {
		actor.BaseAttrs[attr] += change
	}

	for status, change := range impact.StatusesChange {
		actor.Statuses[status] += change
	}

	// Add effects first for cases when effect.Charges became <= 0 but there is same effect in AppliedEffects, so we won't delete and add the same effect
	for _, effect := range impact.AppliedEffects {
		actor.AddEffect(effect)
	}

	for effect, change := range impact.EffectChargesChange {
		if effect.Charges != -1 {
			effect.Charges += change
			if effect.Charges <= 0 {
				removed := impact.RemoveSpentEffect(effect)
				if removed {
					effect = nil
				}
			}
		}
	}
}

// RemoveSpentEffect - removes effect which duration equals zero or charges equal zero.
// Recomputes actor's stats automatically if effect was removed.
func (impact *ActorImpact) RemoveSpentEffect(effect *Effect) bool {
	removed := false

	if effect.Duration == 0 || effect.Charges == 0 {
		effectOrigin := impact.whereEffectFrom(effect)
		if effectOrigin != nil {
			removed = impact.removeEffect(effectOrigin, effect)
		}
	}

	if removed {
		impact.Actor.RecomputeStats()
	}

	return removed
}

// whereEffectFrom - finds effect's origin.
// Very likely it is straight from actor's active effects but it also could be from actor's equipped gear.
func (impact *ActorImpact) whereEffectFrom(effect *Effect) map[EffectType]*Effect {
	if _, exists := impact.Actor.Effects[effect.Kind]; exists {
		return impact.Actor.Effects
	}

	for _, gear := range impact.Actor.EquippedGear {
		if _, exists := gear.Effects[effect.Kind]; exists {
			return gear.Effects
		}
	}

	return nil
}

// removeEffect - removes effect from its origin.
// Need to manually recompute actor's stats after this.
func (impact *ActorImpact) removeEffect(origin map[EffectType]*Effect, effect *Effect) bool {
	if origin == nil {
		return false
	}

	removed := false
	if _, exists := origin[effect.Kind]; exists {
		delete(origin, effect.Kind)
		delete(impact.EffectsToRemove, effect)
		removed = true
	}
	return removed
}

// RemoveEffects - removes all effects which were marked as needed to remove.
// These effects do not necessarily expire or run out.
// Recomputes actor's stats automatically if it removed any of effects.
func (impact *ActorImpact) RemoveEffects() {
	needToRecompute := false

	for effect := range impact.EffectsToRemove {
		effectOrigin := impact.whereEffectFrom(effect)
		if effectOrigin != nil {
			needToRecompute = impact.removeEffect(effectOrigin, effect)
			if needToRecompute {
				effect = nil
			}
		}
	}

	if needToRecompute {
		impact.Actor.RecomputeStats()
	}
}

//
// -- ACTOR'S EFFECT RELATED METHODS --
//

// TickEffects - updates duration of all effects related to actor.
// If effects modify something every turn, it will be applied.
// Permanent effects stay permanent, unlimited charges are not spending.
func (a *Actor) TickEffects(rng *rand.Rand) {
	statsChanged := false
	impact := NewActorImpact(a)
	effects := a.CollectActiveEffects()

	for _, effect := range effects {
		ResolveReactions(TriggerEffectOnTurn, impact, impact, rng)
		if effect.IsTemp() {
			effect.ExtendDuration(-1)
		}
		if effect.IsExpired() {
			ResolveReactions(TriggerEffectOnExpire, impact, impact, rng)
			impact.EffectsToRemove[effect] = true
			statsChanged = true
		}
	}

	if statsChanged {
		impact.Apply()
		a.RecomputeStats()
	}
}

// CollectActiveEffects - collects all effects related to actor. It can be its proper effects and effects from its gear.
func (a *Actor) CollectActiveEffects() []*Effect {
	effects := make([]*Effect, 0)
	for _, effect := range a.Effects {
		effects = append(effects, effect)
	}

	for _, gear := range a.EquippedGear {
		for _, effect := range gear.Effects {
			effects = append(effects, effect)
		}
	}

	return effects
}

func (a *Actor) AddEffect(newEffect *Effect) {
	if oldEffect, exists := a.Effects[newEffect.Kind]; exists {
		if !newEffect.IsTemp() {
			oldEffect.MakeInfinite()
		} else {
			oldEffect.ExtendDuration(newEffect.Duration)
		}

		if !newEffect.IsLimited() {
			oldEffect.MakeUnlimited()
		} else {
			oldEffect.AddCharges(newEffect.Charges)
		}
	} else {
		a.Effects[newEffect.Kind] = newEffect
	}

	a.RecomputeStats()
}

func (a *Actor) RecomputeStats() {
	a.ResetAttrs()
	a.ResetStatuses()
	a.ComputeStats()
}

func (a *Actor) ResetAttrs() {
	for attr := range a.BaseAttrs {
		a.DerivedAttrs[attr] = a.BaseAttrs[attr]
	}
}

func (a *Actor) ResetStatuses() {
	for status := range a.Statuses {
		a.Statuses[status] = 0
	}
}

func (a *Actor) ComputeStats() {
	for _, effect := range a.Effects {
		for mod, val := range effect.AttrsChange {
			a.DerivedAttrs[mod] += val
		}

		for status, val := range effect.StatusesChange {
			a.Statuses[status] += val
		}
	}
}

//
//
// --- ACTOR CONSTRUCTORS ---
//
//

//
// -- PLAYER --
//

// Temporary solution. Better decision is using config parser but for now it is ok
var PlayerDefault = AttrConf{
	MaxHealth:           MediumHealth,
	MaxStamina:          MediumStamina,
	AttackStaminaCost:   MediumAttackStaminaCost,
	MoveStaminaCost:     MediumMoveStaminaCost,
	Strength:            MediumStrength,
	Dexterity:           MediumChance,
	Hostility:           MediumHostility,
	CounterAttackChance: MediumChance,
}

func NewDefaultPlayer(id ActorId, pos geometry.Point) *Actor {
	return &Actor{
		Id:           id,
		Kind:         PlayerType,
		MovePattern:  DefaultMovePattern,
		Pos:          pos,
		Vitals:       NewVitals(PlayerDefault.MaxHealth, PlayerDefault.MaxStamina),
		BaseAttrs:    NewAttrs(&PlayerDefault),
		DerivedAttrs: NewAttrs(&PlayerDefault),
		Statuses:     make(map[StatusType]int),
		Effects:      make(map[EffectType]*Effect),
		Traits:       make(map[TriggerType][]*Reaction),
	}
}

//
// -- ZOMBIE --
//

var ZombieDefault = AttrConf{
	MaxHealth:           HighHealth,
	MaxStamina:          MediumStamina,
	AttackStaminaCost:   MediumAttackStaminaCost,
	MoveStaminaCost:     MediumMoveStaminaCost,
	Strength:            MediumStrength,
	Dexterity:           LowChance,
	Hostility:           MediumHostility,
	CounterAttackChance: LowChance,
}

func NewDefaultZombie(id ActorId, pos geometry.Point) *Actor {
	return &Actor{
		Id:           id,
		Kind:         ZombieType,
		MovePattern:  DefaultMovePattern,
		Pos:          pos,
		Vitals:       NewVitals(ZombieDefault.MaxHealth, ZombieDefault.MaxStamina),
		BaseAttrs:    NewAttrs(&ZombieDefault),
		DerivedAttrs: NewAttrs(&ZombieDefault),
		Statuses:     make(map[StatusType]int),
		Effects:      make(map[EffectType]*Effect),
		Traits:       make(map[TriggerType][]*Reaction),
	}
}

//
// -- VAMPIRE --
//

var VampireDefault = AttrConf{
	MaxHealth:           HighHealth,
	MaxStamina:          MediumStamina,
	AttackStaminaCost:   MediumAttackStaminaCost,
	MoveStaminaCost:     MediumMoveStaminaCost,
	Strength:            MediumStrength,
	Dexterity:           HighChance,
	Hostility:           HighHostility,
	CounterAttackChance: HighChance,
}

func NewDefaultVampire(id ActorId, pos geometry.Point) *Actor {
	a := &Actor{
		Id:           id,
		Kind:         VampireType,
		MovePattern:  DefaultMovePattern,
		Pos:          pos,
		Vitals:       NewVitals(VampireDefault.MaxHealth, VampireDefault.MaxStamina),
		BaseAttrs:    NewAttrs(&VampireDefault),
		DerivedAttrs: NewAttrs(&VampireDefault),
		Statuses:     make(map[StatusType]int),
		Effects:      make(map[EffectType]*Effect),
		Traits:       make(map[TriggerType][]*Reaction),
	}

	OnHitOpponent := Reaction{
		Kind:            TriggerOnHit,
		Target:          TargetOpponent,
		BaseAttrsChange: make(map[AttrType]Change),
		Chance:          Guaranteed,
	}
	OnHitOpponent.BaseAttrsChange[MaxHealth] = Change{AttrHolder: TargetSource, Attr: Strength, Scale: -1.0}

	OnHitSource := Reaction{
		Kind:            TriggerOnHit,
		Target:          TargetSource,
		VitalsChange:    make(map[VitalType]Change),
		BaseAttrsChange: make(map[AttrType]Change),
		Chance:          Guaranteed,
	}
	OnHitSource.VitalsChange[HP] = Change{AttrHolder: TargetSource, Attr: Strength, Scale: 1.0}
	OnHitSource.BaseAttrsChange[MaxHealth] = Change{AttrHolder: TargetSource, Attr: Strength, Scale: 1.0}

	a.Traits[TriggerOnHit] = append(a.Traits[TriggerOnHit], &OnHitOpponent, &OnHitSource)
	a.Effects[FirstHitProtectionEffect] = NewFirstHitProtectionEffect()
	a.RecomputeStats()

	return a
}

//
// -- GHOST --
//

var GhostDefault = AttrConf{
	MaxHealth:           LowHealth,
	MaxStamina:          MediumStamina,
	AttackStaminaCost:   MediumAttackStaminaCost,
	MoveStaminaCost:     MediumMoveStaminaCost,
	Strength:            LowStrength,
	Dexterity:           HighChance,
	Hostility:           LowHostility,
	CounterAttackChance: LowChance,
}

func NewDefaultGhost(id ActorId, pos geometry.Point) *Actor {
	a := &Actor{
		Id:           id,
		Kind:         GhostType,
		MovePattern:  TeleportMovePattern,
		Pos:          pos,
		Vitals:       NewVitals(GhostDefault.MaxHealth, GhostDefault.MaxStamina),
		BaseAttrs:    NewAttrs(&GhostDefault),
		DerivedAttrs: NewAttrs(&GhostDefault),
		Statuses:     make(map[StatusType]int),
		Effects:      make(map[EffectType]*Effect),
		Traits:       make(map[TriggerType][]*Reaction),
	}

	return a
}

//
// -- OGRE --
//

var OgreDefault = AttrConf{
	MaxHealth:           VeryHighHealth,
	MaxStamina:          HighStamina,
	AttackStaminaCost:   MediumAttackStaminaCost,
	MoveStaminaCost:     MediumMoveStaminaCost,
	Strength:            VeryHighStrength,
	Dexterity:           LowChance,
	Hostility:           MediumHostility,
	CounterAttackChance: Guaranteed,
}

var OrgeTraits = map[TriggerType][]*Reaction{
	TriggerOnHit: []*Reaction{
		&Reaction{
			Kind:   TriggerOnHit,
			Target: TargetSource,
			Chance: Guaranteed,
			EffectsToApply: []*Effect{
				&Effect{
					Kind:     FatigueEffect,
					Duration: 2,
					Charges:  -1,
					StatusesChange: map[StatusType]int{
						StatusFatigue: 1,
					},
					Procs: map[TriggerType][]*Reaction{
						TriggerEffectOnExpire: []*Reaction{
							&Reaction{
								Kind:   TriggerEffectOnExpire,
								Target: TargetSource,
								Chance: Guaranteed,
								EffectsToApply: []*Effect{
									&Effect{
										Kind:     InfallibleEffect,
										Duration: -1,
										Charges:  1,
										StatusesChange: map[StatusType]int{
											StatusInfallible: 1,
										},
										ConsumeOn: TriggerOnPreHit,
									},
								},
							},
						},
					},
				},
			},
		},
	},
}

func NewDefaultOgre(id ActorId, pos geometry.Point) *Actor {
	a := &Actor{
		Id:           id,
		Kind:         OgreType,
		MovePattern:  DefaultMovePattern,
		Pos:          pos,
		Vitals:       NewVitals(OgreDefault.MaxHealth, OgreDefault.MaxStamina),
		BaseAttrs:    NewAttrs(&OgreDefault),
		DerivedAttrs: NewAttrs(&OgreDefault),
		Statuses:     make(map[StatusType]int),
		Effects:      make(map[EffectType]*Effect),
		Traits:       OrgeTraits,
	}

	return a
}

//
// -- SNAKE-MAGE --
//

var SnakeMageDefault = AttrConf{
	MaxHealth:           MediumHealth,
	MaxStamina:          MediumStamina,
	AttackStaminaCost:   MediumAttackStaminaCost,
	MoveStaminaCost:     MediumMoveStaminaCost,
	Strength:            MediumStrength,
	Dexterity:           VeryHighChance,
	Hostility:           HighHostility,
	CounterAttackChance: MediumChance,
}

func NewDefaultSnakeMage(id ActorId, pos geometry.Point) *Actor {
	a := &Actor{
		Id:           id,
		Kind:         SnakeMageType,
		MovePattern:  DiagonalMovePattern,
		Pos:          pos,
		Vitals:       NewVitals(SnakeMageDefault.MaxHealth, SnakeMageDefault.MaxStamina),
		BaseAttrs:    NewAttrs(&SnakeMageDefault),
		DerivedAttrs: NewAttrs(&SnakeMageDefault),
		Statuses:     make(map[StatusType]int),
		Effects:      make(map[EffectType]*Effect),
		Traits:       make(map[TriggerType][]*Reaction),
	}

	OnHit := Reaction{
		Kind:           TriggerOnHit,
		Target:         TargetOpponent,
		EffectsToApply: []*Effect{NewSleepEffect(2)},
		Chance:         MediumChance,
	}

	a.Traits[TriggerOnHit] = append(a.Traits[TriggerOnHit], &OnHit)

	return a
}

//
// -- MIMIC --
//

var MimicDefault = AttrConf{
	MaxHealth:           HighHealth,
	MaxStamina:          MediumStamina,
	AttackStaminaCost:   MediumAttackStaminaCost,
	MoveStaminaCost:     MediumMoveStaminaCost,
	Strength:            LowStrength,
	Dexterity:           HighChance,
	Hostility:           VeryLowHostility,
	CounterAttackChance: MediumChance,
}

func NewDefaultMimic(id ActorId, pos geometry.Point) *Actor {
	a := &Actor{
		Id:           id,
		Kind:         MimicType,
		MovePattern:  StaticMovePattern,
		Pos:          pos,
		Vitals:       NewVitals(MimicDefault.MaxHealth, MimicDefault.MaxStamina),
		BaseAttrs:    NewAttrs(&MimicDefault),
		DerivedAttrs: NewAttrs(&MimicDefault),
		Statuses:     make(map[StatusType]int),
		Effects:      make(map[EffectType]*Effect),
		Traits:       make(map[TriggerType][]*Reaction),
	}

	return a
}
