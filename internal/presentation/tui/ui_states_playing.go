package tui

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/unclestep/Rogue/internal/dto"
)

// Cardinal aim angles in radians.
// Y axis points downward in grid coordinates (same as screen space).
const (
	aimRight = 0
	aimDown  = math.Pi / 2
	aimLeft  = math.Pi
	aimUp    = -math.Pi / 2
)

//
// --- PLAYING STATE ---
//

type PlayingState struct {
	Ctx           *UIContext
	invalidTarget *dto.Vector // Set when the player tries an invalid move; cleared on next input.
}

func NewPlayingState(ctx *UIContext) *PlayingState {
	return &PlayingState{Ctx: ctx}
}

func (p *PlayingState) Init() tea.Cmd { return nil }

func (p *PlayingState) Update(msg tea.Msg) (ScreenState, tea.Cmd) {
	switch msg := msg.(type) {
	case dto.GameView:
		switch msg.State {
		case dto.StateGameover:
			return NewGameoverState(p.Ctx), nil
		case dto.StateLobby:
			// Host left — everyone is pushed back to lobby.
			return NewLobbyState(p.Ctx), nil
		}

	case tea.KeyMsg:
		return p.handleInput(msg)
	}

	return nil, nil
}

func (p *PlayingState) handleInput(msg tea.KeyMsg) (ScreenState, tea.Cmd) {
	// Clear any previous invalid-move highlight on every new key press.
	p.invalidTarget = nil

	switch {
	case key.Matches(msg, p.Ctx.Keys.Up):
		p.tryMove(dto.Vector{X: 0, Y: -1})
	case key.Matches(msg, p.Ctx.Keys.Down):
		p.tryMove(dto.Vector{X: 0, Y: 1})
	case key.Matches(msg, p.Ctx.Keys.Right):
		p.tryMove(dto.Vector{X: 1, Y: 0})
	case key.Matches(msg, p.Ctx.Keys.Left):
		p.tryMove(dto.Vector{X: -1, Y: 0})

	case key.Matches(msg, p.Ctx.Keys.AimUp):
		p.sendAim(aimUp)
	case key.Matches(msg, p.Ctx.Keys.AimDown):
		p.sendAim(aimDown)
	case key.Matches(msg, p.Ctx.Keys.AimLeft):
		p.sendAim(aimLeft)
	case key.Matches(msg, p.Ctx.Keys.AimRight):
		p.sendAim(aimRight)

	case key.Matches(msg, p.Ctx.Keys.SkipTurn):
		p.sendAction(dto.ActionWait, 0)

	case key.Matches(msg, p.Ctx.Keys.Inventory):
		return NewInventoryState(p.Ctx, ""), nil

	case key.Matches(msg, p.Ctx.Keys.WeaponSlot):
		return NewInventoryState(p.Ctx, "weapon"), nil
	case key.Matches(msg, p.Ctx.Keys.FoodSlot):
		return NewInventoryState(p.Ctx, "food"), nil
	case key.Matches(msg, p.Ctx.Keys.ElixirSlot):
		return NewInventoryState(p.Ctx, "elixir"), nil
	case key.Matches(msg, p.Ctx.Keys.ScrollSlot):
		return NewInventoryState(p.Ctx, "scroll"), nil

	case key.Matches(msg, p.Ctx.Keys.Esc):
		p.sendAction(dto.ActionLeave, 0)
		return NewMenuState(p.Ctx), nil
	}

	return nil, nil
}

// tryMove validates the target cell before sending ActionMove.
// If the target is not walkable the cell is highlighted red for one render frame.
func (p *PlayingState) tryMove(v dto.Vector) {
	world := p.Ctx.World
	if world.Grid == nil || world.Player == nil {
		return
	}
	r := world.Player.Row + v.Y
	c := world.Player.Col + v.X

	if r < 0 || r >= world.Height || c < 0 || c >= world.Width || !isWalkableTile(world.Grid[r][c]) {
		// Show the red highlight and do not send the command.
		p.invalidTarget = &dto.Vector{X: v.X, Y: v.Y}
		return
	}

	p.Ctx.DataToM <- dto.Command{
		PlaythroughId: p.Ctx.PlaythroughId,
		Action:        dto.ActionMove,
		PlayerUUID:    p.Ctx.PlayerUUID,
		MoveVector:    v,
	}
}

func (p *PlayingState) sendAction(action dto.ActionType, itemID int64) {
	p.Ctx.DataToM <- dto.Command{
		PlaythroughId: p.Ctx.PlaythroughId,
		Action:        action,
		PlayerUUID:    p.Ctx.PlayerUUID,
		ItemID:        itemID,
	}
}

// sendAim sends an ActionAim command with the given direction angle (radians).
// Arrow keys map to cardinal angles defined by the aimUp/Down/Left/Right consts.
func (p *PlayingState) sendAim(angle float64) {
	p.Ctx.DataToM <- dto.Command{
		PlaythroughId: p.Ctx.PlaythroughId,
		Action:        dto.ActionAim,
		PlayerUUID:    p.Ctx.PlayerUUID,
		AimAngle:      angle,
	}
}

