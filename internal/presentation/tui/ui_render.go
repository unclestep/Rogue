package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/unclestep/Rogue/internal/dto"
)

// Lipgloss styles used across renderers.
var (
	// Map rendering
	styleExplored = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleUnknown  = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
	styleInvalid  = lipgloss.NewStyle().Background(lipgloss.Color("1")).Foreground(lipgloss.Color("15"))

	// Legacy HUD (kept for test-compatible renderHUD / buildHPBar)
	styleHUD       = lipgloss.NewStyle().Bold(true)
	styleHPGood    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleHPLow     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleEventPane = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	// Panel chrome
	stylePanelBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("238"))
	stylePanelTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250"))

	// Actor colors (visible on map)
	styleActorPlayer  = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	styleActorZombie  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleActorVampire = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleActorGhost   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleActorOgre    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleActorSnake   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleActorMimic   = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	styleActorDefault = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	// Item colors (visible on map)
	styleItemWeapon   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	styleItemFood     = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleItemElixir   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleItemScroll   = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleItemTreasure = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	styleItemDefault  = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	// Tile colors (visible on map)
	styleTileWall  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleTileFloor = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleTileDoor  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleTileExit  = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)

	// Door/key keyhole colors — each keyhole value maps to a distinct color.
	keyholeColors = []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("1")), // red
		lipgloss.NewStyle().Foreground(lipgloss.Color("2")), // green
		lipgloss.NewStyle().Foreground(lipgloss.Color("3")), // yellow
		lipgloss.NewStyle().Foreground(lipgloss.Color("4")), // blue
		lipgloss.NewStyle().Foreground(lipgloss.Color("5")), // purple
		lipgloss.NewStyle().Foreground(lipgloss.Color("6")), // cyan
	}

	// Effect label colors
	styleBuff    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleDebuff  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleNeutral = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	alienGreenBright = lipgloss.Color("82")  // #5fd700 acid green
	alienGreenDim    = lipgloss.Color("28")  // #008700 dark green
	alienGreenFaint  = lipgloss.Color("22")  // #005f00 very dark green
	alienRed         = lipgloss.Color("160") // #d70000 deep red
	alienBlack       = lipgloss.Color("232") // #080808 near black

	styleAlienTitle    = lipgloss.NewStyle().Foreground(alienGreenBright).Bold(true)
	styleAlienPrimary  = lipgloss.NewStyle().Foreground(alienGreenBright)
	styleAlienDim      = lipgloss.NewStyle().Foreground(alienGreenDim)
	styleAlienFaint    = lipgloss.NewStyle().Foreground(alienGreenFaint)
	styleAlienDanger   = lipgloss.NewStyle().Foreground(alienRed).Bold(true)
	styleAlienSelected = lipgloss.NewStyle().Foreground(alienBlack).Background(alienGreenBright).Bold(true)
	styleAlienBorder   = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(alienGreenDim)
	styleAlienErr = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// renderMap builds a dungeon view centred on the player.
