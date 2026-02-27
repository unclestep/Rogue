package entities

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/entity"
)

func TestCanAddItem(t *testing.T) {
	b := entity.NewBackpack()

	t.Run("Empty backpack", func(t *testing.T) {
		if b.CanAddItem(entity.ItemTypeElixir) != true {
			t.Errorf("Should be capable to put an item in empty backpack")
		}
	})

	t.Run("Slot exists", func(t *testing.T) {
		b.Slots[entity.ItemTypeElixir] = []*entity.Item{
			{
				Kind: entity.ItemTypeElixir,
			},
		}

		if b.CanAddItem(entity.ItemTypeElixir) != true {
			t.Errorf("Should be capable to put an item in free backpack")
		}
	})

	t.Run("Slot is full", func(t *testing.T) {
		for range 8 {
			b.Slots[entity.ItemTypeElixir] = append(b.Slots[entity.ItemTypeElixir], &entity.Item{
				Kind: entity.ItemTypeElixir,
			})
		}

		if b.CanAddItem(entity.ItemTypeElixir) != false {
			t.Errorf("Should not be capable to put an item in full backpack")
		}
	})
}

func TestAddItem(t *testing.T) {
	b := entity.NewBackpack()

	t.Run("Add treasures", func(t *testing.T) {
		b.Add(&entity.Item{
			Kind:  entity.ItemTypeTreasure,
			Value: 100,
		})

		if b.TreasuresValue != 100 {
			t.Errorf("Treasures should have been added: got %v, expected %v", b.TreasuresValue, 100)
		}

		b.Add(&entity.Item{
			Kind:  entity.ItemTypeTreasure,
			Value: 23,
		})

		if b.TreasuresValue != 123 {
			t.Errorf("Treasures should have been added: got %v, expected %v", b.TreasuresValue, 123)
		}
	})

	t.Run("Add regular item successfully", func(t *testing.T) {
		item := &entity.Item{Id: 1, Kind: entity.ItemTypeWeapon}
		success := b.Add(item)

		if !success {
			t.Errorf("Expected to successfully add a weapon to the empty slot")
		}
		if len(b.Slots[entity.ItemTypeWeapon]) != 1 {
			t.Errorf("Expected 1 weapon in the slot, got %d", len(b.Slots[entity.ItemTypeWeapon]))
		}
	})

	t.Run("Add regular item when slot is full", func(t *testing.T) {
		for i := 0; i < entity.MaxBackpackTypeCapacity; i++ {
			b.Add(&entity.Item{Id: entity.ItemId(10 + i), Kind: entity.ItemTypeFood})
		}

		overflowItem := &entity.Item{Id: 99, Kind: entity.ItemTypeFood}
		success := b.Add(overflowItem)

		if success {
			t.Errorf("Expected Add to return false when slot is full")
		}
		if len(b.Slots[entity.ItemTypeFood]) != entity.MaxBackpackTypeCapacity {
			t.Errorf("Expected capacity to remain %d, got %d", entity.MaxBackpackTypeCapacity, len(b.Slots[entity.ItemTypeFood]))
		}
	})
}

func TestRetrieveById(t *testing.T) {
	b := entity.NewBackpack()

	item1 := &entity.Item{Id: 10, Kind: entity.ItemTypeScroll}
	item2 := &entity.Item{Id: 20, Kind: entity.ItemTypeElixir}
	item3 := &entity.Item{Id: 30, Kind: entity.ItemTypeScroll}

	b.Add(item1)
	b.Add(item2)
	b.Add(item3)

	t.Run("Retrieve existing item", func(t *testing.T) {
		retrieved := b.RetrieveById(10)
		if retrieved == nil || retrieved.Id != 10 {
			t.Errorf("Expected to retrieve item with ID 10")
		}

		if len(b.Slots[entity.ItemTypeScroll]) != 1 {
			t.Errorf("Expected 1 scroll left, got %d", len(b.Slots[entity.ItemTypeScroll]))
		}

		if b.Slots[entity.ItemTypeScroll][0].Id != 30 {
			t.Errorf("Expected the remaining scroll to be ID 30")
		}
	})

	t.Run("Retrieve non-existing item", func(t *testing.T) {
		retrieved := b.RetrieveById(99)
		if retrieved != nil {
			t.Errorf("Expected nil when retrieving non-existing item, got item ID %d", retrieved.Id)
		}
	})
}

func TestRetrieveRecent(t *testing.T) {
	b := entity.NewBackpack()

	item1 := &entity.Item{Id: 100, Kind: entity.ItemTypeFood}
	item2 := &entity.Item{Id: 200, Kind: entity.ItemTypeFood}

	b.Add(item1)
	b.Add(item2)

	t.Run("Retrieve from populated slot", func(t *testing.T) {
		retrieved := b.RetrieveRecent(entity.ItemTypeFood)
		if retrieved == nil || retrieved.Id != 200 {
			t.Errorf("Expected to retrieve the most recently added item (ID 200)")
		}

		if len(b.Slots[entity.ItemTypeFood]) != 1 {
			t.Errorf("Expected 1 food item left, got %d", len(b.Slots[entity.ItemTypeFood]))
		}
	})

	t.Run("Retrieve from empty slot", func(t *testing.T) {
		retrieved := b.RetrieveRecent(entity.ItemTypeWeapon)
		if retrieved != nil {
			t.Errorf("Expected nil when retrieving from empty/uninitialized slot")
		}
	})

	t.Run("Retrieve until empty", func(t *testing.T) {
		retrieved := b.RetrieveRecent(entity.ItemTypeFood)
		if retrieved == nil || retrieved.Id != 100 {
			t.Errorf("Expected to retrieve ID 100")
		}

		emptyRetrieved := b.RetrieveRecent(entity.ItemTypeFood)
		if emptyRetrieved != nil {
			t.Errorf("Expected nil when slot is fully emptied")
		}
	})
}
