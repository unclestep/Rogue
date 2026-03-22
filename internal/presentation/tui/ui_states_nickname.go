package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NicknameState is shown once at startup when the player has no stored nickname.
// After confirmation the name is written into UIContext.Nickname and the UI
// transitions to the main menu without prompting again for the rest of the
// session.  The server also receives the name on the next ActionJoin command
// and persists it inside the Playthrough for the duration of the game session.
type NicknameState struct {
	Ctx    *UIContext
	input  textinput.Model
	errMsg string
}

func NewNicknameState(ctx *UIContext) *NicknameState {
	ti := textinput.New()
	ti.Placeholder = "Enter your name…"
	ti.CharLimit = 20
	ti.Width = 24
	ti.Focus()
	return &NicknameState{Ctx: ctx, input: ti}
}

func (n *NicknameState) Init() tea.Cmd { return textinput.Blink }

func (n *NicknameState) Update(msg tea.Msg) (ScreenState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			name := strings.TrimSpace(n.input.Value())
			if name == "" {
				n.errMsg = "Name cannot be empty."
				return nil, nil
			}
			n.Ctx.Nickname = name
			return NewMenuState(n.Ctx), nil

		case "esc":
			// Allow skipping with a generic name so the player isn't stuck.
			n.Ctx.Nickname = "Adventurer"
			return NewMenuState(n.Ctx), nil
		}
	}

	var cmd tea.Cmd
	n.input, cmd = n.input.Update(msg)
	return nil, cmd
}

func (n *NicknameState) View() string {
	w, h := n.Ctx.Width, n.Ctx.Height

	title := styleAlienTitle.Render("ENTER YOUR NAME")
	sep := styleAlienFaint.Render(strings.Repeat("─", 28))
	field := n.input.View()

	errLine := ""
	if n.errMsg != "" {
		errLine = "\n" + styleAlienErr.Render("✗  "+n.errMsg)
	}

	hint := styleAlienFaint.Render("[Enter]  confirm    [Esc]  skip")

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		sep,
		"",
		field+errLine,
		"",
		hint,
	)

	panel := styleAlienBorder.Padding(1, 3).Render(body)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, panel)
}
