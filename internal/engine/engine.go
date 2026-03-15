package engine

import (
	"log"
	"math/rand"
	"slices"
	"time"

	"github.com/unclestep/Rogue/internal/adapter/tui_adapter"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
	"github.com/unclestep/Rogue/internal/fsm"
)

type GameEngine struct {
	// Communication between components
	// TODO: map channels for multiplayer
	DataFromVM <-chan dto.Command
	DataToVM   chan<- dto.WorldInfo
	// dto session is created here
	Session      *model.Playthrough                     `json:"-"`
	GameLogic    *fsm.GameLogic                         `json:"game_logic"`
	MonsterLogic *fsm.MonsterLogic                      `json:"monster_logic"`
	VisibleAreas map[model.ActorId]*service.VisibleArea `jsov:"visible_areas"`
	TuiMapper    *tui_adapter.TuiMapper
	Seed         int64
	Rng          *rand.Rand
}

func NewGameEngine(dataFromVM <-chan dto.Command, dataToVM chan<- dto.WorldInfo, session *model.Playthrough, onAutosave func(), onGameover func(), seed int64) *GameEngine {
	return &GameEngine{
		DataFromVM:   dataFromVM,
		DataToVM:     dataToVM,
		Session:      session,
		GameLogic:    fsm.NewGameLogic(session, seed),
		MonsterLogic: fsm.NewMonsterLogic(session, seed),
		VisibleAreas: make(map[model.ActorId]*service.VisibleArea),
		TuiMapper:    tui_adapter.NewTuiMapper(session),
		Seed:         seed,
		Rng:          rand.New(rand.NewSource(seed)),
	}
}

func (g *GameEngine) Run() {
	g.broadcast(nil)
	for {
		commands := g.collectCommands()
		events := g.processTick(commands)
		g.broadcast(events)
	}
}

func (g *GameEngine) broadcast(events []service.Event) {
	for _, player := range g.Session.Players {
		snapshot := g.TuiMapper.MakeWorldSnapshot(player, g.VisibleAreas[player.Id], events)
		g.DataToVM <- snapshot
	}
}

func (g *GameEngine) collectCommands() []dto.Command {
	if g.Session.State != model.PlayingGameState {
		return []dto.Command{<-g.DataFromVM}
	}

	movedPlayers := make(map[model.ActorId]bool, len(g.Session.Players))
	commands := make([]dto.Command, 0, 32)
	timeout := time.Duration(g.Session.GameRules.TimeForMove) * time.Second
	var timeoutChan <-chan time.Time

loop:
	for len(movedPlayers) < len(g.Session.GetAlivePlayers()) {
		select {
		case cmd := <-g.DataFromVM:
			player := g.Session.GetPlayer(cmd.PlayerUUID)
			if player == nil {
				log.Fatalf("Player ID %v does not exist\n", cmd.PlayerUUID)
			}
			if timeoutChan == nil && timeout > 0 {
				timer := time.NewTimer(timeout)
				defer timer.Stop()
				timeoutChan = timer.C
			}
			if _, madeMove := movedPlayers[player.Id]; !madeMove {
				commands = append(commands, cmd)
				movedPlayers[player.Id] = true
			}
		case <-timeoutChan:
			break loop
		default:
		}
	}

	return commands
}

func (g *GameEngine) processTick(commands []dto.Command) []service.Event {
	slices.SortFunc(commands, func(a, b dto.Command) int {
		playerA, playerB := g.Session.GetPlayer(a.PlayerUUID), g.Session.GetPlayer(b.PlayerUUID)

		if playerA.DerivedAttrs[model.AttrDexterity] > playerB.DerivedAttrs[model.AttrDexterity] {
			return 1
		} else if playerA.DerivedAttrs[model.AttrDexterity] < playerB.DerivedAttrs[model.AttrDexterity] {
			return -1
		}
		return 0
	})

	// Tick effects (apply OnTurn effects and recompute actors' stats)
	g.Session.TickEffects(g.Rng)

	// Create players events
	events := make([]service.Event, 0, len(g.Session.Players)*2+len(g.Session.Monsters)*2)

	for _, cmd := range commands {
		player := g.Session.GetPlayer(cmd.PlayerUUID)

		if player == nil || g.Session.State == model.PlayingGameState && (player.Vitals[model.VitalHP] <= 0 || (!player.HasStaminaForAction() && !player.HasStaminaForHit() && !player.HasStaminaForMove())) {
			continue
		}

		events = append(events, g.GameLogic.ProcessPlayer(&cmd)...)
		g.VisibleAreas[player.Id].Update(player.Pos, 3)
	}

	if g.Session.State == model.PlayingGameState {
		// Give a turn to monsters
		g.MonsterLogic.UpdateScent(g.Rng)
		events = append(events, g.MonsterLogic.ProcessAllMonstersTurn()...)
	}

	return events
}
