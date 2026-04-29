package service_test

import (
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/conv"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func setupSpawnerEnv(seed int64) (*model.SessionContext, *service.ObjectSpawner) {
	ctx := createTestContext(seed)

	tg := service.NewTopologyGenerator()
	tg.Gen(ctx, 80, 24, 3, 3, 0)
	ctx.Playthrough.DungParams = &model.DungParams{TreasureValueMultiplier: 1.0}

	rules := &model.GameRules{
		ItemsConf: map[model.ItemLabel]*model.Item{
			"Steak": {Kind: model.ItemTypeFood, Label: "Steak"},
		},
		ActorsConf: map[model.ActorLabel]*model.Actor{
			"Ogre": {Kind: model.ActorOgre, Label: "Ogre"},
		},
	}

	spawner := service.NewObjectSpawner(rules)
	return ctx, spawner
}

func TestObjectSpawnerGenerateLimits(t *testing.T) {
	itemWeights := []conv.KV[model.ItemLabel, int]{{Key: "Steak", Val: 100}}
	actorWeights := []conv.KV[model.ActorLabel, int]{{Key: "Ogre", Val: 100}}

	for i := 0; i < 100; i++ {
		seed := time.Now().UnixNano() + int64(i)
		ctx, spawner := setupSpawnerEnv(seed)
		m := ctx.Playthrough.Map

		totalItemCapacity := 0
		totalActorCapacity := 0
		for _, room := range m.GetRooms() {
			if room.Id != m.EntranceRoomId {
				totalItemCapacity += room.GetItemCapacity()
				totalActorCapacity += room.GetActorCapacity()
			}
		}

		t.Run("Normal Generation", func(t *testing.T) {
			spawner.GenerateItems(ctx, 3, itemWeights)
			spawner.GenerateMonsters(ctx, 3, actorWeights)

			if len(ctx.Playthrough.Items) != 3 {
				t.Errorf("Seed %v: Expected 3 items, got %d", seed, len(ctx.Playthrough.Items))
			}
			if len(ctx.Playthrough.Monsters) != 3 {
				t.Errorf("Seed %v: Expected 3 monsters, got %d", seed, len(ctx.Playthrough.Monsters))
			}

			spawner.GenerateItems(ctx, 3, itemWeights)
			spawner.GenerateMonsters(ctx, 3, actorWeights)

			if len(ctx.Playthrough.Items) != 6 {
				t.Errorf("Seed %v: Expected 6 items after 2nd gen, got %d", seed, len(ctx.Playthrough.Items))
			}
			if len(ctx.Playthrough.Monsters) != 6 {
				t.Errorf("Seed %v: Expected 6 monsters after 2nd gen, got %d", seed, len(ctx.Playthrough.Monsters))
			}
		})

		t.Run("Zero and Negative Generation", func(t *testing.T) {
			spawner.GenerateItems(ctx, -1, itemWeights)
			spawner.GenerateMonsters(ctx, 0, actorWeights)

			if len(ctx.Playthrough.Items) != 6 || len(ctx.Playthrough.Monsters) != 6 {
				t.Errorf("Seed %v: Zero/negative generation should not add objects", seed)
			}
		})

		t.Run("Overflow Generation", func(t *testing.T) {
			hugeNumber := 80 * 24

			currentItemsCount := len(ctx.Playthrough.Items)
			currentActorsCount := len(ctx.Playthrough.Monsters)

			spawner.GenerateItems(ctx, hugeNumber, itemWeights)
			spawner.GenerateMonsters(ctx, hugeNumber, actorWeights)

			expectedItemsTotal := currentItemsCount + (totalItemCapacity - currentItemsCount)
			expectedActorsTotal := currentActorsCount + (totalActorCapacity - currentActorsCount)

			if len(ctx.Playthrough.Items) != expectedItemsTotal {
				t.Errorf("Seed %v: Overflow gen expected %d items (max capacity), got %d",
					seed, expectedItemsTotal, len(ctx.Playthrough.Items))
			}

			if len(ctx.Playthrough.Monsters) != expectedActorsTotal {
				t.Errorf("Seed %v: Overflow gen expected %d monsters (max capacity), got %d",
					seed, expectedActorsTotal, len(ctx.Playthrough.Monsters))
			}

			for _, item := range ctx.Playthrough.Items {
				if !m.IsWalkable(item.Pos) {
					t.Errorf("Seed %v: Item %v spawned on non-walkable tile", seed, item.Pos)
				}
			}
		})
	}
}

