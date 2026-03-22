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

func buildItemUsageMap() *model.Map {
	const W, H = 5, 5

	grid := make([][]model.Cell, H)
	for y := range grid {
		grid[y] = make([]model.Cell, W)
		for x := range grid[y] {
			tileType := model.Floor
			if y == 0 || y == H-1 || x == 0 || x == W-1 {
				tileType = model.Wall
			}
			grid[y][x] = model.Cell{Type: tileType, RoomId: 1}
		}
	}

	blueprint := &model.MapBlueprint{
		Width:    W,
		Height:   H,
		TileGrid: grid,
		Rooms: []*model.Room{
			{
				Id:     1,
				Pos:    geometry.Point{X: 1, Y: 1},
				Width:  W - 2,
				Height: H - 2,
				Center: geometry.Point{X: W / 2, Y: H / 2},
			},
		},
		Doors:          map[geometry.Point]*model.DoorMetadata{},
		EntranceRoomId: 1,
		ExitRoomId:     1,
		ExitPoint:      model.NewInvalidPoint(),
	}

	return model.NewMapFromBlueprint(blueprint)
}

func setupItemUsageEnv() (*model.SessionContext, *service.ItemUsage, *model.Actor) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildItemUsageMap()

	actor := model.NewDefaultPlayer(1, geometry.Point{X: 2, Y: 2}, 9)
	play.Players[actor.Id] = actor
	play.Map.SetActor(actor.Pos, int64(actor.Id))

	ctx := model.NewSessionContext(play)
	return ctx, service.NewItemUsageService(), actor
}

func actorWithNoStamina(actor *model.Actor) {
	actor.Vitals[model.VitalStamina] = 0
}

//
//
// --- CONSUME ITEM ---
//
//

//
// -- Nil guards --
//

func TestConsumeItemNilItemReturnsNil(t *testing.T) {
	_, usage, actor := setupItemUsageEnv()

	event := usage.ConsumeItem(nil, actor)

	if event != nil {
		t.Errorf("Expected nil for nil item, got non-nil event")
	}
}

func TestConsumeItemNilActorReturnsNil(t *testing.T) {
	_, usage, _ := setupItemUsageEnv()

	event := usage.ConsumeItem(model.NewDefaultFood(1, model.NewInvalidPoint()), nil)

	if event != nil {
		t.Errorf("Expected nil for nil actor, got non-nil event")
	}
}

//
// -- Stamina gate --
//

func TestConsumeItemNoStaminaReturnsNoStamina(t *testing.T) {
	_, usage, actor := setupItemUsageEnv()
	actorWithNoStamina(actor)

	event := usage.ConsumeItem(model.NewDefaultFood(1, model.NewInvalidPoint()), actor)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.ItemUsageOutcomeNoStamina {
		t.Errorf("Expected NoStamina, got %v", event.Outcome)
	}
}

//
// -- Impact wiring --
//

func TestConsumeItemFoodAddsHPVitalsChange(t *testing.T) {
	_, usage, actor := setupItemUsageEnv()
	food := model.NewDefaultFood(1, model.NewInvalidPoint())

	event := usage.ConsumeItem(food, actor)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.User.VitalsChange[model.VitalHP] != food.VitalsChange[model.VitalHP] {
		t.Errorf("Expected HP change=%d from food, got %d",
			food.VitalsChange[model.VitalHP],
			event.User.VitalsChange[model.VitalHP])
	}
}

func TestConsumeItemDeductsActionStaminaCostFromVitalsChange(t *testing.T) {
	_, usage, actor := setupItemUsageEnv()
	food := model.NewDefaultFood(1, model.NewInvalidPoint())
	expectedStaminaCost := -actor.DerivedAttrs[model.AttrActionStaminaCost]

	event := usage.ConsumeItem(food, actor)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	// VitalsChange[Stamina] = item.VitalsChange[Stamina] (none for food) - ActionStaminaCost
	if event.User.VitalsChange[model.VitalStamina] != expectedStaminaCost {
		t.Errorf("Expected stamina change=%d, got %d",
			expectedStaminaCost,
			event.User.VitalsChange[model.VitalStamina])
	}
}

func TestConsumeItemScrollAddsBaseAttrsChange(t *testing.T) {
	_, usage, actor := setupItemUsageEnv()
	scroll := model.NewDefaultStrengthScroll(1, model.NewInvalidPoint())

	event := usage.ConsumeItem(scroll, actor)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.User.BaseAttrsChange[model.AttrStrength] != scroll.BaseAttrsChange[model.AttrStrength] {
		t.Errorf("Expected BaseAttrsChange Strength=%d, got %d",
			scroll.BaseAttrsChange[model.AttrStrength],
			event.User.BaseAttrsChange[model.AttrStrength])
	}
}