func (p *PlayingState) View() string {
	world := p.Ctx.World
	if world.Player == nil || world.Grid == nil {
		return "Loading..."
	}

	totalW := p.Ctx.Width
	totalH := p.Ctx.Height
	if totalW < 60 {
		totalW = 60
	}
	if totalH < 20 {
		totalH = 20
	}

	const (
		leftInnerW  = 20 // player panel inner width
		rightInnerW = 22 // target/events panel inner width
	)

	// Outer widths include 2 border chars each.
	centerInnerW := totalW - (leftInnerW + 2) - (rightInnerW + 2)
	if centerInnerW < 20 {
		centerInnerW = 20
	}

	// Vertical split for center column: depth=1 line, rest goes to map.
	const depthInnerH = 1
	mapInnerH := totalH - (depthInnerH + 2) - 2 // depth outer + map border
	if mapInnerH < 5 {
		mapInnerH = 5
	}

	// Vertical split for right column: ~1/3 target, rest events.
	targetInnerH := totalH/3 - 2
	if targetInnerH < 3 {
		targetInnerH = 3
	}
	eventsInnerH := totalH - (targetInnerH + 2) - 2
	if eventsInnerH < 2 {
		eventsInnerH = 2
	}

	leftInnerH := totalH - 2 // single panel spans full height

	border := stylePanelBorder

	// Left: player stats (full height).
	leftPanel := border.Width(leftInnerW).Height(leftInnerH).Render(
		renderPlayerPanel(world.Player, world.Leaderboard, leftInnerW, leftInnerH),
	)

	// Center top: dungeon depth.
	dungeon := 0
	if world.Player.HUD != nil {
		dungeon = world.Player.HUD.Dungeon
	}
	depthPanel := border.Width(centerInnerW).Height(depthInnerH).Render(
		renderDepthPanel(dungeon),
	)

	// Center bottom: map.
	mapPanel := border.Width(centerInnerW).Height(mapInnerH).Render(
		renderMap(world, p.invalidTarget, centerInnerW, mapInnerH),
	)
	centerPanel := lipgloss.JoinVertical(lipgloss.Left, depthPanel, mapPanel)

	// Right top: adjacent combat target.
	target := findAdjacentTarget(world)
	targetPanel := border.Width(rightInnerW).Height(targetInnerH).Render(
		renderTargetPanel(target, rightInnerW, targetInnerH),
	)

	// Right bottom: event log.
	eventsPanel := border.Width(rightInnerW).Height(eventsInnerH).Render(
		renderEventsPanel(world.Events, eventsInnerH),
	)
	rightPanel := lipgloss.JoinVertical(lipgloss.Left, targetPanel, eventsPanel)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, centerPanel, rightPanel)
}

// findAdjacentTarget scans the 8 cells around the player and returns the first
// non-player visible actor, or nil if none is found.
func findAdjacentTarget(world dto.GameView) *dto.Actor {
	if world.Grid == nil || world.Player == nil {
		return nil
	}
	pr, pc := world.Player.Row, world.Player.Col
	dirs := [8][2]int{
		{0, -1}, {0, 1}, {-1, 0}, {1, 0},
		{-1, -1}, {1, -1}, {-1, 1}, {1, 1},
	}
	for _, d := range dirs {
		r, c := pr+d[1], pc+d[0]
		if r < 0 || r >= world.Height || c < 0 || c >= world.Width {
			continue
		}
		cell := world.Grid[r][c]
		if cell.Actor != nil && !strings.Contains(string(cell.Actor.Kind), "player") {
			return cell.Actor
		}
	}
	return nil
}

//
// --- INVENTORY STATE ---
//

// Consumable slot order — excludes keys (used via door interaction) and treasures (not usable).
var slotOrder = []dto.ItemType{"weapon", "food", "elixir", "scroll"}

type inventoryEntry struct {
	item     *dto.Item
	equipped bool
}

type InventoryState struct {
	Ctx        *UIContext
	filterType dto.ItemType
	entries    []inventoryEntry
}

func NewInventoryState(ctx *UIContext, filterType dto.ItemType) *InventoryState {
	entries := buildInventoryEntries(ctx.World, filterType)
	return &InventoryState{Ctx: ctx, filterType: filterType, entries: entries}
}

func buildInventoryEntries(world dto.GameView, filterType dto.ItemType) []inventoryEntry {
	if world.Player == nil {
		return nil
	}
	equipped := world.Player.Equipped
	inventory := world.Player.Inventory

	// Determine which slots to show.
	var slots []dto.ItemType
	if filterType != "" {
		// Iterate actual inventory keys so subtype keys like "elixir_dexterity"
		// are matched when the filter is "elixir". slotOrder only holds generic
		// category names and would miss all subtypes.
		for s := range inventory {
			if strings.HasPrefix(string(s), string(filterType)) {
				slots = append(slots, s)
			}
		}
		sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })
	} else {
		slots = slotOrder
	}

	var entries []inventoryEntry
	for _, slot := range slots {
		items, ok := inventory[slot]
		if !ok || len(items) == 0 {
			continue
		}
		sorted := make([]*dto.Item, len(items))
		copy(sorted, items)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Id < sorted[j].Id })

		for _, item := range sorted {
			isEquipped := equipped[slot] != nil && equipped[slot].Id == item.Id
			entries = append(entries, inventoryEntry{item: item, equipped: isEquipped})
		}
	}
	return entries
}

