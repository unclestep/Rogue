package tui

import (
	"strings"
	"testing"

	"github.com/unclestep/Rogue/internal/dto"
)

// --- isWalkableTile ---

func TestIsWalkableTileReturnsTrueForWalkableTypes(t *testing.T) {
	walkable := []dto.TopologyType{
		dto.TopologyFloor, dto.TopologyCorridor,
		dto.TopologyOpenDoor, dto.TopologyClosedDoor, dto.TopologyExit,
	}
	for _, tt := range walkable {
		cell := &dto.Cell{TopologyType: tt}
		if !isWalkableTile(cell) {
			t.Errorf("Expected %v to be walkable", tt)
		}
	}
}

func TestIsWalkableTileReturnsFalseForBlockingTypes(t *testing.T) {
	blocking := []dto.TopologyType{
		dto.TopologyWall, dto.TopologyEmpty,
		dto.TopologyUnknown,
	}
	for _, tt := range blocking {
		cell := &dto.Cell{TopologyType: tt}
		if isWalkableTile(cell) {
			t.Errorf("Expected %v to be non-walkable", tt)
		}
	}
}

func TestIsWalkableTileReturnsFalseForNilCell(t *testing.T) {
	if isWalkableTile(nil) {
		t.Error("Expected nil cell to be non-walkable")
	}
}

// --- tileChar ---

func TestTileCharReturnsCorrectSymbols(t *testing.T) {
	cases := []struct {
		topology dto.TopologyType
		want     string
	}{
		{dto.TopologyWall, "#"},
		{dto.TopologyFloor, "·"},
		{dto.TopologyCorridor, "·"},
		{dto.TopologyOpenDoor, "/"},
		{dto.TopologyClosedDoor, "%"},
		{dto.TopologyExit, ">"},
		{dto.TopologyEmpty, " "},
		{dto.TopologyUnknown, " "},
	}
	for _, tc := range cases {
		cell := &dto.Cell{TopologyType: tc.topology}
		if got := tileChar(cell); got != tc.want {
			t.Errorf("tileChar(%v) = %q, want %q", tc.topology, got, tc.want)
		}
	}
}

// --- actorChar ---

