package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/unclestep/Rogue/internal/dto"
)

// gougeASCII is the block-letter logotype rendered in the main menu.
const gougeASCII = " ██████╗  ██████╗ ██╗   ██╗ ██████╗ ███████╗\n" +
	"██╔════╝ ██╔═══██╗██║   ██║██╔════╝ ██╔════╝\n" +
	"██║  ███╗██║   ██║██║   ██║██║  ███╗█████╗  \n" +
	"██║   ██║██║   ██║██║   ██║██║   ██║██╔══╝  \n" +
	"╚██████╔╝╚██████╔╝╚██████╔╝╚██████╔╝███████╗\n" +
	" ╚═════╝  ╚═════╝  ╚═════╝  ╚═════╝ ╚══════╝"

//
// --- MENU STATE ---
//

type menuItem struct {
	title string
	desc  string
}

type MenuState struct {
	Ctx     *UIContext
	items   []menuItem
	cursor  int
	waiting bool
}

func NewMenuState(ctx *UIContext) *MenuState {
	items := []menuItem{
		{title: "New Game", desc: "Start a fresh dungeon run"},
	}
	if ctx.LastPlaythroughId != "" {
		items = append(items, menuItem{
			title: "Continue",
			desc:  fmt.Sprintf("Resume session %s", shortID(ctx.LastPlaythroughId)),
		})
	}
	items = append(items,
		menuItem{title: "Join Game", desc: "Enter a session ID to join another player"},
		menuItem{title: "Quit", desc: "Exit the game"},
	)
	return &MenuState{Ctx: ctx, items: items}
}

func (m *MenuState) Init() tea.Cmd { return nil }

func (m *MenuState) Update(msg tea.Msg) (ScreenState, tea.Cmd) {
	switch msg := msg.(type) {
	case dto.GameView:
		if m.waiting {
			switch msg.State {
			case dto.StateLobby:
				return NewLobbyState(m.Ctx), nil
			case dto.StatePlaying:
				return NewPlayingState(m.Ctx), nil
			case dto.StateUnknown:
				m.Ctx.LastPlaythroughId = ""
				return NewMenuState(m.Ctx), nil
			}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return nil, tea.Quit
		case "up", "w", "k":
			if m.cursor > 0 {
				m.cursor--
			} else if m.cursor == 0 {
				m.cursor = len(m.items) - 1
			}
		case "down", "s", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			} else if m.cursor == len(m.items)-1 {
				m.cursor = 0
			}
		case "enter":
			return m.handleSelect(m.items[m.cursor].title)
		}
	}

	return nil, nil
}

func (m *MenuState) handleSelect(title string) (ScreenState, tea.Cmd) {
	switch title {
	case "New Game":
		m.Ctx.DataToM <- dto.Command{
			Action:         dto.ActionJoin,
			PlayerUUID:     m.Ctx.PlayerUUID,
			PlayerNickname: m.Ctx.Nickname,
			PlaythroughParams: &dto.PlaythroughParams{
				RulesId: 0,
				Seed:    time.Now().UnixNano(),
			},
		}
		m.waiting = true

	case "Continue":
		m.Ctx.DataToM <- dto.Command{
			Action:         dto.ActionJoin,
			PlayerUUID:     m.Ctx.PlayerUUID,
			PlayerNickname: m.Ctx.Nickname,
			PlaythroughId:  m.Ctx.LastPlaythroughId,
		}
		m.waiting = true

	case "Join Game":
		return NewJoinState(m.Ctx), nil

	case "Quit":
		return nil, tea.Quit
	}

	return nil, nil
}

func (m *MenuState) View() string {
	w, h := m.Ctx.Width, m.Ctx.Height

	if m.waiting {
		spinner := styleAlienDim.Render("[") +
			styleAlienPrimary.Render(" connecting... ") +
			styleAlienDim.Render("]")
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, spinner)
	}

	// Logo
	logo := styleAlienTitle.Render(gougeASCII)

	// Menu items
	rows := make([]string, len(m.items))
	for i, item := range m.items {
		if i == m.cursor {
			marker := styleAlienDanger.Render("▶")
			label := styleAlienSelected.Render(" " + item.title + " ")
			rows[i] = marker + "  " + label
		} else {
			marker := styleAlienFaint.Render("·")
			label := styleAlienDim.Render("  " + item.title)
			rows[i] = marker + label
		}
	}

	menu := strings.Join(rows, "\n")
	hint := styleAlienFaint.Render("[↑↓]  navigate    [Enter]  select    [Q]  quit")

	block := lipgloss.JoinVertical(lipgloss.Center,
		logo,
		"",
		"",
		menu,
		"",
		hint,
	)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, block)
}

//
// --- JOIN STATE ---
//

type JoinState struct {
	Ctx     *UIContext
	input   textinput.Model
	errMsg  string
	waiting bool
}

func NewJoinState(ctx *UIContext) *JoinState {
	ti := textinput.New()
	ti.Placeholder = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
	ti.CharLimit = 36
	ti.Width = 38
	ti.Focus()
	return &JoinState{Ctx: ctx, input: ti}
}

func (j *JoinState) Init() tea.Cmd { return textinput.Blink }

func (j *JoinState) Update(msg tea.Msg) (ScreenState, tea.Cmd) {
	switch msg := msg.(type) {
	case dto.GameView:
		if j.waiting {
			switch msg.State {
			case dto.StateLobby:
				return NewLobbyState(j.Ctx), nil
			case dto.StatePlaying:
				j.waiting = false
				j.errMsg = "Session already started — cannot join mid-game."
				return nil, nil
			case dto.StateUnknown:
				j.waiting = false
				j.errMsg = "Session not found."
				return nil, nil
			}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return NewMenuState(j.Ctx), nil
		case "enter":
			id := j.input.Value()
			if id == "" {
				j.errMsg = "Please enter a session ID."
				return nil, nil
			}
			j.Ctx.DataToM <- dto.Command{
				Action:         dto.ActionJoin,
				PlayerUUID:     j.Ctx.PlayerUUID,
				PlayerNickname: j.Ctx.Nickname,
				PlaythroughId:  id,
			}
			j.waiting = true
			j.errMsg = ""
			return nil, nil
		}
	}

	var cmd tea.Cmd
	j.input, cmd = j.input.Update(msg)
	return nil, cmd
}

func (j *JoinState) View() string {
	w, h := j.Ctx.Width, j.Ctx.Height

	title := styleAlienTitle.Render("JOIN GAME")
	sep := styleAlienFaint.Render(strings.Repeat("─", 42))
	prompt := styleAlienDim.Render("SESSION ID")
	field := j.input.View()

	errLine := ""
	if j.errMsg != "" {
		errLine = "\n" + styleAlienErr.Render("✗  "+j.errMsg)
	}
	status := ""
	if j.waiting {
		status = "\n" + styleAlienDim.Render("[ connecting... ]")
	}

	hint := styleAlienFaint.Render("[Enter]  join    [Esc]  back")

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		sep,
		"",
		prompt,
		field+errLine+status,
		"",
		hint,
	)

	panel := styleAlienBorder.Padding(1, 3).Render(body)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, panel)
}

// shortID returns the first 8 characters of a UUID for compact display.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8] + "…"
	}
	return id
}
