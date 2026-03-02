// Item entity implemenation
package entity

import (
	"github.com/unclestep/Rogue/pkg/geometry"
	"maps"
	"math/rand"
)

// NOTE: If an item is equipable gear, effects from this item should not be added to actor otherwise they will be taken into account twice
type Item struct {
	Id              ItemId                      `json:"id"`
	Kind            ItemType                    `json:"kind"`
	Pos             geometry.Point              `json:"pos"`
	Value           int                         `json:"value"`
	VitalsChange    map[VitalType]int           `json:"vitals_change"`
	BaseAttrsChange map[AttrType]int            `json:"base_attrs_change"`
	Effects         map[EffectType]*Effect      `json:"effects"` // Ongoing stat modifiers
	Procs           map[TriggerType][]*Reaction `json:"procs"`   // Event-driven triggers
}

type ItemId int

type ItemType int

//go:generate stringer -type=ItemType
const (
	ItemTypeNotSpecified ItemType = iota
	ItemTypeTreasure
	ItemTypeKey
	ItemTypeFood
	ItemTypeElixir
	ItemTypeScroll
	ItemTypeWeapon
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

const (
	MinDuration = 1
	MaxDuration = 5

	MinHPRegen = 10
	MaxHPRegen = 50

	MinDexterityBoost = 1
	MaxDexterityBoost = 5

	MinStrengthBoost = 5
	MaxStrengthBoost = 10

	MinMaxHPBoost = 5
	MaxMaxHPBoost = 10
)

//
// -- TREASURE CONSTRUCTORS --
//

func NewTreasureItem(id ItemId, pos geometry.Point, value int) *Item {
	item := &Item{
		Id:   id,
		Pos:  pos,
		Kind: ItemTypeTreasure,
	}
	item.Value = value

	return item
}

//
// -- KEY CONSTRUCTORS --
//

func NewKeyItem(id ItemId, pos geometry.Point) *Item {
	return &Item{
		Id:   id,
		Pos:  pos,
		Kind: ItemTypeKey,
	}
}

//
// -- FOOD CONSTRUCTORS --
//

func NewHPRegenFood(id ItemId, pos geometry.Point, rng *rand.Rand) *Item {
	item := &Item{
		Id:   id,
		Pos:  pos,
		Kind: ItemTypeFood,
	}
	item.VitalsChange = map[VitalType]int{
		HP: MinHPRegen + rng.Intn(MaxHPRegen-MinHPRegen+1),
	}

	return item
}

//
// -- ELIXIR CONSTRUCTORS --
//

func NewDexterityBoostElixir(id ItemId, pos geometry.Point, rng *rand.Rand) *Item {
	item := &Item{
		Id:   id,
		Pos:  pos,
		Kind: ItemTypeElixir,
		Effects: map[EffectType]*Effect{
			DexterityBoostElixir: &Effect{
				Kind:        DexterityBoostElixir,
				Charges:     -1,
				AttrsChange: map[AttrType]int{Dexterity: 25}},
		},
	}
	item.Effects[0].Duration = MinDuration + rng.Intn(MaxDuration)

	return item
}

func NewStrengthBoostElixir(id ItemId, pos geometry.Point, rng *rand.Rand) *Item {
	item := &Item{
		Id:   id,
		Pos:  pos,
		Kind: ItemTypeElixir,
		Effects: map[EffectType]*Effect{
			StrengthBoostElixir: &Effect{
				Kind:        StrengthBoostElixir,
				Charges:     -1,
				AttrsChange: map[AttrType]int{Strength: 25}},
		},
	}
	item.Effects[0].Duration = MinDuration + rng.Intn(MaxDuration)

	return item
}

func NewMaxHealthBoostElixir(id ItemId, pos geometry.Point, rng *rand.Rand) *Item {
	item := &Item{
		Id:   id,
		Pos:  pos,
		Kind: ItemTypeElixir,
		Effects: map[EffectType]*Effect{
			MaxHealthBoostElixir: &Effect{
				Kind:        MaxHealthBoostElixir,
				Charges:     -1,
				AttrsChange: map[AttrType]int{MaxHealth: 25}},
		},
	}
	item.Effects[0].Duration = MinDuration + rng.Intn(MaxDuration)

	return item
}

//
// -- SCROLL CONTRUCTORS --
//

func NewDexterityBoostScroll(id ItemId, pos geometry.Point, rng *rand.Rand) *Item {
	item := &Item{
		Id:   id,
		Pos:  pos,
		Kind: ItemTypeScroll,
	}
	item.BaseAttrsChange = map[AttrType]int{
		Dexterity: MinDexterityBoost + rng.Intn(MaxDexterityBoost-MinDexterityBoost+1),
	}

	return item
}

func NewStrengthBoostScroll(id ItemId, pos geometry.Point, rng *rand.Rand) *Item {
	item := &Item{
		Id:   id,
		Pos:  pos,
		Kind: ItemTypeScroll,
	}
	item.BaseAttrsChange = map[AttrType]int{
		Strength: MinStrengthBoost + rng.Intn(MaxStrengthBoost-MinStrengthBoost+1),
	}

	return item
}

func NewMaxHPBoostScroll(id ItemId, pos geometry.Point, rng *rand.Rand) *Item {
	item := &Item{
		Id:   id,
		Pos:  pos,
		Kind: ItemTypeScroll,
	}
	item.BaseAttrsChange = map[AttrType]int{
		MaxHealth: MinMaxHPBoost + rng.Intn(MaxMaxHPBoost-MinMaxHPBoost+1),
	}

	return item
}

//
// -- WEAPONS CONSTRUCTORS --
//

func NewStrengthBoostWeapon(id ItemId, pos geometry.Point, rng *rand.Rand) *Item {
	item := &Item{
		Id:   id,
		Pos:  pos,
		Kind: ItemTypeWeapon,
		Effects: map[EffectType]*Effect{
			StrengthBoostWeapon: &Effect{
				Kind:     StrengthBoostWeapon,
				Duration: -1,
				Charges:  -1,
			}},
	}
	item.Effects[0].AttrsChange = map[AttrType]int{
		Strength: MinStrengthBoost + rng.Intn(MaxStrengthBoost-MinStrengthBoost+1),
	}

	return item
}
