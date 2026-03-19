package model_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

// setupPlaythrough returns a minimal Playthrough with no map attached.
func setupPlaythrough() *model.Playthrough {
	return model.NewPlaythrough(0, 0, 42)
}

// setupPlaythroughWithMap returns a Playthrough with the standard test map attached.
func setupPlaythroughWithMap() *model.Playthrough {
	p := setupPlaythrough()
	p.Map = setupTestMap()
	return p
}

// addPlayer registers a player actor inside the playthrough under the given UUID.
func addPlayer(p *model.Playthrough, uuid string, id model.ActorId, pos geometry.Point, hp int) *model.Actor {
	actor := model.NewDefaultPlayer(id, pos, 9)
	actor.Vitals[model.VitalHP] = hp
	p.Players[id] = actor
	p.PlayersUuid[uuid] = id
	return actor
}

// addMonster registers a monster actor inside the playthrough.
func addMonster(p *model.Playthrough, id model.ActorId, pos geometry.Point) *model.Actor {
	actor := model.NewDefaultZombie(id, pos)
	p.Monsters[id] = actor
	return actor
}

// --- NewPlaythrough ---

func TestNewPlaythrough(t *testing.T) {
	p := model.NewPlaythrough(7, 3, 99)

	if p.PlaythroughId != 7 {
		t.Errorf("Expected PlaythroughId=7, got %d", p.PlaythroughId)
	}
	if p.RulesId != 3 {
		t.Errorf("Expected RulesId=3, got %d", p.RulesId)
	}
	if p.Seed != 99 {
		t.Errorf("Expected Seed=99, got %d", p.Seed)
	}
	if p.State != model.LobbyGameState {
		t.Errorf("Expected initial state LobbyGameState, got %d", p.State)
	}
	if p.DynamicDifficulty != 1 {
		t.Errorf("Expected DynamicDifficulty=1, got %v", p.DynamicDifficulty)
	}
	if p.Depth != 0 {
		t.Errorf("Expected Depth=0, got %d", p.Depth)
	}
	if p.NextId != 1 {
		t.Errorf("Expected NextId=1, got %d", p.NextId)
	}
	if p.Map != nil {
		t.Errorf("Expected Map=nil at construction")
	}
	if p.Players == nil || p.Monsters == nil || p.Items == nil {
		t.Errorf("Expected Players, Monsters and Items maps to be initialized")
	}
	if p.PlayersUuid == nil {
		t.Errorf("Expected PlayersUuid map to be initialized")
	}
	if p.PendingIntents == nil {
		t.Errorf("Expected PendingIntents map to be initialized")
	}
	if p.TurnEvents == nil {
		t.Errorf("Expected TurnEvents slice to be initialized")
	}
}

// --- GetId ---

func TestGetId(t *testing.T) {
	p := setupPlaythrough()

	first := p.GetId()
	second := p.GetId()
	third := p.GetId()

	if first != 1 {
		t.Errorf("Expected first ID=1, got %d", first)
	}
	if second != 2 {
		t.Errorf("Expected second ID=2, got %d", second)
	}
	if third != 3 {
		t.Errorf("Expected third ID=3, got %d", third)
	}
}

// --- GetAlivePlayers ---

func TestGetAlivePlayers(t *testing.T) {
	p := setupPlaythrough()
	pos := geometry.Point{X: 1, Y: 1}

	addPlayer(p, "uuid-alive", 1, pos, 50)
	addPlayer(p, "uuid-dead", 2, pos, 0)

	alive := p.GetAlivePlayers()

	if len(alive) != 1 {
		t.Errorf("Expected 1 alive player, got %d", len(alive))
	}
	if alive[0] != 1 {
		t.Errorf("Expected alive player ID=1, got %d", alive[0])
	}

	// Dead player should have an invalid position after the call.
	deadPos := p.Players[2].Pos
	if deadPos != model.NewInvalidPoint() {
		t.Errorf("Expected dead player position to be invalid, got %v", deadPos)
	}
}

// --- GetItem ---

func TestGetItem(t *testing.T) {
	p := setupPlaythrough()

	item := &model.Item{Id: 10, Kind: model.ItemTypeFood}
	p.Items[10] = item

	got := p.GetItem(10)
	if got == nil || got.Id != 10 {
		t.Errorf("Expected item with ID=10, got %v", got)
	}

	notFound := p.GetItem(999)
	if notFound != nil {
		t.Errorf("Expected nil for non-existent item, got %v", notFound)
	}
}

