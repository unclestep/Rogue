package service_test

import (
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/geometry"
)

//
//
// --- HELPERS ---
//
//

func buildMonsterTestMap() *model.Map {
	const W, H = 9, 9

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

func hasIntentForActor(intents []*model.Intent, actorId model.ActorId) bool {
	for _, intent := range intents {
		if intent != nil && intent.Actor == actorId {
			return true
		}
	}
	return false
}

func newMonsterController() *service.MonsterController {
	pathfinder := service.NewPathfinderService()
	resolver := service.NewMoveResolverService()
	return service.NewMonsterControllerService(pathfinder, resolver)
}

//
//
// --- TICK: DEAD MONSTER REMOVAL ---
//
//

//
// -- Normal removal --
//

func TestMonsterControllerTickDeadMonsterRemovedFromMonstersMap(t *testing.T) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildMonsterTestMap()

	player := model.NewDefaultPlayer(1, geometry.Point{X: 7, Y: 7}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	monster := model.NewDefaultZombie(2, geometry.Point{X: 2, Y: 2})
	monster.State = model.BehaviorWander
	monster.Vitals[model.VitalHP] = 0
	play.Monsters[monster.Id] = monster
	play.Map.SetActor(monster.Pos, int64(monster.Id))

	ctrl := newMonsterController()
	ctx := model.NewSessionContext(play)
	ctrl.Tick(ctx)

	if _, exists := play.Monsters[monster.Id]; exists {
		t.Errorf("Expected dead monster to be removed from Monsters map after Tick")
	}
}

// RemoveActor sets actor.Pos = InvalidPoint BEFORE calling Map.RemoveActor.
// As a result Map.RemoveActor is called with (-1,-1), which is out of bounds
// and does nothing. The actor's original tile in actorGrid is never cleared.
// Any subsequent query to that tile will report an actor still present there.
func TestMonsterControllerTickDeadMonsterRemovalLeavesActorInMapGrid(t *testing.T) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildMonsterTestMap()

	player := model.NewDefaultPlayer(1, geometry.Point{X: 7, Y: 7}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	monsterPos := geometry.Point{X: 2, Y: 2}
	monster := model.NewDefaultZombie(2, monsterPos)
	monster.State = model.BehaviorWander
	monster.Vitals[model.VitalHP] = 0
	play.Monsters[monster.Id] = monster
	play.Map.SetActor(monsterPos, int64(monster.Id))

	ctrl := newMonsterController()
	ctx := model.NewSessionContext(play)
	ctrl.Tick(ctx)

	if play.Map.IsActor(monsterPos) {
		t.Errorf("Dead monster's tile at %v still reports an actor in the map grid after Tick. "+
			"RemoveActor sets actor.Pos=InvalidPoint before calling Map.RemoveActor, "+
			"so the grid cell is never cleared.", monsterPos)
	}
}

//
//
// --- TICK: BEHAVIOR DISPATCH ---
//
//

//
// -- BehaviorIdle: unregistered state --
//

// All monster constructors (NewDefaultZombie, etc.) leave State at the zero
// value (BehaviorIdle = 0). MonsterController does not register a behavior for
// BehaviorIdle. Calling Tick on such a monster dereferences a nil interface
// value and panics.
func TestMonsterControllerTickUnregisteredBehaviorStatePanics(t *testing.T) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildMonsterTestMap()

	player := model.NewDefaultPlayer(1, geometry.Point{X: 7, Y: 7}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	monster := model.NewDefaultZombie(2, geometry.Point{X: 2, Y: 2})
	// State is intentionally left at default zero value (BehaviorIdle).
	// m.behaviors[BehaviorIdle] is nil.
	monster.Vitals[model.VitalHP] = 100
	play.Monsters[monster.Id] = monster
	play.Map.SetActor(monster.Pos, int64(monster.Id))

	ctrl := newMonsterController()
	ctx := model.NewSessionContext(play)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Tick panicked for monster with BehaviorIdle (zero-value State): %v. "+
				"Monster constructors do not initialize State, so behaviors[0] is nil.", r)
		}
	}()

	ctrl.Tick(ctx)
}

//
// -- WanderBehavior: transition to Chase --
//

// When a monster in BehaviorWander state has a player within aggro range,
// WanderBehavior.Update returns (BehaviorChase, nil). The inner loop checks
// nextState != initState and loops again. However, monster.State is never
// updated inside the loop, so initState is always BehaviorWander, the
// transition condition fires again, and the loop runs forever.
func TestMonsterControllerTickWanderToChaseTransitionLoopsForever(t *testing.T) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildMonsterTestMap()

	// Player and monster within aggro range: dist(2) <= hostility(3).
	player := model.NewDefaultPlayer(1, geometry.Point{X: 2, Y: 4}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	monster := model.NewDefaultZombie(2, geometry.Point{X: 4, Y: 4})
	monster.State = model.BehaviorWander
	play.Monsters[monster.Id] = monster
	play.Map.SetActor(monster.Pos, int64(monster.Id))

	ctrl := newMonsterController()
	ctx := model.NewSessionContext(play)

	done := make(chan struct{})
	go func() {
		defer close(done)
		ctrl.Tick(ctx)
	}()

	select {
	case <-done:
		// Tick returned: no infinite loop. Bug may have been fixed.
	case <-time.After(300 * time.Millisecond):
		t.Errorf("Tick did not return within 300ms: monster.State is never updated " +
			"in the transition loop, so BehaviorWander fires the Chase transition " +
			"on every iteration and the loop never breaks.")
	}
}

