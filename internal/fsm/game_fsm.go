package fsm

import (
	"log"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
)

type GameLogic struct {
	States  map[model.GameState]GameBehavior
	Session *model.GameSession
	Ctx     *GameBehaviorContext
}

func NewGameLogic(session *model.GameSession, seed int64) *GameLogic {
	gl := &GameLogic{
		States:  make(map[model.GameState]GameBehavior),
		Session: session,
		Ctx:     NewGameBehaviorContext(session, seed),
	}
	gl.RegisterAllStates()

	return gl
}

func (g *GameLogic) RegisterAllStates() {
	g.States[model.LobbyGameState] = &LobbyGameBehavior{}
	g.States[model.PlayingGameState] = &PlayingGameBehavior{}
	g.States[model.GameOverGameState] = &GameOverGameBehavior{}
}

func (g *GameLogic) ProcessPlayer(input *dto.Command) []service.Event {
	player := g.Session.GetPlayer(input.PlayerUUID)

	if player == nil {
		return nil
	}

	if !player.HasStaminaForHit() && !player.HasStaminaForMove() && !player.HasStaminaForAction() {
		return nil
	}

	behavior, exists := g.States[g.Session.State]
	if !exists {
		log.Fatalf("Game session's state %v does not exist\n", g.Session.State)
	}

	newState, events := behavior.Update(input, g.Ctx)
	if newState != g.Session.State {
		g.Session.State = newState
	}

	return events
}
