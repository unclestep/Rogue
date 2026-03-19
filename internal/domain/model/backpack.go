// Package model Backpack model implementation
package model

import (
	"github.com/unclestep/Rogue/pkg/algorithm"
)

type Backpack struct {
	Slots          map[ItemType][]*Item `json:"slots"`
	TreasuresValue int                  `json:"treasures_values"`
	SlotsCapacity  int                  `json:"slots_capacity"`
}

//
// -- CONSTRUCTORS --
//

func NewBackpack(slotsCapacity int) *Backpack {
	return &Backpack{
		Slots:          make(map[ItemType][]*Item),
		TreasuresValue: 0,
		SlotsCapacity:  slotsCapacity,
	}
}

//
// -- GETTERS --
//

func (b *Backpack) GetSlot(itemType ItemType) []*Item {
	if b == nil || b.Slots == nil {
		return nil
	}

	items, exists := b.Slots[itemType]
	if !exists {
		return nil
	}

	return items
}

//
// -- PREDICATES --
//

func (b *Backpack) CanAddItem(itemType ItemType) bool {
	if b == nil || b.Slots == nil {
		return false
	}

	items, exists := b.Slots[itemType]

	if !exists || len(items) < b.SlotsCapacity {
		return true
	}

	return false
}

//
// -- MUTATORS --
//

func (b *Backpack) Add(i *Item) bool {
	if i.Kind == ItemTypeTreasure {
		b.TreasuresValue += i.Value
		return true
	}

	if items, exists := b.Slots[i.Kind]; exists && len(items) == b.SlotsCapacity {
		return false
	}

	// If new item literally the same (pointer to same memory allocation), do not add item
	for _, item := range b.Slots[i.Kind] {
		if item == i {
			return false
		}
	}

	b.Slots[i.Kind] = append(b.Slots[i.Kind], i)
	return true
}

func (b *Backpack) RetrieveById(itemId ItemId) *Item {
	for kind, items := range b.Slots {
		for i, item := range items {
			if item.Id == itemId {
				b.Slots[kind] = algorithm.RemoveOrderly(items, i)
				return item
			}
		}
	}

	return nil
}

func (b *Backpack) RetrieveRecent(itemType ItemType) *Item {
	items, exists := b.Slots[itemType]
	if !exists || len(items) == 0 {
		return nil
	}

	last := len(items) - 1
	recent := items[last]
	b.Slots[itemType] = b.Slots[itemType][:last]

	return recent
}

//
// -- CLONE METHODS --
//

func (b *Backpack) Clone() *Backpack {
	if b == nil {
		return nil
	}

	clone := &Backpack{
		Slots:          make(map[ItemType][]*Item, len(b.Slots)),
		TreasuresValue: b.TreasuresValue,
		SlotsCapacity:  b.SlotsCapacity,
	}

	for slot, items := range b.Slots {
		clone.Slots[slot] = make([]*Item, 0, len(items))
		for _, item := range items {
			clone.Slots[slot] = append(clone.Slots[slot], item.Clone())
		}
	}

	return clone
}
