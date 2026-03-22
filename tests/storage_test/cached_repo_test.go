package storage_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/infrastructure/storage"
)

// newCached builds a CachedPlaythroughRepo backed by a MemorySessionRepo.
func newCached() *storage.CachedPlaythroughRepo {
	return storage.NewCachedPlaythroughRepo(storage.NewMemorySessionRepo())
}

// --- Get: cache hit ---

func TestCachedGetReturnsFromCacheWithoutHittingStorage(t *testing.T) {
	repo := newCached()
	p := repo.Create(model.DefaultRulesId, 1)

	// Second Get must come from cache.
	got, err := repo.Get(p.PlaythroughId)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if got.PlaythroughId != p.PlaythroughId {
		t.Errorf("Got id %s, want %s", got.PlaythroughId, p.PlaythroughId)
	}
}

// --- Get: storage hit ---

func TestCachedGetLoadsFromStorageWhenNotCached(t *testing.T) {
	mem := storage.NewMemorySessionRepo()
	p := mem.Create(model.DefaultRulesId, 7)

	// Build a cached repo on top of the same backing store.
	// The cache is empty, so Get must delegate to storage.
	repo := storage.NewCachedPlaythroughRepo(mem)

	got, err := repo.Get(p.PlaythroughId)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if got.PlaythroughId != p.PlaythroughId {
		t.Errorf("Got id %s, want %s", got.PlaythroughId, p.PlaythroughId)
	}
}

// --- Get: not found ---

// Get with a non-existent id must return an error — no silent fallback.
func TestCachedGetReturnsErrorForNonExistentId(t *testing.T) {
	repo := newCached()

	got, err := repo.Get(model.PlaythroughId("999999"))
	if err == nil {
		t.Error("Expected error for non-existent id, got nil")
	}
	if got != nil {
		t.Errorf("Expected nil playthrough for non-existent id, got %v", got.PlaythroughId)
	}
}

// Get(InvalidPlaythroughId) must also return an error — the backing store does not
// have an entry for the empty string.
func TestCachedGetReturnsErrorForInvalidPlaythroughId(t *testing.T) {
	repo := newCached()

	got, err := repo.Get(model.InvalidPlaythroughId)
	if err == nil {
		t.Error("Expected error for InvalidPlaythroughId, got nil")
	}
	if got != nil {
		t.Errorf("Expected nil playthrough, got %v", got.PlaythroughId)
	}
}

// Repeated Get calls for a missing id must all return errors — no side effects.
func TestCachedGetRepeatedMissingIdReturnsErrors(t *testing.T) {
	repo := newCached()
	missingId := model.PlaythroughId("1111")

	_, err1 := repo.Get(missingId)
	_, err2 := repo.Get(missingId)

	if err1 == nil || err2 == nil {
		t.Error("Expected errors for both Get calls on missing id")
	}
}

// --- Create ---

func TestCached_Create_PopulatesCache(t *testing.T) {
	repo := newCached()
	p := repo.Create(model.DefaultRulesId, 42)

	got, err := repo.Get(p.PlaythroughId)
	if err != nil {
		t.Fatalf("playthrough not in cache after Create: %v", err)
	}
	if got.PlaythroughId != p.PlaythroughId {
		t.Errorf("id mismatch: got %s, want %s", got.PlaythroughId, p.PlaythroughId)
	}
}

func TestCached_Create_PropagatesRulesId(t *testing.T) {
	repo := newCached()
	p := repo.Create(model.RulesId(3), 0)

	if p.RulesId != model.RulesId(3) {
		t.Errorf("expected RulesId=3, got %d", p.RulesId)
	}
}

func TestCached_Create_PropagatesSeed(t *testing.T) {
	repo := newCached()
	p := repo.Create(model.DefaultRulesId, 12345)

	if p.Seed != 12345 {
		t.Errorf("expected Seed=12345, got %d", p.Seed)
	}
}