// --- GetActor ---

func TestGetActor(t *testing.T) {
	p := setupPlaythrough()
	pos := geometry.Point{X: 1, Y: 1}

	player := addPlayer(p, "uuid-1", 1, pos, 100)
	monster := addMonster(p, 2, pos)

	gotPlayer := p.GetActor(1)
	if gotPlayer != player {
		t.Errorf("Expected to retrieve the player actor")
	}

	gotMonster := p.GetActor(2)
	if gotMonster != monster {
		t.Errorf("Expected to retrieve the monster actor")
	}

	notFound := p.GetActor(999)
	if notFound != nil {
		t.Errorf("Expected nil for unknown actor ID, got %v", notFound)
	}
}

// --- GetPlayer ---

func TestGetPlayer(t *testing.T) {
	p := setupPlaythrough()
	pos := geometry.Point{X: 1, Y: 1}

	actor := addPlayer(p, "uuid-player", 5, pos, 100)

	got := p.GetPlayer("uuid-player")
	if got != actor {
		t.Errorf("Expected to retrieve player by UUID")
	}

	notFound := p.GetPlayer("no-such-uuid")
	if notFound != nil {
		t.Errorf("Expected nil for unknown UUID, got %v", notFound)
	}
}

// --- IsPlayer / IsMonster ---

func TestIsPlayerAndIsMonster(t *testing.T) {
	p := setupPlaythrough()
	pos := geometry.Point{X: 1, Y: 1}

	addPlayer(p, "uuid-1", 10, pos, 100)
	addMonster(p, 20, pos)

	if !p.IsPlayer(10) {
		t.Errorf("Expected actor 10 to be a player")
	}
	if p.IsPlayer(20) {
		t.Errorf("Expected actor 20 not to be a player")
	}
	if !p.IsMonster(20) {
		t.Errorf("Expected actor 20 to be a monster")
	}
	if p.IsMonster(10) {
		t.Errorf("Expected actor 10 not to be a monster")
	}
}

// --- IsHost ---

func TestIsHost(t *testing.T) {
	p := setupPlaythrough()
	pos := geometry.Point{X: 1, Y: 1}

	addPlayer(p, "uuid-host", 1, pos, 100)
	addPlayer(p, "uuid-guest", 2, pos, 100)
	p.HostId = 1

	if !p.IsHost("uuid-host") {
		t.Errorf("Expected uuid-host to be the host")
	}
	if p.IsHost("uuid-guest") {
		t.Errorf("Expected uuid-guest not to be the host")
	}
	if p.IsHost("no-such-uuid") {
		t.Errorf("Expected unknown UUID not to be the host")
	}
}

// --- IsPlayerDead ---

func TestIsPlayerDead(t *testing.T) {
	p := setupPlaythrough()
	pos := geometry.Point{X: 1, Y: 1}

	addPlayer(p, "uuid-alive", 1, pos, 100)
	addPlayer(p, "uuid-dead", 2, pos, 0)

	if p.IsPlayerDead(1) {
		t.Errorf("Expected player 1 (HP=100) to be alive")
	}
	if !p.IsPlayerDead(2) {
		t.Errorf("Expected player 2 (HP=0) to be dead")
	}
	if !p.IsPlayerDead(999) {
		t.Errorf("Expected non-existent actor to be treated as dead")
	}
}

// --- IsPlayerEscaped ---

func TestIsPlayerEscaped(t *testing.T) {
	p := setupPlaythrough()
	pos := geometry.Point{X: 1, Y: 1}
	invalid := model.NewInvalidPoint()

	addPlayer(p, "uuid-alive", 1, pos, 100)
	addPlayer(p, "uuid-escaped", 2, invalid, 100)
	addPlayer(p, "uuid-dead", 3, invalid, 0)

	if p.IsPlayerEscaped(1) {
		t.Errorf("Expected player at a valid position not to be escaped")
	}
	if !p.IsPlayerEscaped(2) {
		t.Errorf("Expected player with invalid position and HP>0 to be escaped")
	}
	if p.IsPlayerEscaped(3) {
		t.Errorf("Expected dead player (HP=0, invalid pos) not to be escaped")
	}
	if p.IsPlayerEscaped(999) {
		t.Errorf("Expected non-existent player not to be escaped")
	}
}

// --- AreAllPlayersDead ---