func TestObjectSpawnerSpawnTreasure(t *testing.T) {
	for i := 0; i < 100; i++ {
		seed := time.Now().UnixNano() + int64(i)
		ctx, spawner := setupSpawnerEnv(seed)
		m := ctx.Playthrough.Map

		var validRoom *model.Room
		for _, r := range m.GetRooms() {
			if r.GetItemCapacity() > 2 && r.Id != m.EntranceRoomId {
				validRoom = r
				break
			}
		}

		if validRoom == nil {
			continue
		}

		center := validRoom.Center
		deadActor := &model.Actor{
			Id:  99,
			Pos: center,
			DerivedAttrs: map[model.AttrType]int{
				model.AttrMaxHP:      10,
				model.AttrMaxStamina: 10,
				model.AttrStrength:   5,
				model.AttrDexterity:  5,
				model.AttrHostility:  1,
			},
		}

		t.Run("SpawnLootSuccess", func(t *testing.T) {
			initialItems := len(ctx.Playthrough.Items)

			spawner.SpawnTreasure(ctx, deadActor)

			if len(ctx.Playthrough.Items) != initialItems+1 {
				t.Errorf("Seed %v: Failed to spawn treasure", seed)
			}

			var treasure *model.Item
			for _, item := range ctx.Playthrough.Items {
				treasure = item
			}

			if treasure == nil {
				t.Fatalf("Seed %v: Treasure is nil", seed)
			}

			if !m.IsItem(treasure.Pos) {
				t.Errorf("Seed %v: Item grid not updated at %v", seed, treasure.Pos)
			}

			if !m.IsWalkable(treasure.Pos) {
				t.Errorf("Seed %v: Loot spawned on non-walkable tile %v", seed, treasure.Pos)
			}
		})

		t.Run("FindLootPointLogic", func(t *testing.T) {
			// Should spawn loot not in the given point
			initialItems := len(ctx.Playthrough.Items)
			m.SetItem(center, 1)
			spawner.SpawnTreasure(ctx, deadActor)

			if len(ctx.Playthrough.Items) != initialItems+1 {
				t.Errorf("Seed %v: Failed to spawn treasure", seed)
			}
		})

		t.Run("ImpossibleToSpawn", func(t *testing.T) {
			for _, p := range validRoom.GetEmptyItemPoints() {
				m.SetItem(p, 1)
			}

			if len(validRoom.GetEmptyItemPoints()) > 0 {
				t.Errorf("Seed: %v\nShould be no available points in the room, got %v available points", seed, len(validRoom.GetEmptyItemPoints()))
			}

			initialItems := len(ctx.Playthrough.Items)
			spawner.SpawnTreasure(ctx, deadActor)
			if len(ctx.Playthrough.Items) == initialItems+1 {
				t.Errorf("Seed %v: Should fail to spawn treasure", seed)
			}
		})

		t.Run("SpawnLootInvalidCenter (Wall)", func(t *testing.T) {
			wallActor := &model.Actor{
				Pos: geometry.Point{X: 0, Y: 0},
			}
			initialItems := len(ctx.Playthrough.Items)
			spawner.SpawnTreasure(ctx, wallActor)

			if len(ctx.Playthrough.Items) != initialItems {
				t.Errorf("Seed %v: Should not spawn loot from non-walkable area without floor near it", seed)
			}
		})
	}
}
