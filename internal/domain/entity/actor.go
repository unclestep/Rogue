package entity

import (
	"github.com/unclestep/Rogue/internal/pkg/geometry"
)

type Actor struct {
	Id            int       // Unique identificator of actor
	Kind          ActorType // Actor type, e.g. Player, Vampire, Zombie
	AttackPattern AttackPatternType
	MovePattern   MovePatternType
	Pos           geometry.Point
	Vitals        map[VitalType]int           // Non-constant stats, e.g. current hp, stamina
	BaseAttrs     map[AttrType]int            // Base actor's stats: strength, max health and etc.
	DerivedAttrs  map[AttrType]int            // Computed actor's stats: base attributes + effects + weapon
	Statuses      map[StatusType]int          // Computed actor's statuses: statuses are granted by effects
	Effects       map[EffectType]*Effect      // Actor effects: permanent (reversible via dispelling) or temporary (expiring by turn count or usage limit). Effects of the same type stack by extending the duration; however, they do not increase or decrease the modified stats further
	Traits        map[TriggerType][]*Reaction // What actor does in different situations
}

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
	Guranteed      = 100
)

// Attack stamina cost consts
const (
	MediumAttackStaminaCost = 100
)

// Move stamina cost consts
const (
	MediumMoveStaminaCost = 100
)

type StatusType int

const (
	StatusSleep StatusType = iota
	StatusFirstHitProtection
	StatusFatigue
)

func (a *Actor) CanAttack() bool {
	return a.Statuses[StatusSleep] == 0 && a.Statuses[StatusFatigue] == 0
}

func (a *Actor) CanMove() bool {
	return a.Statuses[StatusSleep] == 0
}

type EffectType int

const (
	FatigueEffect EffectType = iota
	FirstHitProtectionEffect
	SleepEffect
	InfallibleEffect
)

//
//
// --- EFFECT ---
//
//

type Effect struct {
	Kind         EffectType
	Duration     int
	Charges      int
	AttrChange   map[AttrType]int   // Effects only affect on the computation of derived attributes: they never modify base attributes
	StatusChange map[StatusType]int // Can inflict status conditions such as Sleep, Stun, etc.
}

//
// -- CONSTRUCTORS --
//

func NewFirstHitProtectionEffect() *Effect {
	effect := &Effect{
		Kind:         FirstHitProtectionEffect,
		Duration:     -1,
		Charges:      1,
		StatusChange: make(map[StatusType]int),
	}
	effect.StatusChange[StatusFirstHitProtection] = 1
	return effect
}

func NewSleepEffect(duration int) *Effect {
	effect := &Effect{
		Kind:         SleepEffect,
		Duration:     duration,
		StatusChange: make(map[StatusType]int),
	}
	effect.StatusChange[StatusSleep] = 1
	return effect
}

func NewFatigueEffect(duration int) *Effect {
	effect := &Effect{
		Kind:         FatigueEffect,
		Duration:     duration,
		StatusChange: make(map[StatusType]int),
	}
	effect.StatusChange[StatusFatigue] = 1
	return effect
}

