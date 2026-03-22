package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/unclestep/Rogue/internal/dto"
)

// deadASCII is the block-letter "DEAD" rendered on the game-over screen.
const deadASCII = "██████╗ ███████╗ █████╗ ██████╗ \n" +
	"██╔══██╗██╔════╝██╔══██╗██╔══██╗\n" +
	"██║  ██║█████╗  ███████║██║  ██║\n" +
	"██║  ██║██╔══╝  ██╔══██║██║  ██║\n" +
	"██████╔╝███████╗██║  ██║██████╔╝\n" +
	"╚═════╝ ╚══════╝╚═╝  ╚═╝╚═════╝ "

//
// --- GAMEOVER STATE ---
//

type GameoverState struct {
	Ctx *UIContext
}

func NewGameoverState(ctx *UIContext) *GameoverState {
	return &GameoverState{Ctx: ctx}
}

func (g *GameoverState) Init() tea.Cmd { return nil }

func (g *GameoverState) Update(msg tea.Msg) (ScreenState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		isHost := g.Ctx.World.Player != nil && g.Ctx.World.Player.IsHost

		switch {
		case key.Matches(msg, g.Ctx.Keys.Return) && isHost:
			g.Ctx.DataToM <- dto.Command{
				PlaythroughId: g.Ctx.PlaythroughId,
				Action:        dto.ActionYes,
				PlayerUUID:    g.Ctx.PlayerUUID,
			}
			return NewMenuState(g.Ctx), nil

		case key.Matches(msg, g.Ctx.Keys.Esc):
			g.Ctx.DataToM <- dto.Command{
				PlaythroughId: g.Ctx.PlaythroughId,
				Action:        dto.ActionLeave,
				PlayerUUID:    g.Ctx.PlayerUUID,
			}
			return nil, tea.Quit
		}
	}
	return nil, nil
}

func (g *GameoverState) View() string {
	w, h := g.Ctx.Width, g.Ctx.Height
	isHost := g.Ctx.World.Player != nil && g.Ctx.World.Player.IsHost

	const panelW = 46
	sep := styleAlienFaint.Render(strings.Repeat("─", panelW-6))

	// "DEAD" ASCII banner in red
	banner := styleAlienDanger.Render(deadASCII)

	// Run statistics
	stats := g.renderStats()

	// Controls
	var controls string
	if isHost {
		controls = styleAlienDim.Render("[Enter]") +
			"  " + styleAlienPrimary.Render("Return to Menu") +
			"     " + styleAlienDim.Render("[Esc]") +
			"  " + styleAlienFaint.Render("Quit")
	} else {
		controls = styleAlienDim.Render("[Esc]") +
			"  " + styleAlienFaint.Render("Quit")
	}

	body := lipgloss.JoinVertical(lipgloss.Center,
		banner,
		"",
		sep,
		"",
		stats,
		"",
		sep,
		"",
		controls,
	)

	panel := styleAlienBorder.Width(panelW).Padding(1, 3).Render(body)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, panel)
}

func (g *GameoverState) renderStats() string {
	p := g.Ctx.World.Player
	if p == nil || p.RunStats == nil {
		return styleAlienFaint.Render("No statistics available.")
	}
	s := p.RunStats

	label := func(k string) string { return styleAlienDim.Render(k) }
	value := func(v int) string { return styleAlienPrimary.Render(fmt.Sprintf("%d", v)) }
	row := func(k string, v int) string {
		return label(fmt.Sprintf("%-22s", k)) + value(v)
	}

	rows := []string{
		row("Deepest level", s.DeepestLevel),
		row("Monsters defeated", s.MonstersDefeated),
		row("Total treasure", s.TotalTreasure),
		row("Food consumed", s.FoodConsumed),
		row("Elixirs drunk", s.ElixirsDrunk),
		row("Scrolls read", s.ScrollsRead),
		row("Hits dealt", s.HitsDealt),
		row("Hits received", s.HitsReceived),
		row("Tiles traveled", s.TilesTraveled),
	}
	return strings.Join(rows, "\n")
}