func TestAreAllPlayersDead(t *testing.T) {
	p := setupPlaythrough()
	pos := geometry.Point{X: 1, Y: 1}

	addPlayer(p, "uuid-1", 1, pos, 100)
	addPlayer(p, "uuid-2", 2, pos, 0)

	if p.AreAllPlayersDead() {
		t.Errorf("Expected not all players to be dead (player 1 is alive)")
	}

	p.Players[1].Vitals[model.VitalHP] = 0

	if !p.AreAllPlayersDead() {
		t.Errorf("Expected all players to be dead after setting HP to 0")
	}
}

func TestAreAllPlayersDeadEmptyPlaythrough(t *testing.T) {
	p := setupPlaythrough()

	// With no players, dead==0 and len(Players)==0, so 0==0 is true.
	if !p.AreAllPlayersDead() {
		t.Errorf("Expected AreAllPlayersDead to return true when there are no players")
	}
}

// --- AreAllAlivePlayersEscaped ---

func TestAreAllAlivePlayersEscaped(t *testing.T) {
	p := setupPlaythrough()
	pos := geometry.Point{X: 1, Y: 1}
	invalid := model.NewInvalidPoint()

	t.Run("OneAliveNotEscaped", func(t *testing.T) {
		p2 := setupPlaythrough()
		addPlayer(p2, "uuid-1", 1, pos, 100)

		if p2.AreAllAlivePlayersEscaped() {
			t.Errorf("Expected false when alive player has not escaped")
		}
	})

	t.Run("AllEscapedOrDead", func(t *testing.T) {
		addPlayer(p, "uuid-escaped", 1, invalid, 100)
		addPlayer(p, "uuid-dead", 2, invalid, 0)

		if !p.AreAllAlivePlayersEscaped() {
			t.Errorf("Expected true when remaining alive players have all escaped")
		}
	})
}

// --- GetKey ---

func TestGetKey(t *testing.T) {
	p := setupPlaythroughWithMap()

	// Place a key item at a known floor position.
	keyPos := geometry.Point{X: 1, Y: 1}
	keyItem := &model.Item{
		Id:      model.ItemId(1),
		Kind:    model.ItemTypeKey,
		Pos:     keyPos,
		Keyhole: 5,
	}
	p.Items[keyItem.Id] = keyItem
	p.Map.SetItem(keyPos, int64(keyItem.Id))

	got, ok := p.GetKey(keyPos)
	if !ok {
		t.Errorf("Expected to find a key at the given position")
	}
	if got == nil || got.Keyhole != 5 {
		t.Errorf("Expected key with Keyhole=5, got %v", got)
	}

	// Non-key item: food at another position.
	foodPos := geometry.Point{X: 2, Y: 1}
	foodItem := &model.Item{
		Id:      model.ItemId(2),
		Kind:    model.ItemTypeFood,
		Pos:     foodPos,
		Keyhole: 0,
	}
	p.Items[foodItem.Id] = foodItem
	p.Map.SetItem(foodPos, int64(foodItem.Id))

	_, ok = p.GetKey(foodPos)
	if ok {
		t.Errorf("Expected no key result for a non-key item")
	}

	// Empty position.
	emptyPos := geometry.Point{X: 3, Y: 3}
	_, ok = p.GetKey(emptyPos)
	if ok {
		t.Errorf("Expected no key at an empty position")
	}
}

// --- AddItem ---

func TestPlaythroughAddItem(t *testing.T) {
	p := setupPlaythroughWithMap()
	pos := geometry.Point{X: 1, Y: 1}

	item := &model.Item{Id: 1, Kind: model.ItemTypeFood, Pos: pos}
	p.AddItem(item)

	if _, exists := p.Items[1]; !exists {
		t.Errorf("Expected item 1 to be registered in Items map")
	}
	if !p.Map.IsItem(pos) {
		t.Errorf("Expected item to appear on the map at %v", pos)
	}
}

func TestAddItemNil(t *testing.T) {
	p := setupPlaythroughWithMap()
	// Should not panic.
	p.AddItem(nil)
}

func TestAddItemUpdatesOldPosition(t *testing.T) {
	p := setupPlaythroughWithMap()
	pos1 := geometry.Point{X: 1, Y: 1}
	pos2 := geometry.Point{X: 2, Y: 1}

	// First registration at pos1.
	item1 := &model.Item{Id: 1, Kind: model.ItemTypeFood, Pos: pos1}
	p.AddItem(item1)

	// Re-register the same ID at pos2 via a new pointer.
	// AddItem should remove the old position from the map.
	item2 := &model.Item{Id: 1, Kind: model.ItemTypeFood, Pos: pos2}
	p.AddItem(item2)

	if p.Map.IsItem(pos1) {
		t.Errorf("Expected old map position %v to be cleared after re-adding item with same ID", pos1)
	}
	if !p.Map.IsItem(pos2) {
		t.Errorf("Expected new map position %v to have the item", pos2)
	}
}

