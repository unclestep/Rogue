package storage

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
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

func (m *MemorySessionRepo) Get(id model.PlaythroughId) (*model.Playthrough, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("playthrough %s not found", id)
	}
	return p, nil
}

func (m *MemorySessionRepo) Create(rulesId model.RulesId, seed int64) *model.Playthrough {
	id := model.PlaythroughId(uuid.New().String())
	p := model.NewPlaythrough(id, rulesId, seed)

	m.mu.Lock()
	m.sessions[id] = p
	m.mu.Unlock()

	return p
}

func (m *MemorySessionRepo) Save(p *model.Playthrough) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[p.PlaythroughId] = p
	return nil
}

func (m *MemorySessionRepo) Delete(id model.PlaythroughId) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}
