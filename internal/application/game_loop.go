package application

import (
	"time"

	"github.com/unclestep/Rogue/internal/application/usecase"
	"github.com/unclestep/Rogue/internal/application/view"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/dto"
)

type GameLoop struct {
	subscribers  map[string]chan dto.GameView
	notification chan dto.Command

	activePlaythroughs map[model.PlaythroughId]bool

	resolveState *usecase.ResolveState
	viewMapper   *view.Mapper
}

func NewGameLoop(resolveState *usecase.ResolveState, viewMapper *view.Mapper) *GameLoop {
	return &GameLoop{
		subscribers:        make(map[string]chan dto.GameView),
		notification:       make(chan dto.Command, 10000),
		activePlaythroughs: make(map[model.PlaythroughId]bool),
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
			g.activePlaythroughs[model.PlaythroughId(cmd.PlaythroughId)] = true
			g.resolveState.Resolve(&cmd)
			g.broadcastState(model.PlaythroughId(cmd.PlaythroughId))
		case <-ticker.C:
			for playId := range g.activePlaythroughs {
				tickCmd := &dto.Command{
					PlaythroughId: int64(playId),
					Action:        dto.ActionTick,
				}
				g.resolveState.Resolve(tickCmd)
				g.broadcastState(playId)
			}
		}
	}
}

func (g *GameLoop) broadcastState(playId model.PlaythroughId) {
	snapshots := g.viewMapper.MakeSnapshots(playId)

	for playerUuid, snapshot := range snapshots {
		ch, exists := g.subscribers[playerUuid]
		if !exists {
			continue
		}

		select {
		case ch <- *snapshot:
		default:
		}
	}
}
