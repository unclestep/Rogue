package entity_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
)

func TestCanAddItem(t *testing.T) {
	b := model.NewBackpack(9)

	t.Run("Empty backpack", func(t *testing.T) {
		if b.CanAddItem(model.ItemTypeElixir) != true {
			t.Errorf("Should be capable to put an item in empty backpack")
		}
	})

	t.Run("Slot exists", func(t *testing.T) {
		b.Slots[model.ItemTypeElixir] = []*model.Item{
			{
				Kind: model.ItemTypeElixir,
			},
		}

		if b.CanAddItem(model.ItemTypeElixir) != true {
			t.Errorf("Should be capable to put an item in free backpack")
		}
	})

	t.Run("Slot is full", func(t *testing.T) {
		for range 8 {
			b.Slots[model.ItemTypeElixir] = append(b.Slots[model.ItemTypeElixir], &model.Item{
				Kind: model.ItemTypeElixir,
			})
		}

		if b.CanAddItem(model.ItemTypeElixir) != false {
			t.Errorf("Should not be capable to put an item in full backpack")
		}
	})
}

func TestAddItem(t *testing.T) {
	b := model.NewBackpack(9)

	t.Run("Add treasures", func(t *testing.T) {
		b.Add(&model.Item{
			Kind:  model.ItemTypeTreasure,
			Value: 100,
		})

		if b.TreasuresValue != 100 {
			t.Errorf("Treasures should have been added: got %v, expected %v", b.TreasuresValue, 100)
		}

		b.Add(&model.Item{
			Kind:  model.ItemTypeTreasure,
			Value: 23,
		})

		if b.TreasuresValue != 123 {
			t.Errorf("Treasures should have been added: got %v, expected %v", b.TreasuresValue, 123)
		}
	})

	t.Run("Add regular item successfully", func(t *testing.T) {
		item := &model.Item{Id: 1, Kind: model.ItemTypeWeapon}
		success := b.Add(item)

		if !success {
			t.Errorf("Expected to successfully add a weapon to the empty slot")
		}
		if len(b.Slots[model.ItemTypeWeapon]) != 1 {
			t.Errorf("Expected 1 weapon in the slot, got %d", len(b.Slots[model.ItemTypeWeapon]))
		}
	})

	t.Run("Add regular item when slot is full", func(t *testing.T) {
		for i := range 9 {
			b.Add(&model.Item{Id: model.ItemId(10 + i), Kind: model.ItemTypeFood})
		}

		overflowItem := &model.Item{Id: 99, Kind: model.ItemTypeFood}
		success := b.Add(overflowItem)

		if success {
			t.Errorf("Expected Add to return false when slot is full")
		}
		if len(b.Slots[model.ItemTypeFood]) != 9 {
			t.Errorf("Expected capacity to remain %d, got %d", 9, len(b.Slots[model.ItemTypeFood]))
		}
	})
}

func TestRetrieveById(t *testing.T) {
	b := model.NewBackpack(9)

	item1 := &model.Item{Id: 10, Kind: model.ItemTypeScroll}
	item2 := &model.Item{Id: 20, Kind: model.ItemTypeElixir}
	item3 := &model.Item{Id: 30, Kind: model.ItemTypeScroll}

	b.Add(item1)
	b.Add(item2)
	b.Add(item3)

	t.Run("Retrieve existing item", func(t *testing.T) {
		retrieved := b.RetrieveById(10)
		if retrieved == nil || retrieved.Id != 10 {
			t.Errorf("Expected to retrieve item with ID 10")
		}

		if len(b.Slots[model.ItemTypeScroll]) != 1 {
			t.Errorf("Expected 1 scroll left, got %d", len(b.Slots[model.ItemTypeScroll]))
		}

		if b.Slots[model.ItemTypeScroll][0].Id != 30 {
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
	b := model.NewBackpack(9)

	item1 := &model.Item{Id: 100, Kind: model.ItemTypeFood}
	item2 := &model.Item{Id: 200, Kind: model.ItemTypeFood}

	b.Add(item1)
	b.Add(item2)

	t.Run("Retrieve from populated slot", func(t *testing.T) {
		retrieved := b.RetrieveRecent(model.ItemTypeFood)
		if retrieved == nil || retrieved.Id != 200 {
			t.Errorf("Expected to retrieve the most recently added item (ID 200)")
		}

		if len(b.Slots[model.ItemTypeFood]) != 1 {
			t.Errorf("Expected 1 food item left, got %d", len(b.Slots[model.ItemTypeFood]))
		}
	})

	t.Run("Retrieve from empty slot", func(t *testing.T) {
		retrieved := b.RetrieveRecent(model.ItemTypeWeapon)
		if retrieved != nil {
			t.Errorf("Expected nil when retrieving from empty/uninitialized slot")
		}
	})

	t.Run("Retrieve until empty", func(t *testing.T) {
		retrieved := b.RetrieveRecent(model.ItemTypeFood)
		if retrieved == nil || retrieved.Id != 100 {
			t.Errorf("Expected to retrieve ID 100")
		}

		emptyRetrieved := b.RetrieveRecent(model.ItemTypeFood)
		if emptyRetrieved != nil {
			t.Errorf("Expected nil when slot is fully emptied")
		}
	})
}
