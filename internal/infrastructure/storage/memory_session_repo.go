package storage

import (
	"sync"

	"github.com/unclestep/Rogue/internal/domain/model"
)

type MemorySessionRepo struct {
	mu       sync.RWMutex
	sessions map[model.SessionId]*model.GameSession
}

func NewMemorySessionRepo() *MemorySessionRepo {
	return &MemorySessionRepo{
		sessions: make(map[model.SessionId]*model.GameSession),
	}
}

func (m *MemorySessionRepo) Save(session *model.GameSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.Id] = session
}

func (m *MemorySessionRepo) Get(sessionId model.SessionId) *model.GameSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, _ := m.sessions[sessionId]
	return session
}
