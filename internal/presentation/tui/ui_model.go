package tui

import (
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/unclestep/Rogue/internal/dto"
)

type UIModel struct {
	Ctx   *UIContext
	State ScreenState
}

func NewUIModel(dataToM chan<- dto.Command, dataFromM <-chan dto.GameView, playerUUID, lastPlaythroughId string) *UIModel {
	ctx := NewUIContext(dataToM, dataFromM, playerUUID, lastPlaythroughId)
	// Show the nickname prompt on first launch.  Once the player sets their
	// name it is stored in UIContext.Nickname and won't be asked again for the
	// lifetime of this process.
	var initialState ScreenState
	if ctx.Nickname == "" {
		initialState = NewNicknameState(ctx)
	} else {
		initialState = NewMenuState(ctx)
	}
	return &UIModel{
		Ctx:   ctx,
		State: initialState,
	}
}

// Run starts the BubbleTea program and blocks until the player quits.
// It returns the PlaythroughId that was active when the program exited so the
// caller can persist it for the "Continue" option on next launch.
func (ui UIModel) Run() string {
	// WithInput(os.Stdin) prevents BubbleTea from trying to open /dev/tty itself,
	// which fails in environments that have no controlling terminal (IDE runners, etc.).
	p := tea.NewProgram(ui, tea.WithAltScreen(), tea.WithInput(os.Stdin))
	finalModel, err := p.Run()
	if err != nil {
		log.Fatalf("UI start error: %v", err)
	}
	if uim, ok := finalModel.(UIModel); ok {
		return uim.Ctx.PlaythroughId
	}
	return ""
}

func (ui UIModel) Init() tea.Cmd {
	return tea.Batch(
		waitForModelMsg(ui.Ctx.DataFromM),
		ui.State.Init(),
	)
}

func waitForModelMsg(dataFromM <-chan dto.GameView) tea.Cmd {
	return func() tea.Msg {
		data, ok := <-dataFromM
		if !ok {
			return nil
		}
		return data
	}
}

func (ui UIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ui.Ctx.Width = msg.Width
		ui.Ctx.Height = msg.Height
	case dto.GameView:
		ui.Ctx.World = msg
		ui.Ctx.PlaythroughId = msg.PlaythroughId
		cmds = append(cmds, waitForModelMsg(ui.Ctx.DataFromM))
	}

	nextState, cmd := ui.State.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	if nextState != nil {
		ui.State = nextState
		cmds = append(cmds, ui.State.Init())
	}

	return ui, tea.Batch(cmds...)
}

func (ui UIModel) View() string {
	return ui.State.View()
}
