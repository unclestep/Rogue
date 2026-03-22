package storage

import (
	"sync"

	"github.com/unclestep/Rogue/internal/application/port"
	"github.com/unclestep/Rogue/internal/domain/model"
)

// CachedPlaythroughRepo holds active sessions in memory and delegates
// persistence to any port.PlaythroughRepository implementation (JSON, proto, etc.).
type CachedPlaythroughRepo struct {
	mu      sync.RWMutex
	cache   map[model.PlaythroughId]*model.Playthrough
	storage port.PlaythroughRepository
}

// NewCachedPlaythroughRepo accepts any port.PlaythroughRepository as the backing store.
func NewCachedPlaythroughRepo(storage port.PlaythroughRepository) *CachedPlaythroughRepo {
	return &CachedPlaythroughRepo{
		cache:   make(map[model.PlaythroughId]*model.Playthrough),
		storage: storage,
	}
}

// Get looks up the playthrough in the cache then in persistent storage.
// Returns an error when the session does not exist — callers must not assume a
// session will be created automatically.
func (r *CachedPlaythroughRepo) Get(id model.PlaythroughId) (*model.Playthrough, error) {
	r.mu.RLock()
	if p, ok := r.cache[id]; ok {
		r.mu.RUnlock()
		return p, nil
	}
	r.mu.RUnlock()

	p, err := r.storage.Get(id)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.cache[id] = p
	r.mu.Unlock()

	return p, nil
}

func (r *CachedPlaythroughRepo) Create(rulesId model.RulesId, seed int64) *model.Playthrough {
	p := r.storage.Create(rulesId, seed)

	r.mu.Lock()
	r.cache[p.PlaythroughId] = p
	r.mu.Unlock()

	return p
}

func (r *CachedPlaythroughRepo) Save(p *model.Playthrough) error {
	r.mu.Lock()
	r.cache[p.PlaythroughId] = p
	r.mu.Unlock()

	return r.storage.Save(p)
}

func (r *CachedPlaythroughRepo) Delete(id model.PlaythroughId) {
	r.mu.Lock()
	delete(r.cache, id)
	r.mu.Unlock()

	r.storage.Delete(id)
}