// --- AddMonster ---

func TestAddMonster(t *testing.T) {
	p := setupPlaythroughWithMap()
	pos := geometry.Point{X: 1, Y: 1}

	monster := model.NewDefaultZombie(10, pos)
	p.AddMonster(monster)

	if _, exists := p.Monsters[10]; !exists {
		t.Errorf("Expected monster 10 to be registered in Monsters map")
	}
	if !p.Map.IsActor(pos) {
		t.Errorf("Expected monster to appear on the map at %v", pos)
	}
}

func TestAddMonsterNil(t *testing.T) {
	p := setupPlaythroughWithMap()
	// Should not panic.
	p.AddMonster(nil)
}

// --- RemoveItem ---

func TestRemoveItem(t *testing.T) {
	p := setupPlaythroughWithMap()
	pos := geometry.Point{X: 1, Y: 1}

	item := &model.Item{Id: 1, Kind: model.ItemTypeFood, Pos: pos}
	p.AddItem(item)
	p.RemoveItem(item)

	if p.Map.IsItem(pos) {
		t.Errorf("Expected item to be removed from the map")
	}
	if item.Pos != model.NewInvalidPoint() {
		t.Errorf("Expected item Pos to be set to InvalidPoint after removal, got %v", item.Pos)
	}
}

func TestRemoveItemNil(t *testing.T) {
	p := setupPlaythroughWithMap()
	// Should not panic.
	p.RemoveItem(nil)
}

// --- KillActor ---

func TestKillActorMonster(t *testing.T) {
	p := setupPlaythroughWithMap()
	pos := geometry.Point{X: 1, Y: 1}

	monster := model.NewDefaultZombie(10, pos)
	p.AddMonster(monster)

	p.KillActor(monster)

	if _, exists := p.Monsters[10]; exists {
		t.Errorf("Expected killed monster to be removed from Monsters map")
	}
	if _, exists := p.DeadMonsters[10]; !exists {
		t.Errorf("Expected killed monster to be moved to DeadMonsters map")
	}
	if p.Map.IsActor(pos) {
		t.Errorf("Expected killed monster to be removed from the map")
	}
}

func TestKillActorPlayer(t *testing.T) {
	p := setupPlaythroughWithMap()
	pos := geometry.Point{X: 1, Y: 1}

	player := addPlayer(p, "uuid-1", 1, pos, 100)
	p.Map.SetActor(pos, int64(player.Id))

	p.KillActor(player)

	if p.Map.IsActor(pos) {
		t.Errorf("Expected killed player to be removed from the map")
	}
	if player.Pos != model.NewInvalidPoint() {
		t.Errorf("Expected killed player Pos to be set to InvalidPoint, got %v", player.Pos)
	}
}

func TestKillActorNil(t *testing.T) {
	p := setupPlaythroughWithMap()
	// Should not panic.
	p.KillActor(nil)
}

// --- HidePlayer ---

func TestHidePlayer(t *testing.T) {
	p := setupPlaythroughWithMap()
	pos := geometry.Point{X: 1, Y: 1}

	player := addPlayer(p, "uuid-1", 1, pos, 100)
	p.Map.SetActor(pos, int64(player.Id))

	p.HidePlayer(1)

	if player.Pos != model.NewInvalidPoint() {
		t.Errorf("Expected player Pos to be invalid after hiding, got %v", player.Pos)
	}
	if p.Map.IsActor(pos) {
		t.Errorf("Expected player to be removed from the map after hiding")
	}
}

func TestHidePlayerNonExistent(t *testing.T) {
	p := setupPlaythroughWithMap()
	// Should not panic.
	p.HidePlayer(999)
}

// --- DisconnectPlayer ---

func TestDisconnectPlayer(t *testing.T) {
	p := setupPlaythroughWithMap()
	pos := geometry.Point{X: 1, Y: 1}

	player := addPlayer(p, "uuid-1", 1, pos, 100)
	p.Map.SetActor(pos, int64(player.Id))

	p.DisconnectPlayer("uuid-1")

	if player.Pos != model.NewInvalidPoint() {
		t.Errorf("Expected disconnected player Pos to be invalid, got %v", player.Pos)
	}
}

