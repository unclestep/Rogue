package model

import (
	"maps"
	"math/rand"

	"github.com/unclestep/Rogue/pkg/geometry"
)

type Actor struct {
	Id           ActorId                     `json:"id"`   // Unique identification of actor
	Kind         ActorType                   `json:"kind"` // Actor type, e.g. Player, Vampire, Zombie
	Label        ActorLabel                  `json:"label"`
	MovePattern  MovePatternType             `json:"move_pattern"`
	MoveDir      geometry.Point              `json:"move_dir"` // Move direction; can be changed by moving or turning view direction
	Pos          geometry.Point              `json:"pos"`
	Vitals       map[VitalType]int           `json:"vitals"`        // Non-constant stats, e.g. current hp, stamina
	BaseAttrs    map[AttrType]int            `json:"base_attrs"`    // Base actor's stats: strength, max health etc.
	DerivedAttrs map[AttrType]int            `json:"derived_attrs"` // Computed actor's stats: base attributes + effects + weapon
	Statuses     map[StatusType]int          `json:"statuses"`      // Computed actor's statuses: statuses are granted by effects
	Effects      map[EffectType]*Effect      `json:"effects"`       // Actor effects: permanent (reversible via dispelling) or temporary (expiring by turn count or usage limit). Effects of the same type stack by extending the duration; however, they do not increase or decrease the modified stats further
	EquippedGear map[ItemType]*Item          `json:"equipped_gear"`
	Backpack     *Backpack                   `json:"backpack"`
	Traits       map[TriggerType][]*Reaction `json:"traits"` // What actor does in different situations
	CurState     ActorStateType              `json:"cur_state"`
}

type ActorId int64

//
//
// --- ACTOR TYPES ---
//
//

type ActorType string

const (
	ActorUnknown   ActorType = "unknown"
	ActorPlayer    ActorType = "player"
	ActorZombie    ActorType = "zombie"
	ActorVampire   ActorType = "vampire"
	ActorGhost     ActorType = "ghost"
	ActorOgre      ActorType = "ogre"
	ActorSnakeMage ActorType = "snake_mage"
	ActorMimic     ActorType = "mimic"
)

type ActorLabel string

const (
	ActorLabelUnknown         ActorLabel = "unknown"
	ActorLabelPlayerCommon    ActorLabel = "player_common"
	ActorLabelZombieCommon    ActorLabel = "zombie_common"
	ActorLabelVampireCommon   ActorLabel = "vampire_common"
	ActorLabelGhostCommon     ActorLabel = "ghost_common"
	ActorLabelOgreCommon      ActorLabel = "ogre_common"
	ActorLabelSnakeMageCommon ActorLabel = "snake_mage_common"
	ActorLabelMimicCommon     ActorLabel = "mimic_common"
)

//
//
// --- ATTACK PATTERNS ---
//
//

//go:generate enumer -type=AttackPatternType -json -text -transform=snake_case
type AttackPatternType int

const (
	AttackPatternDefault AttackPatternType = iota
)

//
//
// --- MOVE PATTERNS ---
//
//

//go:generate enumer -type=MovePatternType -json -text -transform=snake_case
type MovePatternType int

const (
	MovePatternDefault MovePatternType = iota
	MovePatternTeleport
	MovePatternDiagonal
)

//
//
// --- VITALS ---
//
//

//go:generate enumer -type=VitalType -json -text -transform=snake_case
type VitalType int

const (
	VitalUnknown VitalType = iota
	VitalHP
	VitalStamina
)

func NewVitals(hp, stamina int) map[VitalType]int {
	return map[VitalType]int{
		VitalHP:      hp,
		VitalStamina: stamina,
	}
}

//
//
// --- ATTRIBUTES ---
//
//

//go:generate enumer -type=AttrType -json -text -transform=snake_case
type AttrType int

const (
	AttrUnknown AttrType = iota
	AttrMaxHP
	AttrMaxStamina
	AttrAttackStaminaCost
	AttrMoveStaminaCost
	AttrActionStaminaCost // e.g. item usage, weapon equip and unequip
	AttrStaminaRegen
	AttrStrength
	AttrDexterity
	AttrHostility
	AttrCounterAttackChance
)

func NewAttrs(attrConf *AttrConf) map[AttrType]int {
	return map[AttrType]int{
		AttrMaxHP:               attrConf.MaxHealth,
		AttrMaxStamina:          attrConf.MaxStamina,
		AttrAttackStaminaCost:   attrConf.AttackStaminaCost,
		AttrMoveStaminaCost:     attrConf.MoveStaminaCost,
		AttrActionStaminaCost:   attrConf.ActionStaminaCost,
		AttrStaminaRegen:        attrConf.StaminaRegen,
		AttrStrength:            attrConf.Strength,
		AttrDexterity:           attrConf.Dexterity,
		AttrHostility:           attrConf.Hostility,
		AttrCounterAttackChance: attrConf.CounterAttackChance,
	}
}

