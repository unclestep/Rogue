package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	jsonStorage "github.com/unclestep/Rogue/internal/infrastructure/storage/json"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "gouge_json_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// --- JsonPlaythroughRepo ---

func TestJsonRepoCreateWritesFileToDisk(t *testing.T) {
	repo := jsonStorage.NewJsonPlaythroughRepo(tempDir(t))
	p := repo.Create(model.DefaultRulesId, 1)

	got, err := repo.Get(p.PlaythroughId)
	if err != nil {
		t.Fatalf("File not persisted: %v", err)
	}
	if got.PlaythroughId != p.PlaythroughId {
		t.Errorf("id mismatch after Create+Get: got %s, want %s", got.PlaythroughId, p.PlaythroughId)
	}
}

func TestJsonRepoCreateAssignsNonZeroId(t *testing.T) {
	repo := jsonStorage.NewJsonPlaythroughRepo(tempDir(t))
	p := repo.Create(model.DefaultRulesId, 99)

	if p.PlaythroughId == model.InvalidPlaythroughId {
		t.Error("Expected non-zero PlaythroughId after Create")
	}
}

func TestJsonRepo_Create_SetsRulesIdAndSeed(t *testing.T) {
	repo := jsonStorage.NewJsonPlaythroughRepo(tempDir(t))
	p := repo.Create(model.RulesId(2), 555)

	if p.RulesId != model.RulesId(2) {
		t.Errorf("expected RulesId=2, got %d", p.RulesId)
	}
	if p.Seed != 555 {
		t.Errorf("expected Seed=555, got %d", p.Seed)
	}
}

func TestJsonRepo_Get_ReturnsErrorForMissingFile(t *testing.T) {
	repo := jsonStorage.NewJsonPlaythroughRepo(tempDir(t))
	_, err := repo.Get("123456789")
	if err == nil {
		t.Error("expected error for missing JSON file, got nil")
	}
}

func TestJsonRepo_Save_PersistsAndReloads(t *testing.T) {
	dir := tempDir(t)
	repo := jsonStorage.NewJsonPlaythroughRepo(dir)
	p := repo.Create(model.DefaultRulesId, 1)

	p.Depth = 5
	if err := repo.Save(p); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := repo.Get(p.PlaythroughId)
	if err != nil {
		t.Fatalf("Get after Save failed: %v", err)
	}
	if loaded.Depth != 5 {
		t.Errorf("expected Depth=5 after reload, got %d", loaded.Depth)
	}
}

func TestJsonRepo_Save_RoundTrip_PreservesState(t *testing.T) {
	repo := jsonStorage.NewJsonPlaythroughRepo(tempDir(t))
	p := repo.Create(model.DefaultRulesId, 42)
	originalId := p.PlaythroughId

	p.Depth = 3
	_ = repo.Save(p)

	got, _ := repo.Get(originalId)
	if got.PlaythroughId != originalId {
		t.Errorf("PlaythroughId changed after round-trip: got %s, want %s", got.PlaythroughId, originalId)
	}
	if got.Seed != 42 {
		t.Errorf("Seed changed after round-trip: got %d, want 42", got.Seed)
	}
	if got.Depth != 3 {
		t.Errorf("Depth not persisted: got %d, want 3", got.Depth)
	}
}

func TestJsonRepo_Delete_RemovesFile(t *testing.T) {
	repo := jsonStorage.NewJsonPlaythroughRepo(tempDir(t))
	p := repo.Create(model.DefaultRulesId, 1)

	repo.Delete(p.PlaythroughId)

	_, err := repo.Get(p.PlaythroughId)
	if err == nil {
		t.Error("expected error after Delete, file still readable")
	}
}

func TestJsonRepo_Delete_NonExistent_NoPanic(t *testing.T) {
	repo := jsonStorage.NewJsonPlaythroughRepo(tempDir(t))
	repo.Delete("test-999") // must not panic
}

// BUG PROBE: Create silently ignores the error from Save (uses _ = r.Save(p)).
// If the folder is read-only, Create returns a valid *Playthrough but nothing is on disk.
func TestJsonRepo_Create_InReadOnlyDir_FileMissing(t *testing.T) {
	dir := tempDir(t)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Skip("cannot change dir permissions on this OS")
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	repo := jsonStorage.NewJsonPlaythroughRepo(dir)
	p := repo.Create(model.DefaultRulesId, 1)

	_, err := repo.Get(p.PlaythroughId)
	if err == nil {
		t.Error("BUG: Create silently swallows Save error — file does not exist but no error was returned from Create")
	}
}

// --- JsonRulesRepo ---

func TestJsonRulesRepo_Get_DefaultRules_CreatesFile(t *testing.T) {
	dir := tempDir(t)
	repo := jsonStorage.NewJsonRulesRepo(dir)

	rules, err := repo.Get(model.DefaultRulesId)
	if err != nil {
		t.Fatalf("Expected default rules, got error: %v", err)
	}
	if rules == nil {
		t.Fatal("Expected non-nil rules")
	}
}

func TestJsonRulesRepo_Get_UnknownId_FallsBackToDefault(t *testing.T) {
	repo := jsonStorage.NewJsonRulesRepo(tempDir(t))

	rules, err := repo.Get(model.RulesId(9999))
	if err != nil {
		t.Fatalf("Expected fallback to default rules, got error: %v", err)
	}
	if rules == nil {
		t.Fatal("Fallback returned nil")
	}
	if rules.Id != model.DefaultRulesId {
		t.Errorf("Expected DefaultRulesId=%d, got %d", model.DefaultRulesId, rules.Id)
	}
}

func TestJsonRulesRepoSaveRoundTrip(t *testing.T) {
	repo := jsonStorage.NewJsonRulesRepo(tempDir(t))

	rules, _ := repo.Get(model.DefaultRulesId)
	rules.Id = model.RulesId(1)
	if err := repo.Save(rules); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := repo.Get(model.RulesId(1))
	if err != nil {
		t.Fatalf("Get after Save failed: %v", err)
	}
	if loaded.Id != model.RulesId(1) {
		t.Errorf("id mismatch: got %d, want 1", loaded.Id)
	}
}

func TestJsonRulesRepoGetCorruptedDefaultReturnsDefaults(t *testing.T) {
	dir := tempDir(t)
	repo := jsonStorage.NewJsonRulesRepo(dir)

	// Write corrupted JSON for the default rules file.
	defaultPath := filepath.Join(dir, "rules_0.json")
	if err := os.WriteFile(defaultPath, []byte("not valid json {{"), 0o644); err != nil {
		t.Fatalf("Failed to write corrupted file: %v", err)
	}

	rules, err := repo.Get(model.DefaultRulesId)
	if err != nil {
		t.Fatalf("Expected fallback to in-memory defaults, got error: %v", err)
	}
	if rules == nil {
		t.Error("Expected non-nil defaults despite corrupted file")
	}
}
