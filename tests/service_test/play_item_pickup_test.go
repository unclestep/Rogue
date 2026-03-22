package service_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/geometry"
)

//
//
// --- HELPERS ---
//
//

func setupPickupEnv() (*model.SessionContext, *service.Pickup, *model.Actor) {
	play := model.NewPlaythrough("", 0, 0)
	actor := model.NewDefaultPlayer(1, geometry.Point{X: 0, Y: 0}, 9)
	play.Players[actor.Id] = actor

	ctx := model.NewSessionContext(play)
	return ctx, service.NewPickupService(), actor
}

func newTestFood(id model.ItemId) *model.Item {
	return model.NewDefaultFood(id, model.NewInvalidPoint())
}

func newTestKey(id model.ItemId) *model.Item {
	return model.NewKeyItem(id, model.NewInvalidPoint(), 1)
}

//
//
// --- PICKUP ---
//
//

//
// -- Nil guards --
//

func TestPickupNilActorReturnsNil(t *testing.T) {
	_, pickup, _ := setupPickupEnv()

	event := pickup.Pickup(nil, newTestFood(1))

	if event != nil {
		t.Errorf("Expected nil event for nil actor, got non-nil")
	}
}

func TestPickupNilItemReturnsNil(t *testing.T) {
	_, pickup, actor := setupPickupEnv()

	event := pickup.Pickup(actor, nil)

	if event != nil {
		t.Errorf("Expected nil event for nil item, got non-nil")
	}
}

//
// -- Normal pickup --
//

func TestPickupWithSpaceAvailableReturnsSuccess(t *testing.T) {
	_, pickup, actor := setupPickupEnv()
	item := newTestFood(1)

	event := pickup.Pickup(actor, item)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.PickupOutcomeSuccess {
		t.Errorf("Expected PickupOutcomeSuccess, got %v", event.Outcome)
	}
	if event.PickupItem != item {
		t.Errorf("Expected PickupItem to be the picked-up item")
	}
}

func TestPickupSuccessDoesNotYetAddItemToBackpack(t *testing.T) {
	_, pickup, actor := setupPickupEnv()
	item := newTestFood(1)

	pickup.Pickup(actor, item)

	// Pickup.Pickup only builds the event; Perform commits the change.
	if len(actor.Backpack.GetSlot(model.ItemTypeFood)) != 0 {
		t.Errorf("Pickup must not add item to backpack before Perform is called")
	}
}

//
// -- Backpack full --
//

func TestPickupBackpackFullReturnsNoFreeSpace(t *testing.T) {
	_, pickup, actor := setupPickupEnv()

	// Fill all 9 food slots.
	for i := 0; i < 9; i++ {
		actor.Backpack.Add(newTestFood(model.ItemId(i + 100)))
	}

	extraFood := newTestFood(200)
	event := pickup.Pickup(actor, extraFood)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.PickupOutcomeBackpackNoFreeSpace {
		t.Errorf("Expected PickupOutcomeBackpackNoFreeSpace when backpack is full, got %v", event.Outcome)
	}
	if event.PickupItem != nil {
		t.Errorf("Expected PickupItem=nil when backpack is full, got non-nil")
	}
}

// Different item types fill separate slots: a full food slot does not block
// picking up a key.
func TestPickupFullFoodSlotDoesNotBlockKeyPickup(t *testing.T) {
	_, pickup, actor := setupPickupEnv()

	for i := 0; i < 9; i++ {
		actor.Backpack.Add(newTestFood(model.ItemId(i + 100)))
	}

	key := newTestKey(200)
	event := pickup.Pickup(actor, key)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.PickupOutcomeSuccess {
		t.Errorf("Expected Success for key when food slot is full, got %v", event.Outcome)
	}
}

//
// -- Treasure --
//

// Treasure items accumulate as value, not slots. CanAddItem must always allow
// treasure regardless of how many were already picked up.
func TestPickupTreasureAlwaysPickable(t *testing.T) {
	_, pickup, actor := setupPickupEnv()

	for i := 0; i < 100; i++ {
		treasure := model.NewTreasureItem(model.ItemId(i+1), model.NewInvalidPoint(), 10)
		event := pickup.Pickup(actor, treasure)
		if event == nil || event.Outcome != service.PickupOutcomeSuccess {
			t.Fatalf("Expected treasure pickup to always succeed, failed at iteration %d", i)
		}
	}
}

//
//
// --- PERFORM ---
//
//

//
// -- Success path --
//

func TestPickupPerformAddsItemToBackpack(t *testing.T) {
	ctx, pickup, actor := setupPickupEnv()

	item := newTestFood(1)
	ctx.Playthrough.Items[item.Id] = item

	event := pickup.Pickup(actor, item)
	if event.Outcome != service.PickupOutcomeSuccess {
		t.Fatalf("Setup: expected Success")
	}

	event.Perform(ctx)

	slot := actor.Backpack.GetSlot(model.ItemTypeFood)
	if len(slot) != 1 {
		t.Errorf("Expected 1 food item in backpack after Perform, got %d", len(slot))
	}
	if slot[0] != item {
		t.Errorf("Expected the picked-up item to be in backpack slot")
	}
}

func TestPickupPerformRemovesItemFromPlaythrough(t *testing.T) {
	ctx, pickup, actor := setupPickupEnv()

	item := newTestFood(1)
	ctx.Playthrough.Items[item.Id] = item

	event := pickup.Pickup(actor, item)
	event.Perform(ctx)

	if _, exists := ctx.Playthrough.Items[item.Id]; !exists {
		t.Errorf("Expected item to stay in Playthrough.Items after pickup, but it was removed")
	}
}

//
// -- Failed path --
//

// When backpack is full, event.PickupItem is nil. Perform must be a safe no-op.
func TestPickupPerformWhenItemIsNilDoesNothing(t *testing.T) {
	ctx, pickup, actor := setupPickupEnv()

	for i := 0; i < 9; i++ {
		actor.Backpack.Add(newTestFood(model.ItemId(i + 100)))
	}

	extraFood := newTestFood(200)
	ctx.Playthrough.Items[extraFood.Id] = extraFood

	event := pickup.Pickup(actor, extraFood)
	if event.Outcome != service.PickupOutcomeBackpackNoFreeSpace {
		t.Fatalf("Setup: expected BackpackNoFreeSpace")
	}

	event.Perform(ctx)

	// Item must still be in the world.
	if _, exists := ctx.Playthrough.Items[extraFood.Id]; !exists {
		t.Errorf("Expected item to remain in Playthrough.Items when pickup failed, but it was removed")
	}
}

//
// -- Perform idempotency --
//

// Calling Perform twice on the same successful event must not panic and must
// not add the item twice (RetrieveById on second call returns nil, backpack
// state remains valid).
func TestPickupPerformCalledTwiceDoesNotDuplicateItem(t *testing.T) {
	ctx, pickup, actor := setupPickupEnv()

	item := newTestFood(1)
	ctx.Playthrough.Items[item.Id] = item

	event := pickup.Pickup(actor, item)
	event.Perform(ctx)
	event.Perform(ctx) // Second call: item already in backpack, should not crash

	slot := actor.Backpack.GetSlot(model.ItemTypeFood)
	if len(slot) != 1 {
		t.Errorf("Expected exactly 1 item after double Perform, got %d", len(slot))
	}
}
