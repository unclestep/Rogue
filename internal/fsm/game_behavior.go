package fsm

import (
	"log"
	"math/rand"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
	"github.com/unclestep/Rogue/pkg/geometry"
)

type GameBehaviorContext struct {
	Session      *model.GameSession
	Resolver     *service.Resolver
	ItemUsage    *service.ItemUsage
	MonsterLogic *MonsterLogic
	Seed         int64
	Rng          *rand.Rand
}

func NewGameBehaviorContext(session *model.GameSession, seed int64) *GameBehaviorContext {
	return &GameBehaviorContext{
		Session:      session,
		Resolver:     service.NewResolverService(session, seed),
		ItemUsage:    service.NewItemUsage(session),
		MonsterLogic: NewMonsterLogic(session, seed),
		Seed:         seed,
		Rng:          rand.New(rand.NewSource(seed)),
	}
}

type GameBehavior interface {
	Update(input *dto.Command, ctx *GameBehaviorContext) (model.GameState, []service.Event)
}

//
//
// --- LOBBY STATE ---
//
//

type LobbyGameBehavior struct{}

func (lobby *LobbyGameBehavior) Update(input *dto.Command, ctx *GameBehaviorContext) (model.GameState, []service.Event) {
	if input.CommandType == dto.CommandJoin {
		ctx.Session.AddPlayer(input.PlayerUUID)
	}
	if input.CommandType == dto.CommandAgree {
		ctx.Session.NextLevel(ctx.Rng)
		return model.PlayingGameState, nil
	}

	return model.LobbyGameState, nil
}

//
//
// --- PLAYING STATE ---
//
//

type PlayingGameBehavior struct{}

func (play *PlayingGameBehavior) Update(input *dto.Command, ctx *GameBehaviorContext) (model.GameState, []service.Event) {
	player := ctx.Session.GetPlayer(input.PlayerUUID)
	if player == nil {
		log.Printf("Player ID#%v does not exists\n", input.PlayerUUID)
		return model.PlayingGameState, nil
	}

	events := make([]service.Event, 0, 2)

	if input.IsMove() {
		events = play.MovePlayer(player, input, ctx)
	} else if input.CommandType == dto.CommandConsumeItem {
		events = append(events, ctx.ItemUsage.ConsumeItem(model.ItemId(input.ItemID), player))
	} else if input.CommandType == dto.CommandEquipWeapon {
		events = append(events, ctx.ItemUsage.EquipItem(model.ItemId(input.ItemID), player))
	} else if input.CommandType == dto.CommandUnequipWeapon {
		events = append(events, ctx.ItemUsage.UnequipItem(model.ItemId(input.ItemID), player))
	}

	for _, event := range events {
		event.Perform(ctx.Session, ctx.Rng)
	}

	if ctx.Session.AreAllAlivePlayersEscaped() {
		ctx.Session.NextLevel(ctx.Rng)
		ctx.MonsterLogic.Rebuild()
		return model.PlayingGameState, nil
	}

	if ctx.Session.AreAllPlayersDead() {
		ctx.MonsterLogic.Reset()
		return model.GameOverGameState, nil
	}

	return model.PlayingGameState, events
}

func (play *PlayingGameBehavior) MovePlayer(player *model.Actor, input *dto.Command, ctx *GameBehaviorContext) []service.Event {
	event := service.NewMoveEvent(player)
	switch input.CommandType {
	case dto.CommandUp:
		event.Mover.PosChange = geometry.GetDirUp()
	case dto.CommandRight:
		event.Mover.PosChange = geometry.GetDirRight()
	case dto.CommandDown:
		event.Mover.PosChange = geometry.GetDirDown()
	case dto.CommandLeft:
		event.Mover.PosChange = geometry.GetDirLeft()
	default:
		event = nil
	}

	events := ctx.Resolver.ResolveMove(event)

	return events
}

//
//
// --- GAME OVER STATE ---
//
//

type GameOverGameBehavior struct{}

func (gameOver *GameOverGameBehavior) Update(input *dto.Command, ctx *GameBehaviorContext) (model.GameState, []service.Event) {
	if input.CommandType == dto.CommandAgree {
		return model.LobbyGameState, nil
	}

	return model.GameOverGameState, nil
}
