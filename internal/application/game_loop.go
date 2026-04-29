package application

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/unclestep/Rogue/internal/application/usecase"
	"github.com/unclestep/Rogue/internal/application/view"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/dto"
)

type GameLoop struct {
	mu           sync.RWMutex
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
	g.mu.Lock()
	g.subscribers[playerUuid] = newChan
	g.mu.Unlock()
	return newChan
}

func (g *GameLoop) GetNotificationChan() chan<- dto.Command {
	return g.notification
}

func (g *GameLoop) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-g.notification:
			newState, playId := g.resolveState.Resolve(&cmd)
			if playId == model.InvalidPlaythroughId {
				// Session not found or could not be created — tell the sender.
				g.notifyPlayer(cmd.PlayerUUID, dto.GameView{State: dto.StateUnknown})
				break
			}
			// Only track sessions that are actively playing; remove them when they
			// transition to Lobby or GameOver so the ticker stops broadcasting stale state.
			if newState == model.PlayingGameState {
				g.activePlaythroughs[playId] = true
			} else {
				delete(g.activePlaythroughs, playId)
			}
			g.broadcastState(playId)
		case <-ticker.C:
			for playId := range g.activePlaythroughs {
				tickCmd := &dto.Command{
					PlaythroughId: string(playId),
					Action:        dto.ActionTick,
				}
				newState, resolvedId := g.resolveState.Resolve(tickCmd)
				if newState == model.GameOverGameState || resolvedId == model.InvalidPlaythroughId {
					delete(g.activePlaythroughs, playId)
					continue
				}
				g.broadcastState(playId)
			}
		}
	}
}

// notifyPlayer sends a single GameView directly to one subscriber without a playthrough lookup.
func (g *GameLoop) notifyPlayer(playerUUID string, view dto.GameView) {
	if playerUUID == "" {
		return
	}
	g.mu.RLock()
	ch, exists := g.subscribers[playerUUID]
	g.mu.RUnlock()
	if !exists {
		return
	}
	select {
	case ch <- view:
	default:
	}
}

func (g *GameLoop) broadcastState(playId model.PlaythroughId) {
	snapshots := g.viewMapper.MakeSnapshots(playId)
	if snapshots == nil {
		log.Printf("[WARN] broadcastState: no snapshots for playthrough %v (repo missing or mapper failed)", playId)
		return
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

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
