package application

import (
	"github.com/unclestep/Rogue/internal/application/usecase"
	"github.com/unclestep/Rogue/internal/application/view"
	"github.com/unclestep/Rogue/internal/dto"
	"time"
)

type GameLoop struct {
	subscribers  map[string]chan dto.GameView
	notification chan dto.Command

	activePlaythroughs map[int64]bool

	resolveState *usecase.ResolveState
	viewMapper   *view.ViewMapper
}

func NewGameLoop(resolveState *usecase.ResolveState, viewMapper *view.ViewMapper) *GameLoop {
	return &GameLoop{
		subscribers:        make(map[string]chan dto.GameView),
		notification:       make(chan dto.Command, 10000),
		activePlaythroughs: make(map[int64]bool),
		resolveState:       resolveState,
		viewMapper:         viewMapper,
	}
}

func (g *GameLoop) AddSubscriber(playerUuid string) <-chan dto.GameView {
	newChan := make(chan dto.GameView, 10)
	g.subscribers[playerUuid] = newChan
	return newChan
}

func (g *GameLoop) GetNotificationChan() chan<- dto.Command {
	return g.notification
}

func (g *GameLoop) Run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case cmd := <-g.notification:
			g.activePlaythroughs[cmd.PlaythroughId] = true
			g.resolveState.Resolve(&cmd)
			g.broadcastState(cmd.PlaythroughId)
		case <-ticker.C:
			for playId := range g.activePlaythroughs {
				tickCmd := &dto.Command{
					PlaythroughId: playId,
					Action:        dto.ActionTick,
				}
				g.resolveState.Resolve(tickCmd)
			}
			g.broadcastState(playId)
		}
	}
}

func (g *GameLoop) broadcastState(playId model.PlaythroughId) {
	playthroughView, playerIds := g.viewMapper.MakeSnapshots(playId)

	for _, pid := range playerIds {
		ch, exists := g.subscribers[pid]
		if !exists {
			continue
		}

		select {
		case ch <- playthroughView:
		default:
		}
	}
}