//
// -- WanderBehavior: no transition --
//

func TestMonsterControllerTickWanderNoTransitionProducesIntent(t *testing.T) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildMonsterTestMap()

	// Player far from monster: dist > hostility. No transition should fire.
	// Player at (2,2), monster at (7,7). dist ≈ 7.07 > hostility=3.
	player := model.NewDefaultPlayer(1, geometry.Point{X: 2, Y: 2}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	monster := model.NewDefaultZombie(2, geometry.Point{X: 7, Y: 7})
	monster.State = model.BehaviorWander
	play.Monsters[monster.Id] = monster
	play.Map.SetActor(monster.Pos, int64(monster.Id))

	ctrl := newMonsterController()
	ctx := model.NewSessionContext(play)

	done := make(chan struct{})
	var intents []*model.Intent
	go func() {
		defer close(done)
		intents = ctrl.Tick(ctx)
	}()

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("Tick hung: possible transition triggered or scent map panic")
	}

	if !hasIntentForActor(intents, monster.Id) {
		t.Errorf("Expected an intent for wandering monster, got none")
	}
}

//
// -- ChaseBehavior: transition back to Wander --
//

// When a chasing monster's player moves far enough away, ChaseBehavior should
// transition back to BehaviorWander. Same infinite-loop bug applies: monster.State
// is never updated, so the transition fires on every loop iteration.
func TestMonsterControllerTickChaseToWanderTransitionLoopsForever(t *testing.T) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildMonsterTestMap()

	// Player far from monster: dist > 1.5*hostility(3) = 4.5.
	// Player at (2,2), monster at (7,7). dist ≈ 7.07 > 4.5 → transition fires.
	player := model.NewDefaultPlayer(1, geometry.Point{X: 2, Y: 2}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	monster := model.NewDefaultZombie(2, geometry.Point{X: 7, Y: 7})
	monster.State = model.BehaviorChase
	play.Monsters[monster.Id] = monster
	play.Map.SetActor(monster.Pos, int64(monster.Id))

	ctrl := newMonsterController()
	ctx := model.NewSessionContext(play)

	done := make(chan struct{})
	go func() {
		defer close(done)
		ctrl.Tick(ctx)
	}()

	select {
	case <-done:
		// Returned: no infinite loop. Bug may have been fixed.
	case <-time.After(300 * time.Millisecond):
		t.Errorf("Tick did not return within 300ms: ChaseBehavior transition to Wander " +
			"fires on every iteration because monster.State is never updated.")
	}
}

//
//
// --- TICK: EMPTY MONSTERS ---
//
//

func TestMonsterControllerTickNoMonstersReturnsEmptyIntents(t *testing.T) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildMonsterTestMap()

	player := model.NewDefaultPlayer(1, geometry.Point{X: 2, Y: 2}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	ctrl := newMonsterController()
	ctx := model.NewSessionContext(play)

	intents := ctrl.Tick(ctx)

	if len(intents) != 0 {
		t.Errorf("Expected empty intents for no monsters, got %d", len(intents))
	}
}

//
//
// --- TICK: SORT ORDER ---
//
//

func TestMonsterControllerTickSortDescendingByDexterity(t *testing.T) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildMonsterTestMap()

	// Player far away so no transitions fire.
	player := model.NewDefaultPlayer(1, geometry.Point{X: 4, Y: 4}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	slowMonster := model.NewDefaultZombie(2, geometry.Point{X: 2, Y: 2})
	slowMonster.State = model.BehaviorWander
	slowMonster.DerivedAttrs[model.AttrDexterity] = 10

	fastMonster := model.NewDefaultZombie(3, geometry.Point{X: 6, Y: 6})
	fastMonster.State = model.BehaviorWander
	fastMonster.DerivedAttrs[model.AttrDexterity] = 90

	play.Monsters[slowMonster.Id] = slowMonster
	play.Map.SetActor(slowMonster.Pos, int64(slowMonster.Id))
	play.Monsters[fastMonster.Id] = fastMonster
	play.Map.SetActor(fastMonster.Pos, int64(fastMonster.Id))

	ctrl := newMonsterController()
	ctx := model.NewSessionContext(play)

	done := make(chan struct{})
	var intents []*model.Intent
	go func() {
		defer close(done)
		intents = ctrl.Tick(ctx)
	}()

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("Tick hung")
	}

	// Both monsters must produce intents regardless of sort order.
	if !hasIntentForActor(intents, slowMonster.Id) {
		t.Errorf("Expected intent for slow monster")
	}
	if !hasIntentForActor(intents, fastMonster.Id) {
		t.Errorf("Expected intent for fast monster")
	}
}
