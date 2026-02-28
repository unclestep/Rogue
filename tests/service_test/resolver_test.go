package service_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/domain/entity"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func setupResolverEnv() (*entity.GameSession, *service.Resolver, *entity.Actor) {
	session := entity.NewGameSession()

	var seed int64 = 2
	rng := rand.New(rand.NewSource(seed))

	session.Map.GenerateTopology(1, 1, rng)
	center := session.Map.Rooms[0].GetCenter()

	resolver := service.NewResolverService(session, time.Now().UnixNano())
	resolver.SetSeed(seed)

	mover := entity.NewDefaultPlayer(1, center)
	session.Monsters[mover.Id] = mover
	session.Map.SetActor(mover.Pos, int(mover.Id))

	return session, resolver, mover
}

func TestResolveMoveCombatLogic(t *testing.T) {
	session, resolver, player := setupResolverEnv()
	var seed int64 = 2
	rng := rand.New(rand.NewSource(seed))

	targetPos := player.Pos.Add(geometry.Point{X: 1, Y: 0})

	t.Run("Bump to Attack (Player vs Enemy)", func(t *testing.T) {
		player.DerivedAttrs[entity.Dexterity] = 100

		enemy := entity.NewDefaultZombie(2, targetPos)
		enemy.DerivedAttrs[entity.Dexterity] = 0
		session.Monsters[enemy.Id] = enemy
		session.Map.SetActor(targetPos, int(enemy.Id))

		startHP := enemy.Vitals[entity.HP]

		moveEvent := service.NewMoveEvent(player)
		moveEvent.Outcome = service.MoveOutcomeSuccess
		moveEvent.Mover.PosChange = geometry.Point{X: 1, Y: 0}

		event := resolver.ResolveMove(moveEvent)[0]
		event.Perform(session, rng)

		if player.Pos == targetPos {
			t.Error("Player should not move into occupied tile")
		}

		expectedHP := startHP - player.DerivedAttrs[entity.Strength]
		expectedStamina := player.DerivedAttrs[entity.MaxStamina] - player.DerivedAttrs[entity.AttackStaminaCost]

		if enemy.Vitals[entity.HP] != expectedHP {
			t.Errorf("Player should have been attacked an enemy. Expected enemy's health: %v, got: %v\n", expectedHP, enemy.Vitals[entity.HP])
		}
		if player.Vitals[entity.Stamina] != expectedStamina {
			t.Errorf("Player should have been spent stamina. Expected player's stamina: %v, got: %v\n", expectedStamina, player.Vitals[entity.Stamina])
		}
	})

	t.Run("Friendly Fire (Monster vs Monster) must unwork", func(t *testing.T) {
		session.Map.RemoveActor(targetPos)

		mon1 := entity.NewDefaultZombie(3, player.Pos)
		mon2 := entity.NewDefaultZombie(4, targetPos)
		session.Monsters[mon1.Id] = mon1
		session.Monsters[mon2.Id] = mon2
		session.Map.SetActor(mon1.Pos, int(mon1.Id))
		session.Map.SetActor(mon2.Pos, int(mon2.Id))

		moveEvent := service.NewMoveEvent(mon1)
		moveEvent.Outcome = service.MoveOutcomeSuccess
		moveEvent.Mover.PosChange = geometry.Point{X: 1, Y: 0}

		event := resolver.ResolveMove(moveEvent)[0]
		event.Perform(session, rng)

		if mon1.Pos == targetPos {
			t.Error("Monster should not move into another monster")
		}
		if mon2.Vitals[entity.HP] != entity.ZombieDefault.MaxHealth {
			t.Error("Monsters should not attack each other (Friendly Fire)")
		}
	})
}

func TestResolveMovePureMovement(t *testing.T) {
	session, resolver, player := setupResolverEnv()
	var seed int64 = 2
	rng := rand.New(rand.NewSource(seed))

	startPos := player.Pos
	targetPos := startPos.Add(geometry.Point{X: 1, Y: 0})

	session.Map.RemoveActor(targetPos)

	moveEvent := service.NewMoveEvent(player)
	moveEvent.Outcome = service.MoveOutcomeSuccess
	moveEvent.Mover.PosChange = geometry.Point{X: 1, Y: 0}

	event := resolver.ResolveMove(moveEvent)[0]
	event.Perform(session, rng)

	if player.Pos != targetPos {
		t.Errorf("Player should have moved to %v, stayed at %v", targetPos, player.Pos)
	}
	if !session.Map.IsActor(targetPos) {
		t.Error("Map grid should update actor position")
	}
}

