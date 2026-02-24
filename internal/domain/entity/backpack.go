// Package entity Backpack entity implementation
package entity

import (
	"github.com/unclestep/Rogue/internal/pkg/algorithm"
)

type Backpack struct {
	Slots          map[ItemType][]*Item
	TreasuresValue int
}

const (
	MaxBackpackTypeCapacity = 9
)

func (b *Backpack) Add(i *Item) bool {
	if i.Kind == ItemTypeTreasure {
		b.TreasuresValue += i.Value
		return true
	}

	if items, exists := b.Slots[i.Kind]; exists && len(items) == MaxBackpackTypeCapacity {
		return false
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
	if !exists {
		return nil
	}

	last := len(items) - 1
	recent := items[last]
	b.Slots[itemType] = b.Slots[itemType][:last]

	return recent
}