// --- Save ---

func TestCached_Save_UpdatesCache(t *testing.T) {
	repo := newCached()
	p := repo.Create(model.DefaultRulesId, 1)
	p.Depth = 7

	if err := repo.Save(p); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got, _ := repo.Get(p.PlaythroughId)
	if got.Depth != 7 {
		t.Errorf("cache not updated: expected Depth=7, got %d", got.Depth)
	}
}

func TestCached_Save_PropagatesErrorFromStorage(t *testing.T) {
	failing := &failingStorage{}
	repo := storage.NewCachedPlaythroughRepo(failing)

	p := model.NewPlaythrough("test-1", model.DefaultRulesId, 0)
	err := repo.Save(p)
	if err == nil {
		t.Error("expected error from failing storage, got nil")
	}
}

// BUG PROBE: Save stores the pointer directly. If the caller mutates the struct
// after Save, the cached pointer reflects the mutation without another Save call.
func TestCached_Save_StoredByPointer_MutationReflectedInCache(t *testing.T) {
	repo := newCached()
	p := repo.Create(model.DefaultRulesId, 0)
	_ = repo.Save(p)

	// Mutate without calling Save again.
	p.Depth = 42

	got, _ := repo.Get(p.PlaythroughId)
	if got.Depth == 42 {
		t.Log("DOCUMENTED: cache stores a pointer — external mutation is visible without Save()")
	}
}

// --- Delete ---

func TestCached_Delete_RemovesFromCache(t *testing.T) {
	repo := newCached()
	p := repo.Create(model.DefaultRulesId, 1)

	repo.Delete(p.PlaythroughId)

	// After deletion, Get must return an error — the session is gone.
	got, err := repo.Get(p.PlaythroughId)
	if err == nil {
		t.Error("Expected error after Delete, but Get succeeded")
	}
	if got != nil {
		t.Errorf("Expected nil after Delete, got playthrough %v", got.PlaythroughId)
	}
}

func TestCached_Delete_NonExistent_NoPanic(t *testing.T) {
	repo := newCached()
	repo.Delete("test-9999") // must not panic
}

// --- Concurrent safety ---

func TestCached_ConcurrentGetSave(t *testing.T) {
	repo := newCached()
	p := repo.Create(model.DefaultRulesId, 1)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = repo.Get(p.PlaythroughId)
		}()
		go func() {
			defer wg.Done()
			_ = repo.Save(p)
		}()
	}
	wg.Wait()
}

func TestCached_ConcurrentCreateDelete(t *testing.T) {
	repo := newCached()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var ids []model.PlaythroughId

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := repo.Create(model.DefaultRulesId, time.Now().UnixNano())
			mu.Lock()
			ids = append(ids, p.PlaythroughId)
			mu.Unlock()
		}()
	}
	wg.Wait()

	for _, id := range ids {
		wg.Add(1)
		go func(id model.PlaythroughId) {
			defer wg.Done()
			repo.Delete(id)
		}(id)
	}
	wg.Wait()
}

// --- failingStorage ---

// failingStorage is a port.PlaythroughRepository that returns errors for Get and Save.
type failingStorage struct{}

func (f *failingStorage) Get(_ model.PlaythroughId) (*model.Playthrough, error) {
	return nil, errStorage
}

func (f *failingStorage) Create(rulesId model.RulesId, seed int64) *model.Playthrough {
	return model.NewPlaythrough(model.PlaythroughId(fmt.Sprintf("%d", time.Now().UnixNano())), rulesId, seed)
}

func (f *failingStorage) Save(_ *model.Playthrough) error { return errStorage }

func (f *failingStorage) Delete(_ model.PlaythroughId) {}

// errStorage is a sentinel error used by failingStorage.
var errStorage = &storageError{"storage unavailable"}

type storageError struct{ msg string }

func (e *storageError) Error() string { return e.msg }