func TestResolveMoveItemPickup(t *testing.T) {
	session, resolver, player := setupResolverEnv()
	var seed int64 = 2
	rng := rand.New(rand.NewSource(seed))

	targetPos := player.Pos.Add(geometry.Point{X: 1, Y: 0})

	t.Run("Pick up Potion", func(t *testing.T) {
		item := &entity.Item{
			Id:   100,
			Kind: entity.ItemTypeElixir,
			Pos:  targetPos,
		}
		session.Items[item.Id] = item
		session.Map.SetItem(targetPos, int(item.Id))

		moveEvent := service.NewMoveEvent(player)
		moveEvent.Outcome = service.MoveOutcomeSuccess
		moveEvent.Mover.PosChange = geometry.Point{X: 1, Y: 0}

		events := resolver.ResolveMove(moveEvent)
		for _, event := range events {
			event.Perform(session, rng)
		}

		if player.Pos != targetPos {
			t.Fatal("Player should move to item tile")
		}

		if retrieved := player.Backpack.RetrieveById(item.Id); retrieved == nil {
			t.Error("Item should be in backpack")
		}

		if session.Map.IsItem(targetPos) {
			t.Error("Item should be removed from map")
		}
	})

	t.Run("Pick up Treasure (Auto-consume)", func(t *testing.T) {
		player.Vitals[entity.Stamina] = player.DerivedAttrs[entity.StaminaRegen]
		targetPos2 := player.Pos.Add(geometry.Point{X: 1, Y: 0})

		treasure := entity.NewTreasureItem(200, targetPos2, 500)
		session.Items[treasure.Id] = treasure
		session.Map.SetItem(targetPos2, int(treasure.Id))

		moveEvent := service.NewMoveEvent(player)
		moveEvent.Outcome = service.MoveOutcomeSuccess
		moveEvent.Mover.PosChange = geometry.Point{X: 1, Y: 0}

		events := resolver.ResolveMove(moveEvent)
		for _, event := range events {
			event.Perform(session, rng)
		}

		if player.Backpack.TreasuresValue != 500 {
			t.Errorf("Treasures value expected 500, got %d", player.Backpack.TreasuresValue)
		}

		if session.Map.IsItem(targetPos2) {
			t.Error("Item should be removed from map")
		}

		if _, exists := session.Items[treasure.Id]; exists {
			t.Error("Treasure item should be deleted from session items map")
		}
	})

	t.Run("Backpack Full", func(t *testing.T) {
		targetPos3 := player.Pos.Add(geometry.Point{X: 0, Y: 1})

		for range entity.MaxBackpackTypeCapacity {
			player.Backpack.Add(&entity.Item{Kind: entity.ItemTypeFood})
		}

		food := &entity.Item{Id: 300, Kind: entity.ItemTypeFood, Pos: targetPos3}
		session.Items[food.Id] = food
		session.Map.SetItem(targetPos3, int(food.Id))

		moveEvent := service.NewMoveEvent(player)
		moveEvent.Outcome = service.MoveOutcomeSuccess
		moveEvent.Mover.PosChange = geometry.Point{X: 0, Y: 1}

		events := resolver.ResolveMove(moveEvent)
		for _, event := range events {
			event.Perform(session, rng)
		}

		if player.Pos != targetPos3 {
			t.Error("Player should move even if backpack is full")
		}

		if !session.Map.IsItem(targetPos3) {
			t.Error("Item should remain on map if backpack is full")
		}

		if _, exists := session.Items[food.Id]; !exists {
			t.Error("Food should remain on the map")
		}
	})
}

func TestResolveMoveDesyncFix(t *testing.T) {
	session, resolver, player := setupResolverEnv()
	var seed int64 = 2
	rng := rand.New(rand.NewSource(seed))

	targetPos := player.Pos.Add(geometry.Point{X: 1, Y: 0})

	session.Map.SetActor(targetPos, 99)

	moveEvent := service.NewMoveEvent(player)
	moveEvent.Outcome = service.MoveOutcomeSuccess
	moveEvent.Mover.PosChange = geometry.Point{X: 1, Y: 0}

	event := resolver.ResolveMove(moveEvent)[0]
	event.Perform(session, rng)

	if player.Pos != targetPos {
		t.Error("Player should move after desync fix")
	}
	if id, _ := session.Map.GetActorID(targetPos); id != int(player.Id) {
		t.Errorf("Map should now contain player ID, got %d", id)
	}
}
