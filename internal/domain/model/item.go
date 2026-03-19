// Package model Item model implementation
package model

import (
	"maps"

	"github.com/unclestep/Rogue/pkg/geometry"
)

// Item NOTE: If an item is equipable gear, effects from this item should not be added to actor otherwise they will be taken into account twice
type Item struct {
	Id              ItemId                      `json:"id"`
	Kind            ItemType                    `json:"kind"`
	Label           ItemLabel                   `json:"label"`
	Pos             geometry.Point              `json:"pos"`
	Keyhole         Keyhole                     `json:"keyhole"`
	Value           int                         `json:"value"`
	VitalsChange    map[VitalType]int           `json:"vitals_change"`
	BaseAttrsChange map[AttrType]int            `json:"base_attrs_change"`
	Effects         map[EffectType]*Effect      `json:"effects"` // Ongoing stat modifiers
	Procs           map[TriggerType][]*Reaction `json:"procs"`   // Event-driven triggers
}

type ItemId int64

type ItemType string

const (
	ItemTypeUnknown  ItemType = "unknown"
	ItemTypeTreasure ItemType = "treasure"
	ItemTypeKey      ItemType = "key"
	ItemTypeFood     ItemType = "food"
	ItemTypeElixir   ItemType = "elixir"
	ItemTypeScroll   ItemType = "scroll"
	ItemTypeWeapon   ItemType = "weapon"
)

type ItemLabel string

const (
	ItemLabelUnknown ItemLabel = "unknown"
	// Default items
	ItemLabelDefaultFood     ItemLabel = "default_food"
	ItemLabelDexterityElixir ItemLabel = "dexterity_elixir"
	ItemLabelStrengthElixir  ItemLabel = "strength_elixir"
	ItemLabelMaxHpElixir     ItemLabel = "max_hp_elixir"
	ItemLabelDexterityScroll ItemLabel = "dexterity_scroll"
	ItemLabelStrengthScroll  ItemLabel = "strength_scroll"
	ItemLabelMaxHpScroll     ItemLabel = "max_hp_scroll"
	ItemLabelDefaultWeapon   ItemLabel = "default_weapon"
	// Internal
	ItemLabelDefaultTreasure ItemLabel = "default_treasure"
	ItemLabelKey             ItemLabel = "key"
)

// Keyhole - a lock identifier used for key-to-door matching
type Keyhole int

const (
	KeyholeNone   Keyhole = 0
	MasterKeyhole Keyhole = -1
)

//
// -- CLONE METHODS --
//

func (i *Item) Clone() *Item {
	if i == nil {
		return nil
	}

	return &Item{
		Id:              i.Id,
		Kind:            i.Kind,
		Pos:             i.Pos,
		Value:           i.Value,
		VitalsChange:    maps.Clone(i.VitalsChange),
		BaseAttrsChange: maps.Clone(i.BaseAttrsChange),
		Effects:         i.CloneEffects(),
		Procs:           i.CloneProcs(),
	}
}

func (i *Item) CloneEffects() map[EffectType]*Effect {
	if i.Effects == nil {
		return nil
	}

	clone := make(map[EffectType]*Effect, len(i.Effects))
	for kind, effect := range i.Effects {
		clone[kind] = effect.Clone()
	}

	return clone
}

func (i *Item) CloneProcs() map[TriggerType][]*Reaction {
	if i.Procs == nil {
		return nil
	}

	clone := make(map[TriggerType][]*Reaction)
	for trigger, reactions := range i.Procs {
		clone[trigger] = make([]*Reaction, 0, len(reactions))
		for _, reaction := range reactions {
			clone[trigger] = append(clone[trigger], reaction.Clone())
		}
	}

	return clone
}

//
//
// --- ITEM CONSTRUCTORS ---
//
//

//
// -- CONSTS --
//

//
// -- TREASURE CONSTRUCTORS --
//

func NewTreasureItem(id ItemId, pos geometry.Point, value int) *Item {
	return &Item{
		Id:    id,
		Pos:   pos,
		Kind:  ItemTypeTreasure,
		Label: ItemLabelDefaultTreasure,
		Value: value,
	}
}

