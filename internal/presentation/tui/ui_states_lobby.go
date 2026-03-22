package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/unclestep/Rogue/internal/dto"
)

//
// --- LOBBY STATE ---
//

type LobbyState struct {
	Ctx *UIContext
}

func NewLobbyState(ctx *UIContext) *LobbyState {
	return &LobbyState{Ctx: ctx}
}

func (l *LobbyState) Init() tea.Cmd { return nil }

func (l *LobbyState) Update(msg tea.Msg) (ScreenState, tea.Cmd) {
	switch msg := msg.(type) {
	case dto.GameView:
		switch msg.State {
		case dto.StatePlaying:
			return NewPlayingState(l.Ctx), nil
		case dto.StateUnknown:
			return NewMenuState(l.Ctx), nil
		}

	case tea.KeyMsg:
		world := l.Ctx.World
		isHost := world.Player != nil && world.Player.IsHost

		switch {
		case key.Matches(msg, l.Ctx.Keys.Return) && isHost:
			l.Ctx.DataToM <- dto.Command{
				PlaythroughId: l.Ctx.PlaythroughId,
				Action:        dto.ActionYes,
				PlayerUUID:    l.Ctx.PlayerUUID,
			}

		case key.Matches(msg, l.Ctx.Keys.Esc):
			l.Ctx.DataToM <- dto.Command{
				PlaythroughId: l.Ctx.PlaythroughId,
				Action:        dto.ActionLeave,
				PlayerUUID:    l.Ctx.PlayerUUID,
			}
			return NewMenuState(l.Ctx), nil
		}
	}

	return nil, nil
}

func (l *LobbyState) View() string {
	w, h := l.Ctx.Width, l.Ctx.Height
	world := l.Ctx.World
	isHost := world.Player != nil && world.Player.IsHost

	sid := l.Ctx.PlaythroughId
	if sid == "" {
		sid = "—"
	}

	const panelW = 50
	sep := styleAlienFaint.Render(strings.Repeat("─", panelW-6))

	// Header
	header := styleAlienTitle.Render("◆  LOBBY")

	// Session ID block
	sidLabel := styleAlienFaint.Render("SESSION ID")
	sidValue := styleAlienPrimary.Render(sid)
	shareHint := styleAlienFaint.Render("Share this ID with friends to join.")

	// Players
	playerList := renderLobbyPlayers(world.LobbyPlayers)

	// Controls
	var controls string
	if isHost {
		controls = styleAlienDim.Render("[Enter]") +
			"  " + styleAlienPrimary.Render("Start Game") +
			"     " + styleAlienDim.Render("[Esc]") +
			"  " + styleAlienFaint.Render("Leave")
	} else {
		controls = styleAlienFaint.Render("Waiting for host…") +
			"     " + styleAlienDim.Render("[Esc]") +
			"  " + styleAlienFaint.Render("Leave")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		sidLabel,
		sidValue,
		shareHint,
		"",
		sep,
		"",
		playerList,
		"",
		sep,
		"",
		controls,
	)

	panel := styleAlienBorder.Width(panelW).Padding(1, 3).Render(body)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, panel)
}

// renderLobbyPlayers builds a styled player-list string for the lobby view.
func renderLobbyPlayers(players []dto.LobbyPlayer) string {
	if len(players) == 0 {
		return styleAlienFaint.Render("PLAYERS  —")
	}
	lines := make([]string, 0, len(players)+1)
	lines = append(lines, styleAlienDim.Render(fmt.Sprintf("PLAYERS  (%d)", len(players))))
	guest := 1
	for _, p := range players {
		name := p.Nickname
		if p.IsHost {
			if name == "" {
				name = "Host"
			}
			lines = append(lines,
				styleAlienDanger.Render("  ★")+" "+styleAlienPrimary.Render(name),
			)
		} else {
			if name == "" {
				name = fmt.Sprintf("Guest %d", guest)
			}
			lines = append(lines,
				styleAlienFaint.Render("  ·")+" "+styleAlienDim.Render(name),
			)
			guest++
		}
	}
	return strings.Join(lines, "\n")
}
