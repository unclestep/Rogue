package usecase_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/application/usecase"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
)

// TestSubmitUnknownPlaythroughIsNoop verifies that submitting an intent for a
// non-existent playthrough does not panic and has no side effects.
func TestSubmitUnknownPlaythroughIsNoop(t *testing.T) {
	playRepo, _ := newTestRepos()
	si := usecase.NewSubmitIntent(playRepo, service.NewMoveResolverService())

	// Must not panic.
	si.Submit("non-existent-id", &dto.Command{
		Action:     dto.ActionConsumeItem,
		PlayerUUID: hostUUID,
		ItemID:     1,
	})
}

// TestSubmitUnknownPlayerIsNoop verifies that submitting an intent for a UUID
// that does not belong to the session does not panic and adds no intent.
func TestSubmitUnknownPlayerIsNoop(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startSession(t, rs, hostUUID)

	si := usecase.NewSubmitIntent(playRepo, service.NewMoveResolverService())
	si.Submit(id, &dto.Command{
		Action:     dto.ActionConsumeItem,
		PlayerUUID: "stranger-uuid",
		ItemID:     1,
	})

	p, _ := playRepo.Get(id)
	if len(p.PendingIntents) != 0 {
		t.Errorf("Expected no pending intents for unknown player, got %d", len(p.PendingIntents))
	}
}

// TestSubmitConsumeAddsIntent verifies that ActionConsumeItem creates an
// IntentConsume entry in PendingIntents for the acting player.
func TestSubmitConsumeAddsIntent(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startSession(t, rs, hostUUID)

	si := usecase.NewSubmitIntent(playRepo, service.NewMoveResolverService())
	si.Submit(id, &dto.Command{
		Action:     dto.ActionConsumeItem,
		PlayerUUID: hostUUID,
		ItemID:     99,
	})

	p, err := playRepo.Get(id)
	if err != nil {
		t.Fatalf("Playthrough not found after submit: %v", err)
	}
	if len(p.PendingIntents) == 0 {
		t.Fatal("Expected a pending consume intent, got none")
	}
	for _, intent := range p.PendingIntents {
		if intent.IntentType != model.IntentConsume {
			t.Errorf("Expected IntentConsume, got %v", intent.IntentType)
		}
		if intent.ItemId != model.ItemId(99) {
			t.Errorf("Expected ItemId 99, got %v", intent.ItemId)
		}
	}
}

// TestSubmitEquipAddsIntent verifies that ActionEquipWeapon creates an
// IntentEquip entry in PendingIntents.
func TestSubmitEquipAddsIntent(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startSession(t, rs, hostUUID)

	si := usecase.NewSubmitIntent(playRepo, service.NewMoveResolverService())
	si.Submit(id, &dto.Command{
		Action:     dto.ActionEquipWeapon,
		PlayerUUID: hostUUID,
		ItemID:     7,
	})

	p, _ := playRepo.Get(id)
	if len(p.PendingIntents) == 0 {
		t.Fatal("Expected a pending equip intent, got none")
	}
	for _, intent := range p.PendingIntents {
		if intent.IntentType != model.IntentEquip {
			t.Errorf("Expected IntentEquip, got %v", intent.IntentType)
		}
	}
}

// TestSubmitUnequipAddsIntent verifies that ActionUnequipWeapon creates an
// IntentUnequip entry in PendingIntents.
func TestSubmitUnequipAddsIntent(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startSession(t, rs, hostUUID)

	si := usecase.NewSubmitIntent(playRepo, service.NewMoveResolverService())
	si.Submit(id, &dto.Command{
		Action:     dto.ActionUnequipWeapon,
		PlayerUUID: hostUUID,
		ItemID:     7,
	})

	p, _ := playRepo.Get(id)
	if len(p.PendingIntents) == 0 {
		t.Fatal("Expected a pending unequip intent, got none")
	}
	for _, intent := range p.PendingIntents {
		if intent.IntentType != model.IntentUnequip {
			t.Errorf("Expected IntentUnequip, got %v", intent.IntentType)
		}
	}
}

// TestSubmitMoveAddsIntent verifies that ActionMove creates a pending move-related
// intent when the player is in a started session with a valid dungeon.
// The move resolver checks the map for walkable tiles, so a dungeon must exist.
func TestSubmitMoveAddsIntent(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startGame(t, rs, hostUUID)

	si := usecase.NewSubmitIntent(playRepo, service.NewMoveResolverService())

	// Try all four cardinal directions — at least one must be valid in a generated dungeon.
	vectors := []dto.Vector{{X: 0, Y: -1}, {X: 0, Y: 1}, {X: 1, Y: 0}, {X: -1, Y: 0}}
	for _, vec := range vectors {
		// Clear pending intents between attempts.
		p, _ := playRepo.Get(id)
		p.PendingIntents = p.PendingIntents[:0]
		playRepo.Save(p)

		si.Submit(id, &dto.Command{
			Action:     dto.ActionMove,
			PlayerUUID: hostUUID,
			MoveVector: vec,
		})

		p, _ = playRepo.Get(id)
		if len(p.PendingIntents) > 0 {
			return // At least one valid move found — test passes.
		}
	}

	t.Error("Expected at least one valid move intent in a generated dungeon, but all directions were blocked")
}
