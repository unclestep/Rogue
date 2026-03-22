package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/unclestep/Rogue/internal/dto"
)

// --- test helpers ---

// newTestCtx creates a UIContext backed by buffered channels for unit testing.
func newTestCtx(t *testing.T) (*UIContext, chan dto.Command) {
	t.Helper()
	toServer := make(chan dto.Command, 8)
	fromServer := make(chan dto.GameView, 1)
	ctx := NewUIContext(toServer, fromServer, "player-uuid", "")
	return ctx, toServer
}

// newTestCtxWithLastID is like newTestCtx but pre-loads a saved playthrough ID.
func newTestCtxWithLastID(t *testing.T, lastID string) (*UIContext, chan dto.Command) {
	t.Helper()
	toServer := make(chan dto.Command, 8)
	fromServer := make(chan dto.GameView, 1)
	ctx := NewUIContext(toServer, fromServer, "player-uuid", lastID)
	return ctx, toServer
}

// pressKey returns a KeyMsg for the given key name.
func pressKey(k string) tea.KeyMsg {
	switch k {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
}

// drainCmd reads one command from the channel, failing if none is present.
func drainCmd(t *testing.T, ch chan dto.Command) dto.Command {
	t.Helper()
	select {
	case cmd := <-ch:
		return cmd
	default:
		t.Fatal("Expected a command on the server channel, but none was sent")
		return dto.Command{}
	}
}

// noCmd asserts the channel is empty.
func noCmd(t *testing.T, ch chan dto.Command) {
	t.Helper()
	select {
	case cmd := <-ch:
		t.Errorf("Unexpected command sent to server: action=%v", cmd.Action)
	default:
	}
}

// playingWorld builds a minimal GameView suitable for PlayingState.
func playingWorld(playerRow, playerCol int, grid [][]*dto.Cell) dto.GameView {
	return dto.GameView{
		State:  dto.StatePlaying,
		Height: len(grid),
		Width:  len(grid[0]),
		Grid:   grid,
		Player: &dto.Player{
			Row: playerRow,
			Col: playerCol,
			HUD: &dto.HUD{HP: 10, MaxHP: 10},
		},
	}
}

// floorGrid returns a Height×Width grid that is all floor, so every move is valid.
func floorGrid(h, w int) [][]*dto.Cell {
	grid := make([][]*dto.Cell, h)
	for r := 0; r < h; r++ {
		grid[r] = make([]*dto.Cell, w)
		for c := 0; c < w; c++ {
			grid[r][c] = &dto.Cell{
				TopologyType:    dto.TopologyFloor,
				VisibilityState: dto.VisibilityVisible,
			}
		}
	}
	return grid
}

// wallGrid returns a Height×Width grid that is all walls, so every move is invalid.
func wallGrid(h, w int) [][]*dto.Cell {
	grid := make([][]*dto.Cell, h)
	for r := 0; r < h; r++ {
		grid[r] = make([]*dto.Cell, w)
		for c := 0; c < w; c++ {
			grid[r][c] = &dto.Cell{
				TopologyType:    dto.TopologyWall,
				VisibilityState: dto.VisibilityVisible,
			}
		}
	}
	return grid
}

//
// --- PlayingState ---
//

func TestPlayingStateWalkableMoveSendsActionMove(t *testing.T) {
	directions := []struct {
		keyName string
		vector  dto.Vector
	}{
		{"w", dto.Vector{X: 0, Y: -1}},
		{"s", dto.Vector{X: 0, Y: 1}},
		{"a", dto.Vector{X: -1, Y: 0}},
		{"d", dto.Vector{X: 1, Y: 0}},
	}
	for _, dir := range directions {
		ctx, ch := newTestCtx(t)
		// 3×3 all-floor grid; player at centre — all 4 moves are valid.
		ctx.World = playingWorld(1, 1, floorGrid(3, 3))
		state := NewPlayingState(ctx)

		nextState, _ := state.Update(pressKey(dir.keyName))

		if nextState != nil {
			t.Errorf("key=%q: expected no state transition, got %T", dir.keyName, nextState)
		}
		cmd := drainCmd(t, ch)
		if cmd.Action != dto.ActionMove {
			t.Errorf("key=%q: expected ActionMove, got %v", dir.keyName, cmd.Action)
		}
		if cmd.MoveVector != dir.vector {
			t.Errorf("key=%q: expected vector %v, got %v", dir.keyName, dir.vector, cmd.MoveVector)
		}
	}
}

func TestPlayingStateInvalidMoveDoesNotSendCommand(t *testing.T) {
	ctx, ch := newTestCtx(t)
	// All walls — no move is valid.
	ctx.World = playingWorld(1, 1, wallGrid(3, 3))
	state := NewPlayingState(ctx)

	state.Update(pressKey("d")) // right into a wall

	noCmd(t, ch)
	if state.invalidTarget == nil {
		t.Error("Expected invalidTarget to be set after blocked move")
	}
}

func TestPlayingStateMoveWithNilGridDoesNotSendCommand(t *testing.T) {
	ctx, ch := newTestCtx(t)
	ctx.World = dto.GameView{Player: &dto.Player{Row: 0, Col: 0}}
	state := NewPlayingState(ctx)

	state.Update(pressKey("w"))

	noCmd(t, ch)
}

func TestPlayingStateSpaceSendsActionWait(t *testing.T) {
	ctx, ch := newTestCtx(t)
	ctx.World = playingWorld(1, 1, floorGrid(3, 3))
	state := NewPlayingState(ctx)

	nextState, _ := state.Update(pressKey("space"))

	if nextState != nil {
		t.Errorf("Expected no state transition, got %T", nextState)
	}
	cmd := drainCmd(t, ch)
	if cmd.Action != dto.ActionWait {
		t.Errorf("Expected ActionWait, got %v", cmd.Action)
	}
}

func TestPlayingStateEscSendsLeaveAndReturnsMenuState(t *testing.T) {
	ctx, ch := newTestCtx(t)
	ctx.World = playingWorld(1, 1, floorGrid(3, 3))
	state := NewPlayingState(ctx)

	nextState, _ := state.Update(pressKey("esc"))

	if _, ok := nextState.(*MenuState); !ok {
		t.Errorf("Expected *MenuState after Esc, got %T", nextState)
	}
	cmd := drainCmd(t, ch)
	if cmd.Action != dto.ActionLeave {
		t.Errorf("Expected ActionLeave, got %v", cmd.Action)
	}
}

func TestPlayingStateInventoryKeyOpensInventory(t *testing.T) {
	ctx, _ := newTestCtx(t)
	ctx.World = playingWorld(1, 1, floorGrid(3, 3))
	state := NewPlayingState(ctx)

	nextState, _ := state.Update(pressKey("i"))

	if _, ok := nextState.(*InventoryState); !ok {
		t.Errorf("Expected *InventoryState after 'i', got %T", nextState)
	}
}

func TestPlayingStateSlotKeysOpenFilteredInventory(t *testing.T) {
	slots := []struct {
		keyName    string
		filterType dto.ItemType
		itemKind   dto.ItemType
		itemLabel  dto.ItemLabel
	}{
		{"h", "weapon", "weapon", "Sword"},
		{"j", "food", "food", "Bread"},
		{"k", "elixir", "elixir_strength", "Potion"},
		{"e", "scroll", "scroll_strength", "Scroll"},
	}
	for _, s := range slots {
		ctx, _ := newTestCtx(t)
		world := playingWorld(1, 1, floorGrid(3, 3))
		// Populate the inventory so the view title is rendered.
		world.Player.Inventory = map[dto.ItemType][]*dto.Item{
			s.itemKind: {{Id: 1, Kind: s.itemKind, Label: s.itemLabel}},
		}
		world.Player.Equipped = map[dto.ItemType]*dto.Item{}
		ctx.World = world
		state := NewPlayingState(ctx)

		nextState, _ := state.Update(pressKey(s.keyName))

		inv, ok := nextState.(*InventoryState)
		if !ok {
			t.Errorf("key=%q: expected *InventoryState, got %T", s.keyName, nextState)
			continue
		}
		// The inventory list title must reflect the filter type.
		view := inv.View()
		if !containsIgnoreCase(view, string(s.filterType)) {
			t.Errorf("key=%q: inventory view does not mention filter %q", s.keyName, s.filterType)
		}
	}
}

func TestPlayingStateGameoverViewTransition(t *testing.T) {
	ctx, _ := newTestCtx(t)
	state := NewPlayingState(ctx)

	nextState, _ := state.Update(dto.GameView{State: dto.StateGameover})

	if _, ok := nextState.(*GameoverState); !ok {
		t.Errorf("Expected *GameoverState on StateGameover, got %T", nextState)
	}
}

func TestPlayingStateLobbyViewTransition(t *testing.T) {
	ctx, _ := newTestCtx(t)
	state := NewPlayingState(ctx)

	nextState, _ := state.Update(dto.GameView{State: dto.StateLobby})

	if _, ok := nextState.(*LobbyState); !ok {
		t.Errorf("Expected *LobbyState on StateLobby, got %T", nextState)
	}
}

func TestPlayingStateViewReturnsLoadingWhenPlayerNil(t *testing.T) {
	ctx, _ := newTestCtx(t)
	ctx.World = dto.GameView{} // no Player, no Grid
	state := NewPlayingState(ctx)

	got := state.View()
	if got != "Loading..." {
		t.Errorf("Expected \"Loading...\" for nil player/grid, got %q", got)
	}
}

func TestPlayingStateInvalidTargetClearedOnNextInput(t *testing.T) {
	ctx, ch := newTestCtx(t)
	// All walls so move is invalid.
	ctx.World = playingWorld(1, 1, wallGrid(3, 3))
	state := NewPlayingState(ctx)

	// First key: blocked move sets invalidTarget.
	state.Update(pressKey("d"))
	if state.invalidTarget == nil {
		t.Fatal("Expected invalidTarget to be set after blocked move")
	}

	// Now change the world so a move is valid, then press a key.
	ctx.World = playingWorld(1, 1, floorGrid(3, 3))
	state.Update(pressKey("d"))

	if state.invalidTarget != nil {
		t.Error("Expected invalidTarget to be cleared on next key press")
	}
	drainCmd(t, ch) // discard the valid move
}

//
// --- MenuState ---
//

func TestMenuStateNewGameSendsJoinCommandAndWaits(t *testing.T) {
	ctx, ch := newTestCtxWithLastID(t, "")
	m := NewMenuState(ctx)

	nextState, _ := m.handleSelect("New Game")

	if nextState != nil {
		t.Errorf("Expected nil (stay in MenuState), got %T", nextState)
	}
	if !m.waiting {
		t.Error("Expected waiting=true after New Game selection")
	}
	cmd := drainCmd(t, ch)
	if cmd.Action != dto.ActionJoin {
		t.Errorf("Expected ActionJoin, got %v", cmd.Action)
	}
	if cmd.PlaythroughParams == nil {
		t.Error("Expected PlaythroughParams to be populated for New Game")
	}
	if cmd.PlaythroughId != "" {
		t.Errorf("Expected empty PlaythroughId for New Game, got %q", cmd.PlaythroughId)
	}
}

func TestMenuStateContinueSendsJoinWithSavedId(t *testing.T) {
	savedID := "saved-session-id"
	ctx, ch := newTestCtxWithLastID(t, savedID)
	m := NewMenuState(ctx)

	nextState, _ := m.handleSelect("Continue")

	if nextState != nil {
		t.Errorf("Expected nil (stay in MenuState), got %T", nextState)
	}
	cmd := drainCmd(t, ch)
	if cmd.Action != dto.ActionJoin {
		t.Errorf("Expected ActionJoin, got %v", cmd.Action)
	}
	if cmd.PlaythroughId != savedID {
		t.Errorf("Expected PlaythroughId=%q, got %q", savedID, cmd.PlaythroughId)
	}
}

func TestMenuStateJoinGameTransitionsToJoinState(t *testing.T) {
	ctx, _ := newTestCtx(t)
	m := NewMenuState(ctx)

	nextState, _ := m.handleSelect("Join Game")

	if _, ok := nextState.(*JoinState); !ok {
		t.Errorf("Expected *JoinState, got %T", nextState)
	}
}

func TestMenuStateQuitReturnsTeaQuit(t *testing.T) {
	ctx, _ := newTestCtx(t)
	m := NewMenuState(ctx)

	nextState, cmd := m.handleSelect("Quit")

	if nextState != nil {
		t.Errorf("Expected nil state for Quit, got %T", nextState)
	}
	if cmd == nil {
		t.Error("Expected tea.Quit command, got nil")
	}
}

func TestMenuStateGameViewLobbyTransitionsWhenWaiting(t *testing.T) {
	ctx, _ := newTestCtx(t)
	m := NewMenuState(ctx)
	m.waiting = true

	nextState, _ := m.Update(dto.GameView{State: dto.StateLobby})

	if _, ok := nextState.(*LobbyState); !ok {
		t.Errorf("Expected *LobbyState on StateLobby, got %T", nextState)
	}
}

func TestMenuStateGameViewPlayingTransitionsWhenWaiting(t *testing.T) {
	ctx, _ := newTestCtx(t)
	m := NewMenuState(ctx)
	m.waiting = true

	nextState, _ := m.Update(dto.GameView{State: dto.StatePlaying})

	if _, ok := nextState.(*PlayingState); !ok {
		t.Errorf("Expected *PlayingState on StatePlaying, got %T", nextState)
	}
}

func TestMenuStateGameViewUnknownClearsLastIdAndRebuildsMenu(t *testing.T) {
	ctx, _ := newTestCtxWithLastID(t, "old-session-id")
	m := NewMenuState(ctx)
	m.waiting = true

	nextState, _ := m.Update(dto.GameView{State: dto.StateUnknown})

	newMenu, ok := nextState.(*MenuState)
	if !ok {
		t.Fatalf("Expected *MenuState on StateUnknown, got %T", nextState)
	}
	if newMenu.Ctx.LastPlaythroughId != "" {
		t.Errorf("Expected LastPlaythroughId to be cleared, got %q", newMenu.Ctx.LastPlaythroughId)
	}
}

func TestMenuStateGameViewIgnoredWhenNotWaiting(t *testing.T) {
	ctx, _ := newTestCtx(t)
	m := NewMenuState(ctx)
	// Not waiting: GameView messages should be ignored.

	nextState, _ := m.Update(dto.GameView{State: dto.StateLobby})

	if nextState != nil {
		t.Errorf("Expected nil (no transition) when not waiting, got %T", nextState)
	}
}

//
// --- JoinState ---
//

func TestJoinStateEmptyInputShowsError(t *testing.T) {
	ctx, ch := newTestCtx(t)
	j := NewJoinState(ctx)

	nextState, _ := j.Update(pressKey("enter"))

	if nextState != nil {
		t.Errorf("Expected nil state (stay), got %T", nextState)
	}
	if j.errMsg == "" {
		t.Error("Expected an error message for empty input")
	}
	noCmd(t, ch)
}

func TestJoinStateEscReturnsMenuState(t *testing.T) {
	ctx, _ := newTestCtx(t)
	j := NewJoinState(ctx)

	nextState, _ := j.Update(pressKey("esc"))

	if _, ok := nextState.(*MenuState); !ok {
		t.Errorf("Expected *MenuState on Esc, got %T", nextState)
	}
}

func TestJoinStateUnknownStateShowsNotFoundError(t *testing.T) {
	ctx, _ := newTestCtx(t)
	j := NewJoinState(ctx)
	j.waiting = true

	nextState, _ := j.Update(dto.GameView{State: dto.StateUnknown})

	if nextState != nil {
		t.Errorf("Expected nil (stay in JoinState), got %T", nextState)
	}
	if j.waiting {
		t.Error("Expected waiting to be cleared after StateUnknown")
	}
	if j.errMsg == "" {
		t.Error("Expected an error message for StateUnknown")
	}
}

func TestJoinStateAlreadyStartedShowsError(t *testing.T) {
	ctx, _ := newTestCtx(t)
	j := NewJoinState(ctx)
	j.waiting = true

	nextState, _ := j.Update(dto.GameView{State: dto.StatePlaying})

	if nextState != nil {
		t.Errorf("Expected nil (stay in JoinState), got %T", nextState)
	}
	if j.waiting {
		t.Error("Expected waiting to be cleared after StatePlaying")
	}
	if j.errMsg == "" {
		t.Error("Expected an error message for already started session")
	}
}

func TestJoinStateLobbyTransitionsWhenWaiting(t *testing.T) {
	ctx, _ := newTestCtx(t)
	j := NewJoinState(ctx)
	j.waiting = true

	nextState, _ := j.Update(dto.GameView{State: dto.StateLobby})

	if _, ok := nextState.(*LobbyState); !ok {
		t.Errorf("Expected *LobbyState on StateLobby, got %T", nextState)
	}
}

//
// --- LobbyState ---
//

func TestLobbyStateHostEnterSendsActionYes(t *testing.T) {
	ctx, ch := newTestCtx(t)
	ctx.World = dto.GameView{
		Player: &dto.Player{IsHost: true},
	}
	state := NewLobbyState(ctx)

	state.Update(pressKey("enter"))

	cmd := drainCmd(t, ch)
	if cmd.Action != dto.ActionYes {
		t.Errorf("Expected ActionYes from host Enter, got %v", cmd.Action)
	}
}

func TestLobbyStateGuestEnterDoesNothing(t *testing.T) {
	ctx, ch := newTestCtx(t)
	ctx.World = dto.GameView{
		Player: &dto.Player{IsHost: false},
	}
	state := NewLobbyState(ctx)

	nextState, _ := state.Update(pressKey("enter"))

	if nextState != nil {
		t.Errorf("Expected no transition for guest Enter, got %T", nextState)
	}
	noCmd(t, ch)
}

func TestLobbyStateEscSendsLeaveAndReturnsMenuState(t *testing.T) {
	ctx, ch := newTestCtx(t)
	state := NewLobbyState(ctx)

	nextState, _ := state.Update(pressKey("esc"))

	if _, ok := nextState.(*MenuState); !ok {
		t.Errorf("Expected *MenuState on Esc, got %T", nextState)
	}
	cmd := drainCmd(t, ch)
	if cmd.Action != dto.ActionLeave {
		t.Errorf("Expected ActionLeave, got %v", cmd.Action)
	}
}

func TestLobbyStatePlayingViewTransition(t *testing.T) {
	ctx, _ := newTestCtx(t)
	state := NewLobbyState(ctx)

	nextState, _ := state.Update(dto.GameView{State: dto.StatePlaying})

	if _, ok := nextState.(*PlayingState); !ok {
		t.Errorf("Expected *PlayingState on StatePlaying, got %T", nextState)
	}
}

func TestLobbyStateUnknownViewTransition(t *testing.T) {
	ctx, _ := newTestCtx(t)
	state := NewLobbyState(ctx)

	nextState, _ := state.Update(dto.GameView{State: dto.StateUnknown})

	if _, ok := nextState.(*MenuState); !ok {
		t.Errorf("Expected *MenuState on StateUnknown (session deleted), got %T", nextState)
	}
}

//
// --- GameoverState ---
//

func TestGameoverStateHostEnterSendsYesAndReturnsMenuState(t *testing.T) {
	ctx, ch := newTestCtx(t)
	ctx.World = dto.GameView{
		Player: &dto.Player{IsHost: true},
	}
	state := NewGameoverState(ctx)

	nextState, _ := state.Update(pressKey("enter"))

	if _, ok := nextState.(*MenuState); !ok {
		t.Errorf("Expected *MenuState after host Enter, got %T", nextState)
	}
	cmd := drainCmd(t, ch)
	if cmd.Action != dto.ActionYes {
		t.Errorf("Expected ActionYes, got %v", cmd.Action)
	}
}

func TestGameoverStateGuestEnterDoesNothing(t *testing.T) {
	ctx, ch := newTestCtx(t)
	ctx.World = dto.GameView{
		Player: &dto.Player{IsHost: false},
	}
	state := NewGameoverState(ctx)

	nextState, _ := state.Update(pressKey("enter"))

	if nextState != nil {
		t.Errorf("Expected no transition for guest Enter, got %T", nextState)
	}
	noCmd(t, ch)
}

func TestGameoverStateEscSendsLeaveAndQuits(t *testing.T) {
	ctx, ch := newTestCtx(t)
	state := NewGameoverState(ctx)

	_, cmd := state.Update(pressKey("esc"))

	if cmd == nil {
		t.Error("Expected tea.Quit command on Esc")
	}
	sentCmd := drainCmd(t, ch)
	if sentCmd.Action != dto.ActionLeave {
		t.Errorf("Expected ActionLeave on Esc, got %v", sentCmd.Action)
	}
}

//
// --- InventoryState ---
//

func TestInventoryStateEscReturnsPlayingState(t *testing.T) {
	ctx, _ := newTestCtx(t)
	state := NewInventoryState(ctx, "")

	nextState, _ := state.Update(pressKey("esc"))

	if _, ok := nextState.(*PlayingState); !ok {
		t.Errorf("Expected *PlayingState on Esc, got %T", nextState)
	}
}

func TestInventoryStateConsumeItemSendsActionConsumeAndReturnsPlaying(t *testing.T) {
	ctx, ch := newTestCtx(t)
	bread := &dto.Item{Id: 1, Kind: "food", Label: "Bread"}
	ctx.World = dto.GameView{
		Player: &dto.Player{
			Inventory: map[dto.ItemType][]*dto.Item{"food": {bread}},
			Equipped:  map[dto.ItemType]*dto.Item{},
		},
	}
	state := NewInventoryState(ctx, "")
	entry := inventoryEntry{item: bread, equipped: false}

	nextState, _ := state.useItem(entry)

	if _, ok := nextState.(*PlayingState); !ok {
		t.Errorf("Expected *PlayingState after consume, got %T", nextState)
	}
	cmd := drainCmd(t, ch)
	if cmd.Action != dto.ActionConsumeItem {
		t.Errorf("Expected ActionConsumeItem, got %v", cmd.Action)
	}
	if cmd.ItemID != int64(bread.Id) {
		t.Errorf("Expected ItemID=%d, got %d", bread.Id, cmd.ItemID)
	}
}

func TestInventoryStateEquipWeaponSendsActionEquip(t *testing.T) {
	ctx, ch := newTestCtx(t)
	sword := &dto.Item{Id: 5, Kind: "weapon", Label: "Sword"}
	state := NewInventoryState(ctx, "")
	entry := inventoryEntry{item: sword, equipped: false}

	nextState, _ := state.useItem(entry)

	if _, ok := nextState.(*PlayingState); !ok {
		t.Errorf("Expected *PlayingState after equip, got %T", nextState)
	}
	cmd := drainCmd(t, ch)
	if cmd.Action != dto.ActionEquipWeapon {
		t.Errorf("Expected ActionEquipWeapon, got %v", cmd.Action)
	}
}

func TestInventoryStateUnequipWeaponSendsActionUnequip(t *testing.T) {
	ctx, ch := newTestCtx(t)
	sword := &dto.Item{Id: 5, Kind: "weapon", Label: "Sword"}
	state := NewInventoryState(ctx, "")
	entry := inventoryEntry{item: sword, equipped: true}

	nextState, _ := state.useItem(entry)

	if _, ok := nextState.(*PlayingState); !ok {
		t.Errorf("Expected *PlayingState after unequip, got %T", nextState)
	}
	cmd := drainCmd(t, ch)
	if cmd.Action != dto.ActionUnequipWeapon {
		t.Errorf("Expected ActionUnequipWeapon, got %v", cmd.Action)
	}
}

// containsIgnoreCase checks whether s contains substr case-insensitively.
func containsIgnoreCase(s, substr string) bool {
	s2 := strings.ToLower(s)
	return strings.Contains(s2, strings.ToLower(substr))
}