func TestConsumeItemElixirAddsEffectsToAppliedEffects(t *testing.T) {
	_, usage, actor := setupItemUsageEnv()
	elixir := model.NewDefaultDexterityElixir(1, model.NewInvalidPoint())

	event := usage.ConsumeItem(elixir, actor)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if _, exists := event.User.AppliedEffects[model.EffectElixirDexterity]; !exists {
		t.Errorf("Expected elixir effect to be in AppliedEffects after ConsumeItem")
	}
}

func TestConsumeItemSetsRetrievedItemForBackpackRemoval(t *testing.T) {
	_, usage, actor := setupItemUsageEnv()
	food := model.NewDefaultFood(1, model.NewInvalidPoint())

	event := usage.ConsumeItem(food, actor)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.RetrievedItem != food {
		t.Errorf("Expected RetrievedItem to be the consumed item")
	}
}

//
// -- Perform: consume --
//

func TestConsumeItemPerformRemovesItemFromBackpack(t *testing.T) {
	ctx, usage, actor := setupItemUsageEnv()

	food := model.NewDefaultFood(1, model.NewInvalidPoint())
	actor.Backpack.Add(food)

	event := usage.ConsumeItem(food, actor)
	if event == nil {
		t.Fatalf("Expected non-nil event")
	}

	event.Perform(ctx)

	slot := actor.Backpack.GetSlot(model.ItemTypeFood)
	if len(slot) != 0 {
		t.Errorf("Expected food to be removed from backpack after Perform, got %d items", len(slot))
	}
}

func TestConsumeItemPerformAppliesHPChange(t *testing.T) {
	ctx, usage, actor := setupItemUsageEnv()

	food := model.NewDefaultFood(1, model.NewInvalidPoint())
	actor.Backpack.Add(food)
	actor.Vitals[model.VitalHP] = 50
	initialHP := actor.Vitals[model.VitalHP]

	event := usage.ConsumeItem(food, actor)
	event.Perform(ctx)

	if actor.Vitals[model.VitalHP] <= initialHP {
		t.Errorf("Expected HP to increase after consuming food, got HP=%d (was %d)",
			actor.Vitals[model.VitalHP], initialHP)
	}
}

//
//
// --- EQUIP ITEM ---
//
//

//
// -- Nil guards --
//

func TestEquipItemNilItemReturnsNil(t *testing.T) {
	ctx, usage, actor := setupItemUsageEnv()

	event := usage.EquipItem(ctx.Playthrough, nil, actor)

	if event != nil {
		t.Errorf("Expected nil for nil item, got non-nil event")
	}
}

func TestEquipItemNilActorReturnsNil(t *testing.T) {
	ctx, usage, _ := setupItemUsageEnv()

	event := usage.EquipItem(ctx.Playthrough, model.NewDefaultWeapon(1, model.NewInvalidPoint()), nil)

	if event != nil {
		t.Errorf("Expected nil for nil actor, got non-nil event")
	}
}

//
// -- Stamina gate --
//

func TestEquipItemNoStaminaReturnsNoStamina(t *testing.T) {
	ctx, usage, actor := setupItemUsageEnv()
	actorWithNoStamina(actor)

	event := usage.EquipItem(ctx.Playthrough, model.NewDefaultWeapon(1, model.NewInvalidPoint()), actor)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.ItemUsageOutcomeNoStamina {
		t.Errorf("Expected NoStamina, got %v", event.Outcome)
	}
}

//
// -- No existing gear --
//

func TestEquipItemNoExistingGearSetsDroppedItemNil(t *testing.T) {
	ctx, usage, actor := setupItemUsageEnv()
	weapon := model.NewDefaultWeapon(1, model.NewInvalidPoint())
	actor.Backpack.Add(weapon)

	event := usage.EquipItem(ctx.Playthrough, weapon, actor)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.ItemUsageOutcomeSuccess {
		t.Errorf("Expected Success, got %v", event.Outcome)
	}
	if event.DroppedItem != nil {
		t.Errorf("Expected DroppedItem=nil when no previous weapon equipped, got non-nil")
	}
	if event.ItemToEquip != weapon {
		t.Errorf("Expected ItemToEquip to be the new weapon")
	}
}

//
// -- Replacing existing gear --
//