func (inv *InventoryState) Init() tea.Cmd { return nil }

func (inv *InventoryState) Update(msg tea.Msg) (ScreenState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, inv.Ctx.Keys.Esc) {
			return NewPlayingState(inv.Ctx), nil
		}

		// Number key selection: 0 = unequip weapon, 1-9 = use item at that index.
		ch := msg.String()
		if len(ch) == 1 && ch[0] >= '0' && ch[0] <= '9' {
			idx := int(ch[0] - '0')

			// 0 = unequip weapon (only for weapon filter)
			if idx == 0 {
				if inv.filterType == "weapon" {
					return inv.unequipWeapon()
				}
				return nil, nil
			}

			// 1-9 = select item at that position.
			itemIdx := idx - 1
			if itemIdx >= 0 && itemIdx < len(inv.entries) {
				return inv.useItem(inv.entries[itemIdx])
			}
		}
	}

	return nil, nil
}

func (inv *InventoryState) unequipWeapon() (ScreenState, tea.Cmd) {
	world := inv.Ctx.World
	if world.Player == nil {
		return NewPlayingState(inv.Ctx), nil
	}
	equipped := world.Player.Equipped
	weapon, ok := equipped["weapon"]
	if !ok || weapon == nil {
		return NewPlayingState(inv.Ctx), nil
	}

	inv.Ctx.DataToM <- dto.Command{
		PlaythroughId: inv.Ctx.PlaythroughId,
		Action:        dto.ActionUnequipWeapon,
		PlayerUUID:    inv.Ctx.PlayerUUID,
		ItemID:        int64(weapon.Id),
	}
	return NewPlayingState(inv.Ctx), nil
}

func (inv *InventoryState) useItem(e inventoryEntry) (ScreenState, tea.Cmd) {
	item := e.item
	itemID := int64(item.Id)

	var action dto.ActionType
	switch {
	case strings.HasPrefix(string(item.Kind), "weapon"):
		if e.equipped {
			action = dto.ActionUnequipWeapon
		} else {
			action = dto.ActionEquipWeapon
		}
	default:
		action = dto.ActionConsumeItem
	}

	inv.Ctx.DataToM <- dto.Command{
		PlaythroughId: inv.Ctx.PlaythroughId,
		Action:        action,
		PlayerUUID:    inv.Ctx.PlayerUUID,
		ItemID:        itemID,
	}
	return NewPlayingState(inv.Ctx), nil
}

func (inv *InventoryState) View() string {
	var sb strings.Builder

	title := "Inventory"
	if inv.filterType != "" {
		title = fmt.Sprintf("Inventory — %s", strings.Title(string(inv.filterType)))
	}
	sb.WriteString(stylePanelTitle.Render(title))
	sb.WriteString("\n\n")

	if len(inv.entries) == 0 {
		sb.WriteString(styleNeutral.Render("  Empty"))
		sb.WriteString("\n")
	} else {
		for i, e := range inv.entries {
			if i >= 9 {
				break
			}
			num := fmt.Sprintf("[%d] ", i+1)
			label := string(e.item.Label)
			if e.equipped {
				label += " (equipped)"
			}

			desc := formatItemDesc(e.item)
			if desc != "" {
				label += "  " + styleNeutral.Render(desc)
			}
			sb.WriteString(fmt.Sprintf("  %s%s\n", styleBuff.Render(num), label))
		}
	}

	// Weapon: show unequip option
	if inv.filterType == "weapon" {
		equipped := inv.Ctx.World.Player.Equipped
		if w, ok := equipped["weapon"]; ok && w != nil {
			sb.WriteString(fmt.Sprintf("\n  %s%s\n", styleBuff.Render("[0] "), "Unequip weapon"))
		}
	}

	sb.WriteString("\n" + styleNeutral.Render("  [Esc] Back"))

	return lipgloss.NewStyle().Padding(1, 2).Render(sb.String())
}

// formatItemDesc builds a short description of the item's effects.
func formatItemDesc(item *dto.Item) string {
	parts := make([]string, 0, 4)
	if v, ok := item.VitalsChange[dto.VitalHP]; ok && v != 0 {
		parts = append(parts, fmt.Sprintf("HP %+d", v))
	}
	if v, ok := item.AttrsChange[dto.Strength]; ok && v != 0 {
		parts = append(parts, fmt.Sprintf("STR %+d", v))
	}
	if v, ok := item.AttrsChange[dto.Dexterity]; ok && v != 0 {
		parts = append(parts, fmt.Sprintf("DEX %+d", v))
	}
	if v, ok := item.AttrsChange[dto.MaxHealth]; ok && v != 0 {
		parts = append(parts, fmt.Sprintf("MaxHP %+d", v))
	}
	for _, eff := range item.Effects {
		parts = append(parts, eff)
	}
	return strings.Join(parts, "  ")
}
