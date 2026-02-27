package entity

import (
	"math/rand"

	"github.com/unclestep/Rogue/internal/pkg/geometry"
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
	NotSpecifiedType ActorType = iota
	PlayerType
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
)

type VitalType int

const (
	NoVital VitalType = iota
	HP
	Stamina
)

func NewVitals(hp, stamina int) map[VitalType]int {
	return map[VitalType]int{
		HP:      hp,
		Stamina: stamina,
	}
}

type AttrType int

const (
	NoAttr AttrType = iota
	MaxHealth
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
	return map[AttrType]int{
		MaxHealth:           attrConf.MaxHealth,
		MaxStamina:          attrConf.MaxStamina,
		AttackStaminaCost:   attrConf.AttackStaminaCost,
		MoveStaminaCost:     attrConf.MoveStaminaCost,
		StaminaRegen:        attrConf.StaminaRegen,
		Strength:            attrConf.Strength,
		Dexterity:           attrConf.Dexterity,
		Hostility:           attrConf.Hostility,
		CounterAttackChance: attrConf.CounterAttackChance,
	}
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
// -- PREDICATES --
//

func (a *Actor) HasStaminaForMove() bool {
	return a.Vitals[Stamina] >= a.DerivedAttrs[MoveStaminaCost]
}

func (a *Actor) HasStaminaForHit() bool {
	return a.Vitals[Stamina] >= a.DerivedAttrs[AttackStaminaCost]
}

//
// -- ACTOR'S EFFECT RELATED METHODS --
//

// TickEffects - updates duration of all effects related to actor.
// If effects modify something every turn, it will be applied.
// Permanent effects stay permanent, unlimited charges are not spending.
func (a *Actor) TickEffects(rng *rand.Rand) {
	impact := NewActorImpact(a)
	effects := a.CollectActiveEffects()

	ResolveReactions(TriggerEffectOnTurn, impact, impact, rng)
	impact.DecrementAllRelatedCharges(TriggerEffectOnTurn)

	for _, effect := range effects {
		if effect.IsTemp() {
			effect.ExtendDuration(-1)
		}
		if effect.IsExpired() {
			impact.EffectsToRemove[effect] = true
		}
	}

	impact.Apply(rng)
	a.RecomputeStats()
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

//
// -- GETTERS --
//

func (impact *ActorImpact) GetEffect(effectType EffectType) *Effect {
	personalEffect, hasInPersonal := impact.Actor.Effects[effectType]
	if hasInPersonal {
		return personalEffect
	}
	appliedEffect, hasInApplied := impact.AppliedEffects[effectType]
	if hasInApplied {
		return appliedEffect
	}
	return nil
}

func (impact *ActorImpact) GetStatus(statusType StatusType) (int, *Effect) {
	for _, effect := range impact.Actor.Effects {
		statusChange := effect.StatusesChange[statusType]
		toRemove := impact.EffectsToRemove[effect]
		if statusChange > 0 && !toRemove {
			return statusChange, effect
		}
	}

	for _, effect := range impact.AppliedEffects {
		statusChange, statusExists := effect.StatusesChange[statusType]
		toRemove := impact.EffectsToRemove[effect]
		if statusExists && statusChange > 0 && !toRemove {
			return statusChange, effect
		}
	}

	return 0, nil
}

//
// -- MUTATORS --
//

func (impact *ActorImpact) ConsumeEffect(effect *Effect) {
	if effect.IsLimited() && !impact.EffectsToRemove[effect] {
		impact.EffectChargesChange[effect] -= 1
		if effect.Charges+impact.EffectChargesChange[effect] <= 0 {
			impact.EffectsToRemove[effect] = true
		}
	}
}

// Apply - applies all changes and removes expired, ran out or marked as needed to remove effects.
func (impact *ActorImpact) Apply(rng *rand.Rand) {
	actor := impact.Actor

	// Remove effects before updating the stats to take into account TriggerOnExpire instructions

	// Update effects charges and mark as needed to remove effects whose charges have run out
	for effect, change := range impact.EffectChargesChange {
		if effect.Charges != -1 {
			effect.Charges += change
			if effect.Charges <= 0 {
				impact.EffectsToRemove[effect] = true
			}
		}
	}

	// Resolve trigger on expire before removing
	for effect := range impact.EffectsToRemove {
		if expireReactions, exists := effect.Procs[TriggerEffectOnExpire]; exists {
			ResolveSpecificReactions(expireReactions, impact, impact, rng)
		}
	}

	// Remove all effects marked as needed to remove
	impact.RemoveEffects()

	// Following lines are updating actor's stats

	actor.Pos = actor.Pos.Add(impact.PosChange)

	for vital, change := range impact.VitalsChange {
		actor.Vitals[vital] += change
	}

	for attr, change := range impact.BaseAttrsChange {
		actor.BaseAttrs[attr] += change
	}

	for status, change := range impact.StatusesChange {
		actor.Statuses[status] += change
	}

	// Add applied effects
	for _, effect := range impact.AppliedEffects {
		actor.AddEffect(effect)
	}
}

// whereEffectFrom - finds effect's origin.
// Very likely it is straight from actor's active effects, but it also could be from actor's equipped gear.
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
// NOTE: Need to manually recompute actor's stats after this.
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
// NOTE: Need to manually recompute actor's stats after this.
func (impact *ActorImpact) RemoveEffects() bool {
	removed := false

	for effect := range impact.EffectsToRemove {
		effectOrigin := impact.whereEffectFrom(effect)
		removed = impact.removeEffect(effectOrigin, effect)
	}
	clear(impact.EffectsToRemove)

	return removed
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
	StaminaRegen:        MediumStamina,
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
		EquippedGear: make(map[ItemType]*Item),
		Backpack:     NewBackpack(),
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
	StaminaRegen:        MediumStamina,
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
	StaminaRegen:        MediumStamina,
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

	OnHitOpponent := &Reaction{
		Kind:            TriggerOnHit,
		Target:          TargetOpponent,
		BaseAttrsChange: make(map[AttrType]Change),
		Chance:          Guaranteed,
	}
	OnHitOpponent.BaseAttrsChange[MaxHealth] = Change{Holder: TargetSource, Attr: Strength, Scale: -1.0}

	OnHitSource := &Reaction{
		Kind:            TriggerOnHit,
		Target:          TargetSource,
		VitalsChange:    make(map[VitalType]Change),
		BaseAttrsChange: make(map[AttrType]Change),
		Chance:          Guaranteed,
	}
	OnHitSource.VitalsChange[HP] = Change{Holder: TargetSource, Attr: Strength, Scale: 1.0}
	OnHitSource.BaseAttrsChange[MaxHealth] = Change{Holder: TargetSource, Attr: Strength, Scale: 1.0}

	a.Traits[TriggerOnHit] = append(a.Traits[TriggerOnHit], OnHitOpponent, OnHitSource)
	a.Effects[UntouchableEffect] = NewUntouchableEffect(-1, 1)
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
	StaminaRegen:        MediumStamina,
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
	StaminaRegen:        HighStamina,
	Strength:            VeryHighStrength,
	Dexterity:           LowChance,
	Hostility:           MediumHostility,
	CounterAttackChance: Guaranteed,
}

var OrgeTraits = map[TriggerType][]*Reaction{
	TriggerOnHit: {
		{
			Kind:   TriggerOnHit,
			Target: TargetSource,
			Chance: Guaranteed,
			EffectsToApply: map[EffectType]*Effect{
				FatigueEffect: {
					Kind:     FatigueEffect,
					Duration: 2,
					Charges:  -1,
					StatusesChange: map[StatusType]int{
						StatusFatigue: 1,
					},
					Procs: map[TriggerType][]*Reaction{
						TriggerEffectOnExpire: {
							{
								Kind:   TriggerEffectOnExpire,
								Target: TargetSource,
								Chance: Guaranteed,
								EffectsToApply: map[EffectType]*Effect{
									InfallibleEffect: {
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
	StaminaRegen:        MediumStamina,
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

	OnHit := &Reaction{
		Kind:           TriggerOnHit,
		Target:         TargetOpponent,
		EffectsToApply: map[EffectType]*Effect{SleepEffect: NewSleepEffect(2)},
		Chance:         MediumChance,
	}

	a.Traits[TriggerOnHit] = append(a.Traits[TriggerOnHit], OnHit)

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
	StaminaRegen:        MediumStamina,
	Strength:            LowStrength,
	Dexterity:           HighChance,
	Hostility:           VeryLowHostility,
	CounterAttackChance: MediumChance,
}

func NewDefaultMimic(id ActorId, pos geometry.Point) *Actor {
	a := &Actor{
		Id:           id,
		Kind:         MimicType,
		MovePattern:  DefaultMovePattern,
		Pos:          pos,
		Vitals:       NewVitals(MimicDefault.MaxHealth, MimicDefault.MaxStamina),
		BaseAttrs:    NewAttrs(&MimicDefault),
		DerivedAttrs: NewAttrs(&MimicDefault),
		Statuses:     make(map[StatusType]int),
		Effects:      make(map[EffectType]*Effect),
		Traits:       make(map[TriggerType][]*Reaction),
	}

	a.Effects[SleepEffect] = &Effect{
		Kind:      SleepEffect,
		Duration:  -1,
		Charges:   1,
		ConsumeOn: TriggerOnDamage,
		Procs: map[TriggerType][]*Reaction{
			TriggerOnDamage: {
				{
					Kind:   TriggerOnDamage,
					Target: TargetOpponent,
					VitalsChange: map[VitalType]Change{
						HP: {
							Holder: TargetOpponent,
							Vital:  HP,
							Amount: -1,
							Scale:  1,
						},
					},
				},
			},
		},
	}

	return a
}