func NewInfallibleEffect(duration int) *Effect {
	effect := &Effect{
		Kind:       InfallibleEffect,
		Duration:   duration,
		AttrChange: make(map[AttrType]int),
	}

	effect.AttrChange[CounterAttackChance] = Guranteed

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
// -- ACTOR'S EFFECT RELATED METHODS --
//

func (a *Actor) TickEffects() {
	statsChanged := false

	for _, effect := range a.Effects {
		if effect.IsTemp() {
			effect.ExtendDuration(-1)
			if effect.IsExpired() {
				delete(a.Effects, effect.Kind)
				statsChanged = true
			}
		}
	}

	if statsChanged {
		a.RecomputeStats()
	}
}

func (a *Actor) ConsumeEffect(effectKind EffectType) {
	if effect, exists := a.Effects[effectKind]; exists {
		effect.Charges--
		if effect.IsRunOut() {
			a.RemoveEffect(effectKind)
		}
	}
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

func (a *Actor) RemoveEffect(effectKind EffectType) {
	delete(a.Effects, effectKind)
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
		for mod, val := range effect.AttrChange {
			a.DerivedAttrs[mod] += val
		}

		for status, val := range effect.StatusChange {
			a.Statuses[status] += val
		}
	}
}

//
//
// --- TRAITS ---
//
//

type TriggerType int

const (
	ComputeUsingSourceDerivedAttr = -1
)

const (
	TriggerOnPreHit TriggerType = iota
	TriggerOnHit
	TriggerOnDamage // What actor does when he gets a damage
	TriggerOnMove
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
	EffectsToApply  []*Effect
	Chance          int
}

type Change struct {
	AttrHolder TargetType
	Attr       AttrType
	Scale      float64
}

func (c *Change) Calc(source, opponent *Actor) int {
	holder := source
	if c.AttrHolder == TargetOpponent {
		holder = opponent
	}
	return int(float64(holder.DerivedAttrs[c.Attr]) * c.Scale)
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

func NewDefaultPlayer(id int, pos geometry.Point) *Actor {
	return &Actor{
		Id:            id,
		Kind:          PlayerType,
		AttackPattern: DefaultAttackPattern,
		MovePattern:   DefaultMovePattern,
		Pos:           pos,
		Vitals:        NewVitals(PlayerDefault.MaxHealth, PlayerDefault.MaxStamina),
		BaseAttrs:     NewAttrs(&PlayerDefault),
		DerivedAttrs:  NewAttrs(&PlayerDefault),
		Statuses:      make(map[StatusType]int),
		Effects:       make(map[EffectType]*Effect),
		Traits:        make(map[TriggerType][]*Reaction),
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

func NewDefaultZombie(id int, pos geometry.Point) *Actor {
	return &Actor{
		Id:            id,
		Kind:          ZombieType,
		AttackPattern: DefaultAttackPattern,
		MovePattern:   DefaultMovePattern,
		Pos:           pos,
		Vitals:        NewVitals(ZombieDefault.MaxHealth, ZombieDefault.MaxStamina),
		BaseAttrs:     NewAttrs(&ZombieDefault),
		DerivedAttrs:  NewAttrs(&ZombieDefault),
		Statuses:      make(map[StatusType]int),
		Effects:       make(map[EffectType]*Effect),
		Traits:        make(map[TriggerType][]*Reaction),
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

func NewDefaultVampire(id int, pos geometry.Point) *Actor {
	a := &Actor{
		Id:            id,
		Kind:          VampireType,
		AttackPattern: DefaultAttackPattern,
		MovePattern:   DefaultMovePattern,
		Pos:           pos,
		Vitals:        NewVitals(VampireDefault.MaxHealth, VampireDefault.MaxStamina),
		BaseAttrs:     NewAttrs(&VampireDefault),
		DerivedAttrs:  NewAttrs(&VampireDefault),
		Statuses:      make(map[StatusType]int),
		Effects:       make(map[EffectType]*Effect),
		Traits:        make(map[TriggerType][]*Reaction),
	}

	OnHitOpponent := Reaction{
		Kind:            TriggerOnHit,
		Target:          TargetOpponent,
		BaseAttrsChange: make(map[AttrType]Change),
		Chance:          Guranteed,
	}
	OnHitOpponent.BaseAttrsChange[MaxHealth] = Change{AttrHolder: TargetSource, Attr: Strength, Scale: -1.0}

	OnHitSource := Reaction{
		Kind:            TriggerOnHit,
		Target:          TargetSource,
		VitalsChange:    make(map[VitalType]Change),
		BaseAttrsChange: make(map[AttrType]Change),
		Chance:          Guranteed,
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

func NewDefaultGhost(id int, pos geometry.Point) *Actor {
	a := &Actor{
		Id:            id,
		Kind:          GhostType,
		AttackPattern: DefaultAttackPattern,
		MovePattern:   TeleportMovePattern,
		Pos:           pos,
		Vitals:        NewVitals(GhostDefault.MaxHealth, GhostDefault.MaxStamina),
		BaseAttrs:     NewAttrs(&GhostDefault),
		DerivedAttrs:  NewAttrs(&GhostDefault),
		Statuses:      make(map[StatusType]int),
		Effects:       make(map[EffectType]*Effect),
		Traits:        make(map[TriggerType][]*Reaction),
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
	CounterAttackChance: Guranteed,
}

func NewDefaultOgre(id int, pos geometry.Point) *Actor {
	a := &Actor{
		Id:            id,
		Kind:          OgreType,
		AttackPattern: DefaultAttackPattern,
		MovePattern:   DefaultMovePattern,
		Pos:           pos,
		Vitals:        NewVitals(OgreDefault.MaxHealth, OgreDefault.MaxStamina),
		BaseAttrs:     NewAttrs(&OgreDefault),
		DerivedAttrs:  NewAttrs(&OgreDefault),
		Statuses:      make(map[StatusType]int),
		Effects:       make(map[EffectType]*Effect),
		Traits:        make(map[TriggerType][]*Reaction),
	}

	OnHit := Reaction{
		Kind:           TriggerOnHit,
		Target:         TargetSource,
		EffectsToApply: []*Effect{NewFatigueEffect(2)},
		Chance:         Guranteed,
	}

	a.Traits[TriggerOnHit] = append(a.Traits[TriggerOnHit], &OnHit)

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

func NewDefaultSnakeMage(id int, pos geometry.Point) *Actor {
	a := &Actor{
		Id:            id,
		Kind:          SnakeMageType,
		AttackPattern: DefaultAttackPattern,
		MovePattern:   DiagonalMovePattern,
		Pos:           pos,
		Vitals:        NewVitals(SnakeMageDefault.MaxHealth, SnakeMageDefault.MaxStamina),
		BaseAttrs:     NewAttrs(&SnakeMageDefault),
		DerivedAttrs:  NewAttrs(&SnakeMageDefault),
		Statuses:      make(map[StatusType]int),
		Effects:       make(map[EffectType]*Effect),
		Traits:        make(map[TriggerType][]*Reaction),
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

func NewDefaultMimic(id int, pos geometry.Point) *Actor {
	a := &Actor{
		Id:            id,
		Kind:          MimicType,
		AttackPattern: DefaultAttackPattern,
		MovePattern:   StaticMovePattern,
		Pos:           pos,
		Vitals:        NewVitals(MimicDefault.MaxHealth, MimicDefault.MaxStamina),
		BaseAttrs:     NewAttrs(&MimicDefault),
		DerivedAttrs:  NewAttrs(&MimicDefault),
		Statuses:      make(map[StatusType]int),
		Effects:       make(map[EffectType]*Effect),
		Traits:        make(map[TriggerType][]*Reaction),
	}

	return a
}