func TestEquipItemReplacingExistingGearSetsDroppedItem(t *testing.T) {
	ctx, usage, actor := setupItemUsageEnv()

	oldWeapon := model.NewDefaultWeapon(1, model.NewInvalidPoint())
	actor.EquippedGear[model.ItemTypeWeapon] = oldWeapon

	newWeapon := model.NewDefaultWeapon(2, model.NewInvalidPoint())
	actor.Backpack.Add(newWeapon)

	event := usage.EquipItem(ctx.Playthrough, newWeapon, actor)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.DroppedItem != oldWeapon {
		t.Errorf("Expected DroppedItem=oldWeapon when replacing gear")
	}
}

//
// -- Perform: equip --
//

func TestEquipItemPerformMovesWeaponToEquippedGear(t *testing.T) {
	ctx, usage, actor := setupItemUsageEnv()

	weapon := model.NewDefaultWeapon(1, model.NewInvalidPoint())
	actor.Backpack.Add(weapon)

	event := usage.EquipItem(ctx.Playthrough, weapon, actor)
	event.Perform(ctx)

	equipped, exists := actor.EquippedGear[model.ItemTypeWeapon]
	if !exists {
		t.Fatalf("Expected weapon in EquippedGear after Perform")
	}
	if equipped != weapon {
		t.Errorf("Expected equipped weapon to be the one from backpack")
	}
}

func TestEquipItemPerformSetsItemPosToInvalid(t *testing.T) {
	ctx, usage, actor := setupItemUsageEnv()

	weapon := model.NewDefaultWeapon(1, geometry.Point{X: 3, Y: 3})
	actor.Backpack.Add(weapon)

	event := usage.EquipItem(ctx.Playthrough, weapon, actor)
	event.Perform(ctx)

	if weapon.Pos != model.NewInvalidPoint() {
		t.Errorf("Expected equipped weapon Pos to be InvalidPoint, got %v", weapon.Pos)
	}
}

func TestEquipItemPerformRemovesWeaponFromBackpack(t *testing.T) {
	ctx, usage, actor := setupItemUsageEnv()

	weapon := model.NewDefaultWeapon(1, model.NewInvalidPoint())
	actor.Backpack.Add(weapon)

	event := usage.EquipItem(ctx.Playthrough, weapon, actor)
	event.Perform(ctx)

	if slot := actor.Backpack.GetSlot(model.ItemTypeWeapon); len(slot) != 0 {
		t.Errorf("Expected weapon to be removed from backpack after equipping, got %d items", len(slot))
	}
}

func TestEquipItemSwapOldWeaponWithInvalidPosIsLost(t *testing.T) {
	ctx, usage, actor := setupItemUsageEnv()

	oldWeapon := model.NewDefaultWeapon(1, model.NewInvalidPoint())
	actor.EquippedGear[model.ItemTypeWeapon] = oldWeapon
	ctx.Playthrough.Items[oldWeapon.Id] = oldWeapon

	newWeapon := model.NewDefaultWeapon(2, model.NewInvalidPoint())
	actor.Backpack.Add(newWeapon)

	event := usage.EquipItem(ctx.Playthrough, newWeapon, actor)
	event.Perform(ctx)

	if w, exists := actor.EquippedGear[oldWeapon.Kind]; exists && oldWeapon == w {
		t.Errorf("Old weapon should have been removed from equipped gear")
	}
	if w, exists := actor.EquippedGear[newWeapon.Kind]; !exists || newWeapon != w {
		t.Errorf("New weapon should be equipped")
	}
	// Old weapon should be dropped to the world so the player can pick it up.
	// With InvalidPoint as drop origin, FindEmptyPoint fails and the item is lost.
	if oldWeapon.Pos == model.NewInvalidPoint() {
		t.Errorf("Fastly unequip weapon should not have position invalid position")
	}
	if !ctx.Playthrough.Map.IsItem(oldWeapon.Pos) {
		t.Errorf("Fastly unequip item should have position on the map")
	}
}

//
//
// --- UNEQUIP ITEM ---
//
//

//
// -- Nil guards --
//

func TestUnequipItemNilItemReturnsNil(t *testing.T) {
	_, usage, actor := setupItemUsageEnv()

	event := usage.UnequipItem(nil, actor)

	if event != nil {
		t.Errorf("Expected nil for nil item, got non-nil event")
	}
}

func TestUnequipItemNilActorReturnsNil(t *testing.T) {
	_, usage, _ := setupItemUsageEnv()

	event := usage.UnequipItem(model.NewDefaultWeapon(1, model.NewInvalidPoint()), nil)

	if event != nil {
		t.Errorf("Expected nil for nil actor, got non-nil event")
	}
}

//
// -- Stamina gate --
//

