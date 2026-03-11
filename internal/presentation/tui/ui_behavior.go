package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/unclestep/Rogue/internal/dto"
)

type UIContext struct {
	DataToM   chan<- dto.Command
	DataFromM <-chan dto.WorldInfo

	World    dto.WorldInfo
	PlayerID int

	Keys   *KeyMap
	Width  int
	Height int
}

func NewUIContext(dataToM chan<- dto.Command, dataFromM <-chan dto.WorldInfo) *UIContext {
	return &UIContext{
		DataToM:   dataToM,
		DataFromM: dataFromM,
	}
}

type ScreenState interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (ScreenState, tea.Cmd)
	View() string
}

type KeyConfig struct {
	// Moving
	Up    []string `json:"up"`
	Right []string `json:"right"`
	Down  []string `json:"down"`
	Left  []string `json:"left"`
	// Yes/no buttons
	Esc    []string `json:"esc"`
	Return []string `json:"return"`
	// Inventory
	Inventory []string `json:"inventory"` // Opens whole inventory
	// Swift access to inventory slots
	WeaponSlot []string `json:"weapons_slot"`
	FoodSlot   []string `json:"foods_slot"`
	ElixirSlot []string `json:"elixirs_slot"`
	ScrollSlot []string `json:"scrolls_slot"`
}

type KeyMap struct {
	Up    key.Binding
	Right key.Binding
	Down  key.Binding
	Left  key.Binding

	Esc    key.Binding
	Return key.Binding

	Inventory key.Binding

	WeaponSlot key.Binding
	FoodSlot   key.Binding
	ElixirSlot key.Binding
	ScrollSlot key.Binding
}

func NewKeyMap(cfg KeyConfig) *KeyMap {
	return &KeyMap{
		Up: key.NewBinding(
			key.WithKeys(cfg.Up...),
			key.WithHelp("w", "Move Up"),
		),
		Right: key.NewBinding(
			key.WithKeys(cfg.Right...),
			key.WithHelp("d", "Move Right"),
		),
		Down: key.NewBinding(
			key.WithKeys(cfg.Down...),
			key.WithHelp("s", "Move Down"),
		),
		Left: key.NewBinding(
			key.WithKeys(cfg.Left...),
			key.WithHelp("a", "Move Left"),
		),
		Esc: key.NewBinding(
			key.WithKeys(cfg.Esc...),
			key.WithHelp("Esc", "Back"),
		),
		Return: key.NewBinding(
			key.WithKeys(cfg.Return...),
			key.WithHelp("Return", "Select"),
		),
		Inventory: key.NewBinding(
			key.WithKeys(cfg.Inventory...),
			key.WithHelp("I", "Open inventory"),
		),
		WeaponSlot: key.NewBinding(
			key.WithKeys(cfg.WeaponSlot...),
			key.WithHelp("h", "Open weapon slot"),
		),
		FoodSlot: key.NewBinding(
			key.WithKeys(cfg.FoodSlot...),
			key.WithHelp("j", "Open food slot"),
		),
		ElixirSlot: key.NewBinding(
			key.WithKeys(cfg.ElixirSlot...),
			key.WithHelp("k", "Open elixir slot"),
		),
		ScrollSlot: key.NewBinding(
			key.WithKeys(cfg.ScrollSlot...),
			key.WithHelp("e", "Open scroll slot"),
		),
	}
}

//
//
// ---
//
//

//
//
// --- PLAYING STATE ---
//
//

type PlayingState struct {
	Ctx *UIContext
}

func NewPlayingState() *PlayingState {
	return &PlayingState{}
}

func (p *PlayingState) Init() tea.Cmd {
	return nil
}

func (p *PlayingState) Update(msg tea.Msg) (ScreenState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		p.handleInput(msg.String())
	case dto.WorldInfo:
		if msg.State == dto.StateGameover {
			return NewGameoverState(), nil
		}
	}

	return nil, nil
}

func (p *PlayingState) handleInput(inp string) (ScreenState, tea.Cmd) {
	switch inp {
	case key.Matches(inp, p.Ctx.Keys.Up):
		p.sendCommand(dto.CommandUp)
	case key.Matches(inp, p.Ctx.Keys.Right):
		p.sendCommand(dto.CommandRight)
	case key.Matches(inp, p.Ctx.Keys.Down):
		p.sendCommand(dto.CommandDown)
	case key.Matches(inp, p.Ctx.Keys.Left):
		p.sendCommand(dto.CommandLeft)
	case key.Matches(inp, p.Ctx.Keys.Inventory):
		return NewInventoryState(p.Ctx), nil
	}

	return nil, nil
}

func (p *PlayingState) sendCommand(cmd dto.CommandType) {
	p.Ctx.DataToM <- dto.Command{
		CommandType: cmd,
		PlayerID:    p.Ctx.PlayerID,
	}
}

func (p *PlayingState) View() string {
	return p.renderMap(p.Ctx.World)
}

func (p *PlayingState) renderMap(world dto.WorldInfo) string {
	return ""
}

type InventoryState struct {
	Ctx       *UIContext
	listModel list.Model
}

//
//
// --- INVENTORY STATE ---
//
//

func NewInventoryState(ctx *UIContext) InventoryState {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 20, 14)
	l.Title = "Inventory"
	return &InventoryState{
		Ctx:       ctx,
		listModel: l,
	}
}

func (i *InventoryState) Init() tea.Cmd {
	items := convertDTOToBubble
}