type AttrConf struct {
	MaxHealth           int
	MaxStamina          int
	AttackStaminaCost   int
	MoveStaminaCost     int
	ActionStaminaCost   int
	StaminaRegen        int
	Strength            int
	Dexterity           int
	Hostility           int
	CounterAttackChance int
}

//
// -- ATTRIBUTE CONSTS --
//

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

// Action stamina cost consts
const (
	MediumActionStaminaCost = 100
)

//
//
// --- ACTOR STATES ---
//
//

type ActorStateType int

const (
	ActorStateIdle ActorStateType = iota
	ActorStateWander
	ActorStateChase
)

//
//
// --- GETTERS ---
//
//

func (a *Actor) GetEffectsInfluence() (map[VitalType][]int, map[AttrType][]int) {
	activeEffects := a.CollectActiveEffects()
	vitalsChange := make(map[VitalType][]int, len(a.Vitals))
	attrsChange := make(map[AttrType][]int, len(a.BaseAttrs))

	for _, effect := range activeEffects {
		for vital, change := range effect.VitalsChange {
			if vitalsChange[vital] == nil {
				vitalsChange[vital] = make([]int, 0, 8)
			}
			vitalsChange[vital] = append(vitalsChange[vital], change)
		}

		for attr, change := range effect.AttrsChange {
			if attrsChange[attr] == nil {
				attrsChange[attr] = make([]int, 0, 8)
			}
			attrsChange[attr] = append(attrsChange[attr], change)
		}
	}

	return vitalsChange, attrsChange
}

//
//
// --- CLONE METHODS ---
//
//

func (a *Actor) Clone() *Actor {
	if a == nil {
		return nil
	}

	return &Actor{
		Id:           a.Id,
		Kind:         a.Kind,
		MovePattern:  a.MovePattern,
		Pos:          a.Pos,
		Vitals:       maps.Clone(a.Vitals),
		BaseAttrs:    maps.Clone(a.BaseAttrs),
		DerivedAttrs: maps.Clone(a.DerivedAttrs),
		Statuses:     maps.Clone(a.Statuses),
		Effects:      a.CloneEffects(),
		EquippedGear: a.CloneGear(),
		Backpack:     a.Backpack.Clone(),
		Traits:       a.CloneTraits(),
		CurState:     a.CurState,
	}
}

func (a *Actor) CloneEffects() map[EffectType]*Effect {
	if a.Effects == nil {
		return nil
	}

	clone := make(map[EffectType]*Effect, len(a.Effects))
	for kind, effect := range a.Effects {
		clone[kind] = effect.Clone()
	}

	return clone
}

func (a *Actor) CloneGear() map[ItemType]*Item {
	if a.EquippedGear == nil {
		return nil
	}

	clone := make(map[ItemType]*Item, len(a.EquippedGear))
	for kind, gear := range a.EquippedGear {
		clone[kind] = gear.Clone()
	}

	return clone
}

func (a *Actor) CloneTraits() map[TriggerType][]*Reaction {
	if a.Traits == nil {
		return nil
	}

	clone := make(map[TriggerType][]*Reaction)
	for trigger, reactions := range a.Traits {
		clone[trigger] = make([]*Reaction, 0, len(reactions))
		for _, reaction := range reactions {
			clone[trigger] = append(clone[trigger], reaction.Clone())
		}
	}

	return clone
}

//
//
// --- PREDICATES ---
//
//

func (a *Actor) HasStaminaForMove() bool {
	return a.Vitals[VitalStamina] >= a.DerivedAttrs[AttrMoveStaminaCost]
}

func (a *Actor) HasStaminaForHit() bool {
	return a.Vitals[VitalStamina] >= a.DerivedAttrs[AttrAttackStaminaCost]
}

func (a *Actor) HasStaminaForAction() bool {
	return a.Vitals[VitalStamina] >= a.DerivedAttrs[AttrActionStaminaCost]
}

