// Item entity implemenation
package entity

import (
	"github.com/unclestep/Rogue/internal/pkg/geometry"
	"math/rand"
)

// NOTE: If an item is equipable gear, effects from this item should not be added to actor otherwise they will be taken into account twice
type Item struct {
	Id              ItemId
	Pos             geometry.Point
	Kind            ItemType
	Value           int
	VitalsChange    map[VitalType]int
	BaseAttrsChange map[AttrType]int
	Effects         map[EffectType]*Effect      // Ongoing stat modifiers
	Procs           map[TriggerType][]*Reaction // Event-driven triggers
}

type ItemId int

type ItemType int

const (
	ItemTypeTreasure ItemType = iota
	ItemTypeFood
	ItemTypeElixir
	ItemTypeScroll
	ItemTypeWeapon
)

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
// -- TREASURES CONSTRUCTORS --
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
