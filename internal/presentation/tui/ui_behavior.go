package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/unclestep/Rogue/internal/dto"
)

type UIContext struct {
	DataToM   chan<- dto.Command
	DataFromM <-chan dto.GameView

	World             dto.GameView
	PlayerUUID        string
	Nickname          string // Chosen once at startup; sent with every join command
	PlaythroughId     string
	LastPlaythroughId string // Persisted session ID loaded from client.json ("Continue" option).

	Keys   *KeyMap
	Width  int
	Height int
}

func NewUIContext(dataToM chan<- dto.Command, dataFromM <-chan dto.GameView, playerUUID, lastPlaythroughId string) *UIContext {
	return &UIContext{
		DataToM:           dataToM,
		DataFromM:         dataFromM,
		PlayerUUID:        playerUUID,
		LastPlaythroughId: lastPlaythroughId,
		Keys:              NewKeyMap(DefaultKeyConfig()),
	}
}

type KeyConfig struct {
	// Moving (WASD)
	Up    []string `json:"up"`
	Right []string `json:"right"`
	Down  []string `json:"down"`
	Left  []string `json:"left"`
	// Flashlight aim (arrow keys — set direction without moving)
	AimUp    []string `json:"aim_up"`
	AimDown  []string `json:"aim_down"`
	AimLeft  []string `json:"aim_left"`
	AimRight []string `json:"aim_right"`
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
	// Skip turn (submit no-op intent so the turn resolves without moving)
	SkipTurn []string `json:"skip_turn"`
}

func DefaultKeyConfig() KeyConfig {
	return KeyConfig{
		Up:         []string{"w"},
		Down:       []string{"s"},
		Right:      []string{"d"},
		Left:       []string{"a"},
		AimUp:      []string{"up"},
		AimDown:    []string{"down"},
		AimLeft:    []string{"left"},
		AimRight:   []string{"right"},
		Esc:        []string{"esc"},
		Return:     []string{"enter"},
		Inventory:  []string{"i"},
		WeaponSlot: []string{"h"},
		FoodSlot:   []string{"j"},
		ElixirSlot: []string{"k"},
		ScrollSlot: []string{"e"},
		SkipTurn:   []string{" "},
	}
}

type KeyMap struct {
	Up    key.Binding
	Right key.Binding
	Down  key.Binding
	Left  key.Binding

	AimUp    key.Binding
	AimDown  key.Binding
	AimLeft  key.Binding
	AimRight key.Binding

	Esc    key.Binding
	Return key.Binding

	Inventory key.Binding

	WeaponSlot key.Binding
	FoodSlot   key.Binding
	ElixirSlot key.Binding
	ScrollSlot key.Binding
	SkipTurn   key.Binding
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
		AimUp: key.NewBinding(
			key.WithKeys(cfg.AimUp...),
			key.WithHelp("↑", "Aim Up"),
		),
		AimDown: key.NewBinding(
			key.WithKeys(cfg.AimDown...),
			key.WithHelp("↓", "Aim Down"),
		),
		AimLeft: key.NewBinding(
			key.WithKeys(cfg.AimLeft...),
			key.WithHelp("←", "Aim Left"),
		),
		AimRight: key.NewBinding(
			key.WithKeys(cfg.AimRight...),
			key.WithHelp("→", "Aim Right"),
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
		SkipTurn: key.NewBinding(
			key.WithKeys(cfg.SkipTurn...),
			key.WithHelp("space", "Skip turn"),
		),
	}
}

type ScreenState interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (ScreenState, tea.Cmd)
	View() string
}