func TestUnequipItemNoStaminaReturnsNoStamina(t *testing.T) {
	_, usage, actor := setupItemUsageEnv()
	actorWithNoStamina(actor)

	weapon := model.NewDefaultWeapon(1, model.NewInvalidPoint())
	actor.EquippedGear[model.ItemTypeWeapon] = weapon

	event := usage.UnequipItem(weapon, actor)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.ItemUsageOutcomeNoStamina {
		t.Errorf("Expected NoStamina, got %v", event.Outcome)
	}
}

//
// -- Guard: not equipped --
//

func TestUnequipItemNotEquippedReturnsCantUnequip(t *testing.T) {
	_, usage, actor := setupItemUsageEnv()

	weapon := model.NewDefaultWeapon(1, model.NewInvalidPoint())
	// Weapon is NOT in EquippedGear.

	event := usage.UnequipItem(weapon, actor)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.ItemUsageOutcomeCantUnequip {
		t.Errorf("Expected CantUnequip when item is not equipped, got %v", event.Outcome)
	}
}

//
// -- Guard: backpack full --
//

func TestUnequipItemBackpackFullReturnsCantUnequip(t *testing.T) {
	_, usage, actor := setupItemUsageEnv()

	weapon := model.NewDefaultWeapon(1, model.NewInvalidPoint())
	actor.EquippedGear[model.ItemTypeWeapon] = weapon

	// Fill backpack weapon slots to capacity.
	for i := 0; i < 9; i++ {
		actor.Backpack.Add(model.NewDefaultWeapon(model.ItemId(i+100), model.NewInvalidPoint()))
	}

	event := usage.UnequipItem(weapon, actor)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.ItemUsageOutcomeCantUnequip {
		t.Errorf("Expected CantUnequip when backpack is full, got %v", event.Outcome)
	}
}

//
// -- Normal unequip --
//

func TestUnequipItemSuccessSetsItemToUnequip(t *testing.T) {
	_, usage, actor := setupItemUsageEnv()

	weapon := model.NewDefaultWeapon(1, model.NewInvalidPoint())
	actor.EquippedGear[model.ItemTypeWeapon] = weapon

	event := usage.UnequipItem(weapon, actor)

	if event == nil {
		t.Fatalf("Expected non-nil event")
	}
	if event.Outcome != service.ItemUsageOutcomeSuccess {
		t.Errorf("Expected Success, got %v", event.Outcome)
	}
	if event.ItemToUnequip != weapon {
		t.Errorf("Expected ItemToUnequip to be the unequipped weapon")
	}
}

//
// -- Perform: unequip --
//

func TestUnequipItemPerformRemovesFromEquippedGear(t *testing.T) {
	ctx, usage, actor := setupItemUsageEnv()

	weapon := model.NewDefaultWeapon(1, model.NewInvalidPoint())
	actor.EquippedGear[model.ItemTypeWeapon] = weapon

	event := usage.UnequipItem(weapon, actor)
	event.Perform(ctx)

	if _, exists := actor.EquippedGear[model.ItemTypeWeapon]; exists {
		t.Errorf("Expected weapon to be removed from EquippedGear after unequip Perform")
	}
}

func TestUnequipItemPerformAddsItemToBackpack(t *testing.T) {
	ctx, usage, actor := setupItemUsageEnv()

	weapon := model.NewDefaultWeapon(1, model.NewInvalidPoint())
	actor.EquippedGear[model.ItemTypeWeapon] = weapon

	event := usage.UnequipItem(weapon, actor)
	event.Perform(ctx)

	slot := actor.Backpack.GetSlot(model.ItemTypeWeapon)
	if len(slot) != 1 {
		t.Errorf("Expected 1 weapon in backpack after unequip Perform, got %d", len(slot))
	}
	if slot[0] != weapon {
		t.Errorf("Expected the unequipped weapon in backpack slot")
	}
}

// Unequip deducts ActionStaminaCost from stamina. After Perform, actor's
// stamina must be lower by exactly AttrActionStaminaCost.
func TestUnequipItemPerformDeductsActionStaminaCost(t *testing.T) {
	ctx, usage, actor := setupItemUsageEnv()

	weapon := model.NewDefaultWeapon(1, model.NewInvalidPoint())
	actor.EquippedGear[model.ItemTypeWeapon] = weapon

	initialStamina := actor.Vitals[model.VitalStamina]
	staminaCost := actor.DerivedAttrs[model.AttrActionStaminaCost]

	event := usage.UnequipItem(weapon, actor)
	event.Perform(ctx)

	expectedStamina := initialStamina - staminaCost
	if actor.Vitals[model.VitalStamina] != expectedStamina {
		t.Errorf("Expected stamina=%d after unequip, got %d", expectedStamina, actor.Vitals[model.VitalStamina])
	}
}
