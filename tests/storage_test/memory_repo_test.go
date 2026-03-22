package storage_test

import (
	"sync"
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/infrastructure/storage"
)

// --- Get ---

func TestMemoryRepoGetNotFoundReturnsError(t *testing.T) {
	repo := storage.NewMemorySessionRepo()

	_, err := repo.Get("9999")
	if err == nil {
		t.Error("Expected error for missing playthrough, got nil")
	}
}

func TestMemoryRepoGetAfterCreateReturnsIt(t *testing.T) {
	repo := storage.NewMemorySessionRepo()
	p := repo.Create(model.DefaultRulesId, 42)

	got, err := repo.Get(p.PlaythroughId)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if got.PlaythroughId != p.PlaythroughId {
		t.Errorf("Expected id %s, got %s", p.PlaythroughId, got.PlaythroughId)
	}
}

func TestMemoryRepoGetInvalidIdReturnsError(t *testing.T) {
	repo := storage.NewMemorySessionRepo()

	_, err := repo.Get(model.InvalidPlaythroughId)
	if err == nil {
		t.Error("Expected error for InvalidPlaythroughId, got nil")
	}
}

// --- Create ---

func TestMemoryRepoCreateAssignsNonZeroId(t *testing.T) {
	repo := storage.NewMemorySessionRepo()
	p := repo.Create(model.DefaultRulesId, 123)

	if p.PlaythroughId == model.InvalidPlaythroughId {
		t.Error("Expected non-zero PlaythroughId after Create")
	}
}

func TestMemoryRepoCreateStoresInMemory(t *testing.T) {
	repo := storage.NewMemorySessionRepo()
	p := repo.Create(model.DefaultRulesId, 1)

	got, err := repo.Get(p.PlaythroughId)
	if err != nil {
		t.Fatalf("Playthrough not found after Create: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil after Create")
	}
}

func TestMemoryRepoCreateSetsCorrectRulesId(t *testing.T) {
	repo := storage.NewMemorySessionRepo()
	p := repo.Create(model.RulesId(5), 0)

	if p.RulesId != model.RulesId(5) {
		t.Errorf("Expected RulesId=5, got %d", p.RulesId)
	}
}

func TestMemoryRepoCreateSetsSeed(t *testing.T) {
	repo := storage.NewMemorySessionRepo()
	p := repo.Create(model.DefaultRulesId, 777)

	if p.Seed != 777 {
		t.Errorf("Expected Seed=777, got %d", p.Seed)
	}
}

func TestMemoryRepoCreateUniqueIds(t *testing.T) {
	repo := storage.NewMemorySessionRepo()
	ids := make(map[model.PlaythroughId]bool)

	for i := range 100 {
		p := repo.Create(model.DefaultRulesId, int64(i))
		if ids[p.PlaythroughId] {
			t.Errorf("Duplicate PlaythroughId generated: %s", p.PlaythroughId)
		}
		ids[p.PlaythroughId] = true
	}
}

// --- Save ---

func TestMemoryRepoSaveOverwritesExisting(t *testing.T) {
	repo := storage.NewMemorySessionRepo()
	p := repo.Create(model.DefaultRulesId, 1)

	p.Depth = 99
	if err := repo.Save(p); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got, _ := repo.Get(p.PlaythroughId)
	if got.Depth != 99 {
		t.Errorf("Expected Depth=99 after Save, got %d", got.Depth)
	}
}

func TestMemoryRepoSaveNonExistentIdCreatesEntry(t *testing.T) {
	repo := storage.NewMemorySessionRepo()
	p := model.NewPlaythrough("test-42", model.DefaultRulesId, 0)

	if err := repo.Save(p); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got, err := repo.Get("test-42")
	if err != nil {
		t.Fatalf("Expected to find saved playthrough, got error: %v", err)
	}
	if got.PlaythroughId != "test-42" {
		t.Errorf("Expected id 42, got %s", got.PlaythroughId)
	}
}

func TestMemoryRepoSaveReturnsNoError(t *testing.T) {
	repo := storage.NewMemorySessionRepo()
	p := repo.Create(model.DefaultRulesId, 0)

	err := repo.Save(p)
	if err != nil {
		t.Errorf("Expected nil error from Save, got: %v", err)
	}
}

// --- Delete ---

func TestMemoryRepoDeleteRemovesFromStore(t *testing.T) {
	repo := storage.NewMemorySessionRepo()
	p := repo.Create(model.DefaultRulesId, 1)

	repo.Delete(p.PlaythroughId)

	_, err := repo.Get(p.PlaythroughId)
	if err == nil {
		t.Error("expected error after Delete, got nil")
	}
}

func TestMemoryRepoDeleteNonExistentNoError(t *testing.T) {
	repo := storage.NewMemorySessionRepo()
	// не должно паниковать
	repo.Delete("9999")
}

// --- Concurrent safety ---

func TestMemoryRepoConcurrentSaveGet(t *testing.T) {
	repo := storage.NewMemorySessionRepo()
	p := repo.Create(model.DefaultRulesId, 1)

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = repo.Save(p)
		}()
		go func() {
			defer wg.Done()
			_, _ = repo.Get(p.PlaythroughId)
		}()
	}
	wg.Wait()
}

func TestMemoryRepoConcurrentCreateDelete(t *testing.T) {
	repo := storage.NewMemorySessionRepo()

	var wg sync.WaitGroup
	var mu sync.Mutex
	ids := make([]model.PlaythroughId, 0)

	for range 20 {
		wg.Go(func() {
			p := repo.Create(model.DefaultRulesId, time.Now().UnixNano())
			mu.Lock()
			ids = append(ids, p.PlaythroughId)
			mu.Unlock()
		})
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
