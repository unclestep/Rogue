package fsm

import (
	"github.com/unclestep/Rogue/internal/domain/entity"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/pkg/command"
	"github.com/unclestep/Rogue/pkg/geometry"
	"math/rand"
)

type GameBehaviorContext struct {
	Session      *entity.GameSession
	MonsterLogic *MonsterLogic
	Seed         int64
	Rng          *rand.Rand
}

func NewGameBehaviorContext(session *entity.GameSession, seed int64) *GameBehaviorContext {
	return &GameBehaviorContext{
		Session:      session,
		MonsterLogic: NewMonsterLogic(session, seed),
		Seed:         seed,
		Rng:          rand.New(rand.NewSource(seed)),
	}
}

type GameBehavior interface {
	Update(ctx *GameBehaviorContext) entity.GameState
}

type LobbyGameBehavior struct{}

func (lobby *LobbyGameBehavior) Update(input *command.Command, ctx *GameBehaviorContext) entity.GameState {
	if input.CommandType == command.CommandJoin {
		ctx.Session.AddPlayer(input.PlayerStats)
	}
	if input.CommandType == command.CommandAgree {
		ctx.Session.NextLevel(ctx.Rng)
		return entity.PlayingGameState
	}

	return entity.LobbyGameState
}

type PlayingGameBehavior struct{}

func (play *PlayingGameBehavior) Update(input *command.Command, ctx *GameBehaviorContext) entity.GameState {

}

func (play *PlayingGameBehavior) movePlayer(input *command.Command, ctx *GameBehaviorContext) *service.MoveEvent {
	player, exists := ctx.Session.Players[input.PlayerID]
	if !exists {
		return nil
	}

	event := service.NewMoveEvent(player)
	switch input.CommandType {
	case command.CommandUp:
		event.Mover.PosChange = geometry.GetDirUp()
	case command.CommandRight:
		event.Mover.PosChange = geometry.GetDirRight()
	case command.CommandDown:
		event.Mover.PosChange = geometry.GetDirDown()
	case command.CommandLeft:
		event.Mover.PosChange = geometry.GetDirLeft()
	default:
		event = nil
	}

	return event
}

func (play *PlayingGameBehavior) consumeItem(input *command.Command, ctx *GameBehaviorContext) {
	player, exists := ctx.Session.Players[input.PlayerID]
	if !exists {
		return nil
	}

	if input.CommandType == command.CommandConsumeItem {
		item := player.Backpack.RetrieveById(input.ItemID)
		item.Consume(player)
	}
}
