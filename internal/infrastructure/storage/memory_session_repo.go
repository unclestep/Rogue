package storage

import (
	"sync"

	"github.com/unclestep/Rogue/internal/domain/model"
)

type MemorySessionRepo struct {
	mu       sync.RWMutex
	sessions map[model.PlaythroughId]*model.Playthrough
}

func NewMemorySessionRepo() *MemorySessionRepo {
	return &MemorySessionRepo{
		sessions: make(map[model.PlaythroughId]*model.Playthrough),
	}
}

func (m *MemorySessionRepo) Save(playthrough *model.Playthrough) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[playthrough.PlaythroughId] = playthrough
}

func (m *MemorySessionRepo) Get(playthroughId model.PlaythroughId) *model.Playthrough {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, _ := m.sessions[playthroughId]
	return session
}