// invalidTarget highlights the tile the player cannot move to (red).
func renderMap(world dto.GameView, invalidTarget *dto.Vector, viewW, viewH int) string {
	if world.Grid == nil || world.Player == nil {
		return "Loading..."
	}

	mapH := world.Height
	mapW := world.Width
	if viewH <= 0 || viewH > mapH {
		viewH = mapH
	}
	if viewW <= 0 || viewW > mapW {
		viewW = mapW
	}

	pr, pc := world.Player.Row, world.Player.Col
	startR := pr - viewH/2
	startC := pc - viewW/2
	if startR < 0 {
		startR = 0
	}
	if startC < 0 {
		startC = 0
	}
	if startR+viewH > mapH {
		startR = mapH - viewH
	}
	if startC+viewW > mapW {
		startC = mapW - viewW
	}

	var sb strings.Builder
	for r := startR; r < startR+viewH; r++ {
		for c := startC; c < startC+viewW; c++ {
			sb.WriteString(renderCell(world, r, c, invalidTarget))
		}
		if r < startR+viewH-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// renderCell renders a single grid cell as a styled string.
func renderCell(world dto.GameView, r, c int, invalidTarget *dto.Vector) string {
	cell := world.Grid[r][c]

	if invalidTarget != nil {
		tr := world.Player.Row + invalidTarget.Y
		tc := world.Player.Col + invalidTarget.X
		if r == tr && c == tc {
			return styleInvalid.Render(tileChar(cell))
		}
	}

	switch cell.VisibilityState {
	case dto.VisibilityVisible:
		return coloredVisibleCellChar(cell)
	case dto.VisibilityExplored:
		return styleExplored.Render(tileChar(cell))
	default:
		return styleUnknown.Render(" ")
	}
}

// visibleCellChar returns the plain character for a fully visible cell.
// Kept for internal use; see coloredVisibleCellChar for colored rendering.
func visibleCellChar(cell *dto.Cell) string {
	if cell.Actor != nil {
		return actorChar(cell.Actor)
	}
	if cell.Item != nil {
		return itemChar(cell.Item)
	}
	return tileChar(cell)
}

// coloredVisibleCellChar returns a colored string for a fully visible cell.
func coloredVisibleCellChar(cell *dto.Cell) string {
	if cell.Actor != nil {
		return coloredActorChar(cell.Actor)
	}
	if cell.Item != nil {
		return coloredItemChar(cell.Item)
	}
	// Color locked doors by their keyhole.
	if cell.TopologyType == dto.TopologyClosedDoor && cell.DoorKeyhole > 0 {
		return keyholeStyle(cell.DoorKeyhole).Render(tileChar(cell))
	}
	return coloredTileChar(cell)
}

// coloredActorChar applies lipgloss color to an actor symbol.
func coloredActorChar(a *dto.Actor) string {
	ch := actorChar(a)
	kind := string(a.Kind)
	switch {
	case strings.Contains(kind, "player"):
		return styleActorPlayer.Render(ch)
	case strings.Contains(kind, "vampire"):
		return styleActorVampire.Render(ch)
	case strings.Contains(kind, "zombie"):
		return styleActorZombie.Render(ch)
	case strings.Contains(kind, "ghost"):
		return styleActorGhost.Render(ch)
	case strings.Contains(kind, "ogre"):
		return styleActorOgre.Render(ch)
	case strings.Contains(kind, "snake"):
		return styleActorSnake.Render(ch)
	case strings.Contains(kind, "mimic"):
		return styleActorMimic.Render(ch)
	default:
		return styleActorDefault.Render(ch)
	}
}

// keyholeStyle returns a lipgloss style for a given keyhole value.
func keyholeStyle(keyhole dto.Keyhole) lipgloss.Style {
	if keyhole <= 0 || len(keyholeColors) == 0 {
		return styleTileDoor
	}
	return keyholeColors[int(keyhole)%len(keyholeColors)]
}

// coloredItemChar applies lipgloss color to an item symbol.
func coloredItemChar(i *dto.Item) string {
	ch := itemChar(i)
	switch {
	case i.Kind == "weapon":
		return styleItemWeapon.Render(ch)
	case i.Kind == "food":
		return styleItemFood.Render(ch)
	case i.Kind == "elixir":
		return styleItemElixir.Render(ch)
	case i.Kind == "scroll":
		return styleItemScroll.Render(ch)
	case i.Kind == "key":
		return keyholeStyle(i.Keyhole).Render(ch)
	case i.Kind == "treasure":
		return styleItemTreasure.Render(ch)
	default:
		return styleItemDefault.Render(ch)
	}
}

// coloredTileChar applies lipgloss color to a tile symbol.
func coloredTileChar(cell *dto.Cell) string {
	ch := tileChar(cell)
	switch cell.TopologyType {
	case dto.TopologyWall:
		return styleTileWall.Render(ch)
	case dto.TopologyFloor, dto.TopologyCorridor:
		return styleTileFloor.Render(ch)
	case dto.TopologyOpenDoor, dto.TopologyClosedDoor:
		return styleTileDoor.Render(ch)
	case dto.TopologyExit:
		return styleTileExit.Render(ch)
	default:
		return ch
	}
}

// tileChar maps a TopologyType to its ASCII representation.
func tileChar(cell *dto.Cell) string {
	switch cell.TopologyType {
	case dto.TopologyWall:
		return "#"
	case dto.TopologyFloor:
		return "·"
	case dto.TopologyCorridor:
		return "·"
	case dto.TopologyOpenDoor:
		return "/"
	case dto.TopologyClosedDoor:
		return "%"
	case dto.TopologyExit:
		return ">"
	default:
		return " "
	}
}

// actorChar returns the symbol for an actor based on its kind string.
func actorChar(a *dto.Actor) string {
	if strings.Contains(string(a.Kind), "player") {
		return "@"
	}
	if strings.Contains(string(a.Kind), "vampire") {
		return "V"
	}
	if strings.Contains(string(a.Kind), "zombie") {
		return "Z"
	}
	if strings.Contains(string(a.Kind), "ghost") {
		return "G"
	}
	if strings.Contains(string(a.Kind), "ogre") {
		return "O"
	}
	if strings.Contains(string(a.Kind), "snake") {
		return "S"
	}
	if strings.Contains(string(a.Kind), "mimic") {
		return "M"
	}
	return "?"
}

// itemChar returns the symbol for an item.
func itemChar(i *dto.Item) string {
	switch i.Kind {
	case "weapon":
		return "("
	case "food":
		return "+"
	case "elixir":
		return "!"
	case "scroll":
		return "["
	case "key":
		return "K"
	case "treasure":
		return "$"
	default:
		return "?"
	}
}

// renderHUD renders the player status bar (legacy, kept for test compatibility).
func renderHUD(hud *dto.HUD) string {
	if hud == nil {
		return ""
	}

	hpBar := buildHPBar(hud.HP, hud.MaxHP, 12)
	hpStyle := styleHPGood
	if hud.MaxHP > 0 && hud.HP*3 < hud.MaxHP {
		hpStyle = styleHPLow
	}

	return styleHUD.Render(fmt.Sprintf(
		" %s %s  STR %d  DEX %d  LVL %d  GOLD %d ",
		hpStyle.Render(fmt.Sprintf("HP %d/%d", hud.HP, hud.MaxHP)),
		hpBar,
		hud.Strength,
		hud.Dexterity,
		hud.Dungeon,
		hud.Treasure,
	))
}

func buildHPBar(hp, maxHP, width int) string {
	if maxHP <= 0 {
		return ""
	}
	filled := hp * width / maxHP
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	if hp*3 < maxHP {
		return styleHPLow.Render(bar)
	}
	return styleHPGood.Render(bar)
}

// renderEvents renders the last n turn events, newest first (legacy, kept for test compatibility).
func renderEvents(events []*dto.Event, n int) string {
	if len(events) == 0 {
		return ""
	}
	start := len(events) - n
	if start < 0 {
		start = 0
	}
	lines := make([]string, 0, n)
	for i := len(events) - 1; i >= start; i-- {
		lines = append(lines, events[i].Desc)
	}
	return styleEventPane.Render(strings.Join(lines, "\n"))
}

// isWalkableTile returns true when a player can step onto the given cell.
func isWalkableTile(cell *dto.Cell) bool {
	if cell == nil {
		return false
	}
	switch cell.TopologyType {
	case dto.TopologyFloor, dto.TopologyCorridor, dto.TopologyOpenDoor, dto.TopologyClosedDoor, dto.TopologyExit:
		return true
	}
	return false
}

// Panel renderers

// renderPlayerPanel renders the left player-stats panel (inner content only).
// The leaderboard is placed below the stats and vertically centred within the
// remaining panel height so it appears in the lower-middle of the column.
func renderPlayerPanel(player *dto.Player, leaderboard []dto.LeaderboardEntry, w, h int) string {
	if player == nil {
		return ""
	}
	hud := player.HUD
	actor := player.Actor
	if hud == nil {
		return ""
	}

	var sb strings.Builder

	// HP
	hpStyle := styleHPGood
	if hud.MaxHP > 0 && hud.HP*3 < hud.MaxHP {
		hpStyle = styleHPLow
	}
	barW := w - 4
	if barW < 4 {
		barW = 4
	}
	sb.WriteString(hpStyle.Render(fmt.Sprintf("HP  %d/%d", hud.HP, hud.MaxHP)))
	sb.WriteByte('\n')
	sb.WriteString(buildHPBar(hud.HP, hud.MaxHP, barW))
	sb.WriteByte('\n')

	// Stamina
	if actor != nil {
		sta := actor.BaseVitals[dto.VitalStamina]
		maxSta := actor.BaseAttrs[dto.MaxStamina]
		sb.WriteString(fmt.Sprintf("STA %d/%d", sta, maxSta))
		sb.WriteByte('\n')
	}

	// Attributes
	sb.WriteString(fmt.Sprintf("STR %-5d DEX %d", hud.Strength, hud.Dexterity))
	sb.WriteByte('\n')
	sb.WriteString(fmt.Sprintf("GOLD %d", hud.Treasure))

	// Applied effects
	if actor != nil && len(actor.AppliedEffects) > 0 {
		sb.WriteString("\n─────────────\nEffects:")
		for _, eff := range actor.AppliedEffects {
			for _, s := range eff.Statuses {
				sb.WriteString("\n" + styleNeutral.Render("● "+s))
			}
			for _, b := range eff.Buffs {
				sb.WriteString("\n" + styleBuff.Render("▲ "+b))
			}
			for _, d := range eff.Debuffs {
				sb.WriteString("\n" + styleDebuff.Render("▼ "+d))
			}
		}
	}

	statsStr := sb.String()

	// --- Leaderboard: vertically centred in the remaining panel space ---
	lbStr := renderLeaderboard(leaderboard, w)
	if lbStr == "" {
		return statsStr
	}

	statsLines := strings.Count(statsStr, "\n") + 1
	lbLines := strings.Count(lbStr, "\n") + 1

	// Centre the leaderboard block within the lower half of the panel.
	// lbTop is the row where the leaderboard should start (0-indexed).
	lbTop := h/2 - lbLines/2
	if lbTop < statsLines+1 {
		lbTop = statsLines + 1
	}
	gap := lbTop - statsLines
	if gap < 1 {
		gap = 1
	}

	return statsStr + strings.Repeat("\n", gap) + lbStr
}

// renderLeaderboard builds a compact three-column table (name / gold / level)
// sorted by total treasure descending.
func renderLeaderboard(entries []dto.LeaderboardEntry, w int) string {
	if len(entries) == 0 {
		return ""
	}

	nameW := w - 13 // leave room for two 5-char numeric columns + spacing
	if nameW < 6 {
		nameW = 6
	}
	if nameW > 12 {
		nameW = 12
	}
	rowFmt := fmt.Sprintf("%%-%ds %%5s %%4s", nameW)

	var sb strings.Builder
	sb.WriteString(stylePanelTitle.Render("LEADERBOARD"))
	sb.WriteByte('\n')
	header := styleNeutral.Render(
		fmt.Sprintf(rowFmt, truncate("NAME", nameW), "GOLD", "LVL"),
	)
	sb.WriteString(header)
	sb.WriteByte('\n')
	sb.WriteString(styleNeutral.Render(strings.Repeat("─", nameW+11)))

	for i, e := range entries {
		sb.WriteByte('\n')
		nick := truncate(e.Nickname, nameW)
		row := fmt.Sprintf(rowFmt, nick,
			fmt.Sprintf("%d", e.TotalTreasure),
			fmt.Sprintf("%d", e.DeepestLevel),
		)
		if i == 0 {
			sb.WriteString(styleBuff.Render(row)) // gold highlight for leader
		} else {
			sb.WriteString(styleNeutral.Render(row))
		}
	}
	return sb.String()
}

// truncate shortens s to at most n runes, appending "…" if cut.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

// renderTargetPanel renders the right target-stats panel (inner content only).
// target is nil when no enemy is adjacent.
func renderTargetPanel(target *dto.Actor, w, _ int) string {
	if target == nil {
		return styleNeutral.Render("No target")
	}

	var sb strings.Builder

	// Kind header
	sb.WriteString(stylePanelTitle.Render(string(target.Kind)))
	sb.WriteByte('\n')

	// HP
	hp := target.BaseVitals[dto.VitalHP]
	maxHP := target.BaseAttrs[dto.MaxHealth]
	hpStyle := styleHPGood
	if maxHP > 0 && hp*3 < maxHP {
		hpStyle = styleHPLow
	}
	barW := w - 4
	if barW < 4 {
		barW = 4
	}
	sb.WriteString(hpStyle.Render(fmt.Sprintf("HP  %d/%d", hp, maxHP)))
	sb.WriteByte('\n')
	sb.WriteString(buildHPBar(hp, maxHP, barW))
	sb.WriteByte('\n')

	// Stamina
	sta := target.BaseVitals[dto.VitalStamina]
	maxSta := target.BaseAttrs[dto.MaxStamina]
	sb.WriteString(fmt.Sprintf("STA %d/%d", sta, maxSta))
	sb.WriteByte('\n')

	// Attributes
	str := target.BaseAttrs[dto.Strength]
	dex := target.BaseAttrs[dto.Dexterity]
	sb.WriteString(fmt.Sprintf("STR %-5d DEX %d", str, dex))

	// Applied effects
	if len(target.AppliedEffects) > 0 {
		sb.WriteString("\n─────────────\nEffects:")
		for _, eff := range target.AppliedEffects {
			for _, s := range eff.Statuses {
				sb.WriteString("\n" + styleNeutral.Render("● "+s))
			}
			for _, b := range eff.Buffs {
				sb.WriteString("\n" + styleBuff.Render("▲ "+b))
			}
			for _, d := range eff.Debuffs {
				sb.WriteString("\n" + styleDebuff.Render("▼ "+d))
			}
		}
	}

	return sb.String()
}

// renderDepthPanel renders the compact depth indicator (inner content, 1 line).
func renderDepthPanel(dungeon int) string {
	return stylePanelTitle.Render(fmt.Sprintf("◆  DEPTH  %d", dungeon))
}

// renderEventsPanel renders the event log panel (inner content only).
// Shows at most h events, newest first.
func renderEventsPanel(events []*dto.Event, h int) string {
	if len(events) == 0 {
		return styleNeutral.Render("No events yet")
	}
	n := h
	if n < 1 {
		n = 1
	}
	start := len(events) - n
	if start < 0 {
		start = 0
	}
	lines := make([]string, 0, n)
	for i := len(events) - 1; i >= start; i-- {
		lines = append(lines, styleEventPane.Render(events[i].Desc))
	}
	return strings.Join(lines, "\n")
}