func TestActorCharReturnsCorrectSymbols(t *testing.T) {
	cases := []struct {
		kind dto.ActorKind
		want string
	}{
		{"player", "@"},
		{"player_common", "@"},
		{"zombie", "Z"},
		{"vampire", "V"},
		{"ghost", "G"},
		{"ogre", "O"},
		{"snake_mage", "S"},
		{"mimic", "M"},
		{"unknown_creature", "?"},
	}
	for _, tc := range cases {
		actor := &dto.Actor{Kind: tc.kind}
		if got := actorChar(actor); got != tc.want {
			t.Errorf("actorChar(%q) = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

// --- renderHUD ---

func TestRenderHUDReturnsEmptyForNilHUD(t *testing.T) {
	if got := renderHUD(nil); got != "" {
		t.Errorf("renderHUD(nil) = %q, want empty string", got)
	}
}

func TestRenderHUDContainsHPAndStats(t *testing.T) {
	hud := &dto.HUD{HP: 15, MaxHP: 20, Strength: 10, Dexterity: 8, Dungeon: 3, Treasure: 50}
	got := renderHUD(hud)
	for _, sub := range []string{"15", "20", "10", "8", "3", "50"} {
		if !strings.Contains(got, sub) {
			t.Errorf("renderHUD output missing %q: %q", sub, got)
		}
	}
}

func TestRenderHUDZeroMaxHPReturnsNonEmptyString(t *testing.T) {
	hud := &dto.HUD{HP: 0, MaxHP: 0}
	got := renderHUD(hud)
	// Should not panic and should return something.
	if got == "" {
		t.Error("renderHUD with zero MaxHP returned empty string")
	}
}

// --- buildHPBar ---

func TestBuildHPBarFullHealth(t *testing.T) {
	bar := buildHPBar(10, 10, 10)
	if strings.Contains(bar, "░") {
		t.Errorf("Full health bar should have no empty segments, got: %q", bar)
	}
}

func TestBuildHPBarEmptyHealth(t *testing.T) {
	bar := buildHPBar(0, 10, 10)
	if strings.Contains(bar, "█") {
		t.Errorf("Empty health bar should have no filled segments, got: %q", bar)
	}
}

func TestBuildHPBarZeroMaxHPReturnsEmpty(t *testing.T) {
	if got := buildHPBar(5, 0, 10); got != "" {
		t.Errorf("buildHPBar with MaxHP=0 should return empty, got %q", got)
	}
}

// --- renderEvents ---

func TestRenderEventsReturnsEmptyForNoEvents(t *testing.T) {
	if got := renderEvents(nil, 5); got != "" {
		t.Errorf("renderEvents(nil) = %q, want empty", got)
	}
}

func TestRenderEventsReturnsLastNEvents(t *testing.T) {
	events := []*dto.Event{
		{Desc: "event-1"},
		{Desc: "event-2"},
		{Desc: "event-3"},
		{Desc: "event-4"},
	}
	got := renderEvents(events, 2)
	if !strings.Contains(got, "event-4") {
		t.Error("Expected newest event (event-4) in output")
	}
	if !strings.Contains(got, "event-3") {
		t.Error("Expected second-newest event (event-3) in output")
	}
	if strings.Contains(got, "event-1") {
		t.Error("Expected oldest event (event-1) to be excluded")
	}
}

func TestRenderEventsLargerNThanSlice(t *testing.T) {
	events := []*dto.Event{{Desc: "only-event"}}
	got := renderEvents(events, 10)
	if !strings.Contains(got, "only-event") {
		t.Error("Expected event to appear when N > len(events)")
	}
}

// --- renderMap ---

func TestRenderMapReturnsLoadingWhenGridNil(t *testing.T) {
	world := dto.GameView{Grid: nil}
	got := renderMap(world, nil, 10, 10)
	if got != "Loading..." {
		t.Errorf("renderMap with nil grid = %q, want \"Loading...\"", got)
	}
}

func TestRenderMapReturnsLoadingWhenPlayerNil(t *testing.T) {
	world := dto.GameView{
		Grid:   [][]*dto.Cell{{{TopologyType: dto.TopologyFloor}}},
		Player: nil,
	}
	got := renderMap(world, nil, 10, 10)
	if got != "Loading..." {
		t.Errorf("renderMap with nil player = %q, want \"Loading...\"", got)
	}
}

func TestRenderMapRendersPlayerSymbol(t *testing.T) {
	grid := [][]*dto.Cell{
		{
			{TopologyType: dto.TopologyWall, VisibilityState: dto.VisibilityVisible},
			{TopologyType: dto.TopologyFloor, VisibilityState: dto.VisibilityVisible, Actor: &dto.Actor{Kind: "player"}},
			{TopologyType: dto.TopologyWall, VisibilityState: dto.VisibilityVisible},
		},
	}
	world := dto.GameView{
		Height: 1, Width: 3,
		Grid:   grid,
		Player: &dto.Player{Row: 0, Col: 1},
	}
	got := renderMap(world, nil, 3, 1)
	if !strings.Contains(got, "@") {
		t.Errorf("renderMap should contain player symbol '@', got: %q", got)
	}
}

func TestRenderMapHighlightsInvalidTarget(t *testing.T) {
	// Two cells: player at (0,0), wall at (0,1).
	grid := [][]*dto.Cell{
		{
			{TopologyType: dto.TopologyFloor, VisibilityState: dto.VisibilityVisible},
			{TopologyType: dto.TopologyWall, VisibilityState: dto.VisibilityVisible},
		},
	}
	world := dto.GameView{
		Height: 1, Width: 2,
		Grid:   grid,
		Player: &dto.Player{Row: 0, Col: 0},
	}
	invalid := &dto.Vector{X: 1, Y: 0}
	// Should not panic; just verify it returns a non-empty string.
	got := renderMap(world, invalid, 2, 1)
	if got == "" {
		t.Error("Expected non-empty output with invalid target highlight")
	}
}

// --- buildInventoryEntries ---

func TestBuildInventoryEntriesReturnsNilForNilPlayer(t *testing.T) {
	world := dto.GameView{Player: nil}
	if got := buildInventoryEntries(world, ""); got != nil {
		t.Errorf("Expected nil entries for nil player, got %v", got)
	}
}

func TestBuildInventoryEntriesAllSlots(t *testing.T) {
	sword := &dto.Item{Id: 1, Kind: "weapon", Label: "Sword"}
	bread := &dto.Item{Id: 2, Kind: "food", Label: "Bread"}
	world := dto.GameView{
		Player: &dto.Player{
			Inventory: map[dto.ItemType][]*dto.Item{
				"weapon": {sword},
				"food":   {bread},
			},
			Equipped: map[dto.ItemType]*dto.Item{},
		},
	}
	entries := buildInventoryEntries(world, "")
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}
}

func TestBuildInventoryEntriesFilterByPrefix(t *testing.T) {
	world := dto.GameView{
		Player: &dto.Player{
			Inventory: map[dto.ItemType][]*dto.Item{
				"weapon":           {{Id: 1, Kind: "weapon", Label: "Sword"}},
				"elixir_dexterity": {{Id: 2, Kind: "elixir_dexterity", Label: "Agility potion"}},
				"elixir_strength":  {{Id: 3, Kind: "elixir_strength", Label: "Strength potion"}},
				"food":             {{Id: 4, Kind: "food", Label: "Bread"}},
			},
			Equipped: map[dto.ItemType]*dto.Item{},
		},
	}
	entries := buildInventoryEntries(world, "elixir")
	if len(entries) != 2 {
		t.Errorf("Expected 2 elixir entries, got %d", len(entries))
	}
	for _, e := range entries {
		if !strings.HasPrefix(string(e.item.Kind), "elixir") {
			t.Errorf("Filter returned non-elixir item: %s", e.item.Kind)
		}
	}
}

func TestBuildInventoryEntriesMarksEquippedWeapon(t *testing.T) {
	sword := &dto.Item{Id: 7, Kind: "weapon", Label: "Sword"}
	world := dto.GameView{
		Player: &dto.Player{
			Inventory: map[dto.ItemType][]*dto.Item{
				"weapon": {sword},
			},
			Equipped: map[dto.ItemType]*dto.Item{
				"weapon": sword,
			},
		},
	}
	entries := buildInventoryEntries(world, "weapon")
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}
	if !entries[0].equipped {
		t.Error("Expected weapon to be marked as equipped")
	}
	if entries[0].item.Id != sword.Id {
		t.Errorf("Expected item id %d, got %d", sword.Id, entries[0].item.Id)
	}
}

// --- formatItemDesc ---

func TestFormatItemDescShowsStats(t *testing.T) {
	item := &dto.Item{
		Id:           1,
		Kind:         "food",
		Label:        "Bread",
		VitalsChange: map[dto.VitalType]int{dto.VitalHP: 5},
		AttrsChange:  map[dto.AttrType]int{dto.Strength: 2},
	}
	desc := formatItemDesc(item)
	if !strings.Contains(desc, "HP +5") {
		t.Errorf("Expected HP +5 in desc, got %q", desc)
	}
	if !strings.Contains(desc, "STR +2") {
		t.Errorf("Expected STR +2 in desc, got %q", desc)
	}
}

func TestFormatItemDescEmpty(t *testing.T) {
	item := &dto.Item{Id: 1, Kind: "weapon", Label: "Sword"}
	desc := formatItemDesc(item)
	if desc != "" {
		t.Errorf("Expected empty desc for item with no stats, got %q", desc)
	}
}