func TestDisconnectPlayerUnknownUUID(t *testing.T) {
	p := setupPlaythroughWithMap()
	// Should not panic.
	p.DisconnectPlayer("no-such-uuid")
}

// --- DisconnectAll ---

func TestDisconnectAll(t *testing.T) {
	p := setupPlaythroughWithMap()
	pos1 := geometry.Point{X: 1, Y: 1}
	pos2 := geometry.Point{X: 2, Y: 1}

	player1 := addPlayer(p, "uuid-1", 1, pos1, 100)
	player2 := addPlayer(p, "uuid-2", 2, pos2, 100)
	p.Map.SetActor(pos1, int64(player1.Id))
	p.Map.SetActor(pos2, int64(player2.Id))

	p.DisconnectAll()

	if player1.Pos != model.NewInvalidPoint() {
		t.Errorf("Expected player 1 Pos to be invalid after DisconnectAll")
	}
	if player2.Pos != model.NewInvalidPoint() {
		t.Errorf("Expected player 2 Pos to be invalid after DisconnectAll")
	}
}

// --- ClearMetrics ---

func TestClearMetrics(t *testing.T) {
	p := setupPlaythrough()

	p.PlayersLevelMetrics[1] = &model.LevelMetrics{DamageTaken: 50, DamageDealt: 100}
	p.PlayersLevelMetrics[2] = &model.LevelMetrics{DamageTaken: 10, DamageDealt: 20}

	p.ClearMetrics()

	for id, m := range p.PlayersLevelMetrics {
		if m.DamageTaken != 0 || m.DamageDealt != 0 {
			t.Errorf("Expected metrics for player %d to be zeroed, got DamageTaken=%d DamageDealt=%d", id, m.DamageTaken, m.DamageDealt)
		}
	}
}

// --- AdaptDifficulty ---

func TestAdaptDifficultyLowHPLowFood(t *testing.T) {
	p := setupPlaythrough()
	pos := geometry.Point{X: 1, Y: 1}

	player := addPlayer(p, "uuid-1", 1, pos, 10) // HP well below 25% of default 100
	player.BaseAttrs[model.AttrMaxHP] = 100
	// No food in backpack.

	// Add food items to the world.
	p.Items[1] = &model.Item{Id: 1, Kind: model.ItemTypeFood, Label: model.ItemLabelDefaultFood}
	p.Items[2] = &model.Item{Id: 2, Kind: model.ItemTypeFood, Label: model.ItemLabelDefaultFood}

	itemWeights, _ := p.AdaptDifficulty()

	if p.DynamicDifficulty != 0.8 {
		t.Errorf("Expected DynamicDifficulty=0.8 when HP and food are low, got %v", p.DynamicDifficulty)
	}
	for label, weight := range itemWeights {
		if weight != 25 {
			t.Errorf("Expected food item %v weight=25, got %d", label, weight)
		}
	}
}

func TestAdaptDifficultyHighHPHighFood(t *testing.T) {
	p := setupPlaythrough()
	pos := geometry.Point{X: 1, Y: 1}

	player := addPlayer(p, "uuid-1", 1, pos, 100) // HP = MaxHP (full)
	player.BaseAttrs[model.AttrMaxHP] = 100
	// Add 3 food items to backpack to satisfy avgFood > 2.0.
	player.Backpack = model.NewBackpack(9)
	food1 := &model.Item{Id: 10, Kind: model.ItemTypeFood}
	food2 := &model.Item{Id: 11, Kind: model.ItemTypeFood}
	food3 := &model.Item{Id: 12, Kind: model.ItemTypeFood}
	player.Backpack.Slots[model.ItemTypeFood] = []*model.Item{food1, food2, food3}

	// Add food items to the world.
	p.Items[1] = &model.Item{Id: 1, Kind: model.ItemTypeFood, Label: model.ItemLabelDefaultFood}

	itemWeights, _ := p.AdaptDifficulty()

	if p.DynamicDifficulty != 1.2 {
		t.Errorf("Expected DynamicDifficulty=1.2 when HP and food are high, got %v", p.DynamicDifficulty)
	}
	for label, weight := range itemWeights {
		if weight != -25 {
			t.Errorf("Expected food item %v weight=-25, got %d", label, weight)
		}
	}
}
