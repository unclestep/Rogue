package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/unclestep/Rogue/internal/dto"
	"log"
)

type UIModel struct {
	Ctx   *UIContext
	State ScreenState
}

func NewUIModel(dataToM chan<- dto.Command, dataFromM <-chan dto.WorldInfo) *UIModel {
	ctx := NewUIContext(dataToM, dataFromM)
	return &UIModel{
		Ctx:   ctx,
		State: NewStartState(ctx),
	}
}

func (ui UIModel) Run() {
	p := tea.NewProgram(ui)
	if _, err := p.Run(); err != nil {
		return log.Fatalf("UI start error: %v", err)
	}
}

func (ui UIModel) Init() tea.Cmd {
	return tea.Batch(
		waitForModelMsg(ui.Ctx.DataFromM),
		ui.State.Init(),
	)
}

func waitForModelMsg(dataFromM <-chan dto.WorldInfo) tea.Cmd {
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
	case dto.WorldInfo:
		ui.Ctx.World = msg
		ui.Ctx.PlayerID = msg.Player.Actor.Id
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