//
// -- KEY CONSTRUCTORS --
//

func NewKeyItem(id ItemId, pos geometry.Point, keyhole Keyhole) *Item {
	return &Item{
		Id:      id,
		Pos:     pos,
		Kind:    ItemTypeKey,
		Label:   ItemLabelKey,
		Keyhole: keyhole,
	}
}

//
//
// --- DEFAULTS ---
//
//

//
// -- FOOD CONSTRUCTORS --
//

func NewDefaultFood(id ItemId, pos geometry.Point) *Item {
	return &Item{
		Id:    id,
		Pos:   pos,
		Kind:  ItemTypeFood,
		Label: ItemLabelDefaultFood,
		VitalsChange: map[VitalType]int{
			VitalHP: 30,
		},
	}
}

//
// -- ELIXIR CONSTRUCTORS --
//

func NewDefaultDexterityElixir(id ItemId, pos geometry.Point) *Item {
	return &Item{
		Id:    id,
		Pos:   pos,
		Kind:  ItemTypeElixir,
		Label: ItemLabelDexterityElixir,
		Effects: map[EffectType]*Effect{
			EffectElixirDexterity: {
				Kind:        EffectElixirDexterity,
				Duration:    3,
				Charges:     -1,
				AttrsChange: map[AttrType]int{AttrDexterity: 25},
			},
		},
	}
}

func NewDefaultStrengthElixir(id ItemId, pos geometry.Point) *Item {
	return &Item{
		Id:    id,
		Pos:   pos,
		Kind:  ItemTypeElixir,
		Label: ItemLabelStrengthElixir,
		Effects: map[EffectType]*Effect{
			EffectElixirStrength: {
				Kind:        EffectElixirStrength,
				Duration:    3,
				Charges:     -1,
				AttrsChange: map[AttrType]int{AttrStrength: 25},
			},
		},
	}
}

func NewDefaultMaxHpElixir(id ItemId, pos geometry.Point) *Item {
	return &Item{
		Id:    id,
		Pos:   pos,
		Kind:  ItemTypeElixir,
		Label: ItemLabelMaxHpElixir,
		Effects: map[EffectType]*Effect{
			EffectElixirMaxHP: {
				Kind:        EffectElixirMaxHP,
				Duration:    3,
				Charges:     -1,
				AttrsChange: map[AttrType]int{AttrMaxHP: 25},
			},
		},
	}
}

//
// -- SCROLL CONSTRUCTORS --
//

func NewDefaultDexterityScroll(id ItemId, pos geometry.Point) *Item {
	return &Item{
		Id:    id,
		Pos:   pos,
		Kind:  ItemTypeScroll,
		Label: ItemLabelDexterityScroll,
		BaseAttrsChange: map[AttrType]int{
			AttrDexterity: 3,
		},
	}
}

func NewDefaultStrengthScroll(id ItemId, pos geometry.Point) *Item {
	return &Item{
		Id:   id,
		Pos:  pos,
		Kind: ItemTypeScroll,
		BaseAttrsChange: map[AttrType]int{
			AttrStrength: 5,
		},
	}
}

func NewDefaultMaxHpScroll(id ItemId, pos geometry.Point) *Item {
	return &Item{
		Id:   id,
		Pos:  pos,
		Kind: ItemTypeScroll,
		BaseAttrsChange: map[AttrType]int{
			AttrMaxHP: 10,
		},
	}
}

//
// -- WEAPONS CONSTRUCTORS --
//

func NewDefaultWeapon(id ItemId, pos geometry.Point) *Item {
	return &Item{
		Id:   id,
		Pos:  pos,
		Kind: ItemTypeWeapon,
		Effects: map[EffectType]*Effect{
			EffectWeaponDefault: {
				Kind:     EffectWeaponDefault,
				Duration: -1,
				Charges:  -1,
				AttrsChange: map[AttrType]int{
					AttrStrength: 15,
				},
			},
		},
	}
}