//
//
// -- ACTOR'S EFFECT RELATED METHODS --
//
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

	if a.Vitals[VitalHP] <= 0 {
		a.Pos = NewInvalidPoint()
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
// -- SETTERS --
//

func (impact *ActorImpact) ResetStamina() {
	impact.VitalsChange[VitalStamina] -= impact.Actor.Vitals[VitalStamina]
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

//
//
// --- REACTION RELATED METHODS ---
//
//

func ResolveReactions(trigger TriggerType, subj *ActorImpact, obj *ActorImpact, rng *rand.Rand) {
	subjReactions := subj.CollectActorReactions(trigger)
	ResolveSpecificReactions(subjReactions, subj, obj, rng)
}

func ResolveSpecificReactions(reactions []*Reaction, subj *ActorImpact, obj *ActorImpact, rng *rand.Rand) {
	for _, reaction := range reactions {
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
}

func (impact *ActorImpact) CollectActorReactions(trigger TriggerType) []*Reaction {
	actor := impact.Actor
	reactions := make([]*Reaction, 0)

	// Innate traits
	reactions = append(reactions, actor.Traits[trigger]...)

	// Gear procs
	for _, gear := range actor.EquippedGear {
		for _, effect := range gear.Effects {
			if _, toRemove := impact.EffectsToRemove[effect]; !toRemove {
				reactions = append(reactions, effect.Procs[trigger]...)
			}
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
		if _, exists := impact.EffectsToRemove[effect]; exists {
			continue
		}

		if effect.ConsumeOn == trigger && effect.Charges != -1 {
			impact.EffectChargesChange[effect] -= 1
			if effect.Charges+impact.EffectChargesChange[effect] == 0 {
				impact.EffectsToRemove[effect] = true
			}
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
	ActionStaminaCost:   MediumActionStaminaCost,
	StaminaRegen:        MediumStamina,
	Strength:            MediumStrength,
	Dexterity:           MediumChance,
	Hostility:           MediumHostility,
	CounterAttackChance: MediumChance,
}

func NewDefaultPlayer(id ActorId, pos geometry.Point, backpackSlotsCapacity int) *Actor {
	return &Actor{
		Id:           id,
		Kind:         ActorPlayer,
		MovePattern:  MovePatternDefault,
		Pos:          pos,
		Vitals:       NewVitals(PlayerDefault.MaxHealth, PlayerDefault.MaxStamina),
		BaseAttrs:    NewAttrs(&PlayerDefault),
		DerivedAttrs: NewAttrs(&PlayerDefault),
		Statuses:     make(map[StatusType]int),
		Effects:      make(map[EffectType]*Effect),
		EquippedGear: make(map[ItemType]*Item),
		Backpack:     NewBackpack(backpackSlotsCapacity),
		Traits:       make(map[TriggerType][]*Reaction),
	}
}

func NewCustomPlayer(id ActorId, pos geometry.Point, backpackSlotsCapacity int, difficulty float64) *Actor {
	p := NewDefaultPlayer(id, pos, backpackSlotsCapacity)
	AdjustStatsByDifficulty(p, difficulty)
	return p
}

func AdjustStatsByDifficulty(actor *Actor, difficulty float64) {
	for vital, val := range actor.Vitals {
		if vital == VitalStamina {
			continue
		}
		actor.Vitals[vital] = int(float64(val) * difficulty)
	}
	for attr, val := range actor.BaseAttrs {
		if attr == AttrMaxStamina || attr == AttrStaminaRegen || attr == AttrAttackStaminaCost || attr == AttrMoveStaminaCost {
			continue
		}
		actor.BaseAttrs[attr] = int(float64(val) * difficulty)
	}
	for attr, val := range actor.DerivedAttrs {
		if attr == AttrMaxStamina || attr == AttrStaminaRegen || attr == AttrAttackStaminaCost || attr == AttrMoveStaminaCost {
			continue
		}
		actor.DerivedAttrs[attr] = int(float64(val) * difficulty)
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
	ActionStaminaCost:   MediumActionStaminaCost,
	StaminaRegen:        MediumStamina,
	Strength:            MediumStrength,
	Dexterity:           LowChance,
	Hostility:           MediumHostility,
	CounterAttackChance: LowChance,
}

func NewDefaultZombie(id ActorId, pos geometry.Point) *Actor {
	return &Actor{
		Id:           id,
		Kind:         ActorZombie,
		MovePattern:  MovePatternDefault,
		Pos:          pos,
		Vitals:       NewVitals(ZombieDefault.MaxHealth, ZombieDefault.MaxStamina),
		BaseAttrs:    NewAttrs(&ZombieDefault),
		DerivedAttrs: NewAttrs(&ZombieDefault),
		Statuses:     make(map[StatusType]int),
		Effects:      make(map[EffectType]*Effect),
		Traits:       make(map[TriggerType][]*Reaction),
	}
}

func NewCustomZombie(id ActorId, pos geometry.Point, difficulty float64) *Actor {
	z := NewDefaultZombie(id, pos)
	AdjustStatsByDifficulty(z, difficulty)
	return z
}

//
// -- VAMPIRE --
//

var VampireDefault = AttrConf{
	MaxHealth:           HighHealth,
	MaxStamina:          MediumStamina,
	AttackStaminaCost:   MediumAttackStaminaCost,
	MoveStaminaCost:     MediumMoveStaminaCost,
	ActionStaminaCost:   MediumActionStaminaCost,
	StaminaRegen:        MediumStamina,
	Strength:            MediumStrength,
	Dexterity:           HighChance,
	Hostility:           HighHostility,
	CounterAttackChance: HighChance,
}

func NewDefaultVampire(id ActorId, pos geometry.Point) *Actor {
	a := &Actor{
		Id:           id,
		Kind:         ActorVampire,
		MovePattern:  MovePatternDefault,
		Pos:          pos,
		Vitals:       NewVitals(VampireDefault.MaxHealth, VampireDefault.MaxStamina),
		BaseAttrs:    NewAttrs(&VampireDefault),
		DerivedAttrs: NewAttrs(&VampireDefault),
		Statuses:     make(map[StatusType]int),
		Effects:      make(map[EffectType]*Effect),
		Traits:       make(map[TriggerType][]*Reaction),
	}

	OnHitOpponent := &Reaction{
		Trigger:         TriggerOnHit,
		Target:          TargetOpponent,
		BaseAttrsChange: make(map[AttrType]Change),
		Chance:          Guaranteed,
	}
	OnHitOpponent.BaseAttrsChange[AttrMaxHP] = Change{Holder: TargetSource, Attr: AttrStrength, Scale: -1.0}

	OnHitSource := &Reaction{
		Trigger:         TriggerOnHit,
		Target:          TargetSource,
		VitalsChange:    make(map[VitalType]Change),
		BaseAttrsChange: make(map[AttrType]Change),
		Chance:          Guaranteed,
	}
	OnHitSource.VitalsChange[VitalHP] = Change{Holder: TargetSource, Attr: AttrStrength, Scale: 1.0}
	OnHitSource.BaseAttrsChange[AttrMaxHP] = Change{Holder: TargetSource, Attr: AttrStrength, Scale: 1.0}

	a.Traits[TriggerOnHit] = append(a.Traits[TriggerOnHit], OnHitOpponent, OnHitSource)
	a.Effects[EffectUntouchable] = NewUntouchableEffect(-1, 1)
	a.RecomputeStats()

	return a
}

func NewCustomVampire(id ActorId, pos geometry.Point, difficulty float64) *Actor {
	v := NewDefaultVampire(id, pos)
	AdjustStatsByDifficulty(v, difficulty)
	return v
}

//
// -- GHOST --
//

var GhostDefault = AttrConf{
	MaxHealth:           LowHealth,
	MaxStamina:          MediumStamina,
	AttackStaminaCost:   MediumAttackStaminaCost,
	MoveStaminaCost:     MediumMoveStaminaCost,
	ActionStaminaCost:   MediumActionStaminaCost,
	StaminaRegen:        MediumStamina,
	Strength:            LowStrength,
	Dexterity:           HighChance,
	Hostility:           LowHostility,
	CounterAttackChance: LowChance,
}

func NewDefaultGhost(id ActorId, pos geometry.Point) *Actor {
	a := &Actor{
		Id:           id,
		Kind:         ActorGhost,
		MovePattern:  MovePatternTeleport,
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

func NewCustomGhost(id ActorId, pos geometry.Point, difficulty float64) *Actor {
	g := NewDefaultGhost(id, pos)
	AdjustStatsByDifficulty(g, difficulty)
	return g
}

//
// -- OGRE --
//

var OgreDefault = AttrConf{
	MaxHealth:           VeryHighHealth,
	MaxStamina:          HighStamina,
	AttackStaminaCost:   MediumAttackStaminaCost,
	MoveStaminaCost:     MediumMoveStaminaCost,
	ActionStaminaCost:   MediumActionStaminaCost,
	StaminaRegen:        HighStamina,
	Strength:            VeryHighStrength,
	Dexterity:           LowChance,
	Hostility:           MediumHostility,
	CounterAttackChance: Guaranteed,
}

var OrgeTraits = map[TriggerType][]*Reaction{
	TriggerOnHit: {
		{
			Trigger: TriggerOnHit,
			Target:  TargetSource,
			Chance:  Guaranteed,
			EffectsToApply: map[EffectType]*Effect{
				EffectFatigue: {
					Kind:     EffectFatigue,
					Duration: 2,
					Charges:  -1,
					StatusesChange: map[StatusType]int{
						StatusFatigue: 1,
					},
					Procs: map[TriggerType][]*Reaction{
						TriggerEffectOnExpire: {
							{
								Trigger: TriggerEffectOnExpire,
								Target:  TargetSource,
								Chance:  Guaranteed,
								EffectsToApply: map[EffectType]*Effect{
									EffectInfallible: {
										Kind:     EffectInfallible,
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
		Kind:         ActorOgre,
		MovePattern:  MovePatternDefault,
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

func NewCustomOgre(id ActorId, pos geometry.Point, difficulty float64) *Actor {
	o := NewDefaultOgre(id, pos)
	AdjustStatsByDifficulty(o, difficulty)
	return o
}

//
// -- SNAKE-MAGE --
//

var SnakeMageDefault = AttrConf{
	MaxHealth:           MediumHealth,
	MaxStamina:          MediumStamina,
	AttackStaminaCost:   MediumAttackStaminaCost,
	MoveStaminaCost:     MediumMoveStaminaCost,
	ActionStaminaCost:   MediumActionStaminaCost,
	StaminaRegen:        MediumStamina,
	Strength:            MediumStrength,
	Dexterity:           VeryHighChance,
	Hostility:           HighHostility,
	CounterAttackChance: MediumChance,
}

func NewDefaultSnakeMage(id ActorId, pos geometry.Point) *Actor {
	a := &Actor{
		Id:           id,
		Kind:         ActorSnakeMage,
		MovePattern:  MovePatternDiagonal,
		Pos:          pos,
		Vitals:       NewVitals(SnakeMageDefault.MaxHealth, SnakeMageDefault.MaxStamina),
		BaseAttrs:    NewAttrs(&SnakeMageDefault),
		DerivedAttrs: NewAttrs(&SnakeMageDefault),
		Statuses:     make(map[StatusType]int),
		Effects:      make(map[EffectType]*Effect),
		Traits:       make(map[TriggerType][]*Reaction),
	}

	OnHit := &Reaction{
		Trigger:        TriggerOnHit,
		Target:         TargetOpponent,
		EffectsToApply: map[EffectType]*Effect{EffectSleep: NewSleepEffect(2)},
		Chance:         MediumChance,
	}

	a.Traits[TriggerOnHit] = append(a.Traits[TriggerOnHit], OnHit)

	return a
}

func NewCustomSnakeMage(id ActorId, pos geometry.Point, difficulty float64) *Actor {
	sm := NewDefaultSnakeMage(id, pos)
	AdjustStatsByDifficulty(sm, difficulty)
	return sm
}

//
// -- MIMIC --
//

var MimicDefault = AttrConf{
	MaxHealth:           HighHealth,
	MaxStamina:          MediumStamina,
	AttackStaminaCost:   MediumAttackStaminaCost,
	MoveStaminaCost:     MediumMoveStaminaCost,
	ActionStaminaCost:   MediumActionStaminaCost,
	StaminaRegen:        MediumStamina,
	Strength:            LowStrength,
	Dexterity:           HighChance,
	Hostility:           VeryLowHostility,
	CounterAttackChance: MediumChance,
}

func NewDefaultMimic(id ActorId, pos geometry.Point) *Actor {
	a := &Actor{
		Id:           id,
		Kind:         ActorMimic,
		MovePattern:  MovePatternDefault,
		Pos:          pos,
		Vitals:       NewVitals(MimicDefault.MaxHealth, MimicDefault.MaxStamina),
		BaseAttrs:    NewAttrs(&MimicDefault),
		DerivedAttrs: NewAttrs(&MimicDefault),
		Statuses:     make(map[StatusType]int),
		Effects:      make(map[EffectType]*Effect),
		Traits:       make(map[TriggerType][]*Reaction),
	}

	a.Effects[EffectSleep] = &Effect{
		Kind:      EffectSleep,
		Duration:  -1,
		Charges:   1,
		ConsumeOn: TriggerOnDamage,
		Procs: map[TriggerType][]*Reaction{
			TriggerOnDamage: {
				{
					Trigger: TriggerOnDamage,
					Target:  TargetOpponent,
					VitalsChange: map[VitalType]Change{
						VitalHP: {
							Holder: TargetOpponent,
							Vital:  VitalHP,
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

func NewCustomMimic(id ActorId, pos geometry.Point, difficulty float64) *Actor {
	m := NewDefaultMimic(id, pos)
	AdjustStatsByDifficulty(m, difficulty)
	return m
}
