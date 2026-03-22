package usecase_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/application/port"
	"github.com/unclestep/Rogue/internal/application/usecase"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
	"github.com/unclestep/Rogue/internal/infrastructure/storage"
)

// memoryRulesRepo is an in-memory stub that always returns the default rules.
type memoryRulesRepo struct{}

func (m *memoryRulesRepo) Get(_ model.RulesId) (*model.GameRules, error) {
	return model.NewDefaultGameRules(), nil
}

func (m *memoryRulesRepo) Save(_ *model.GameRules) error { return nil }

// newTestRepos returns a fresh pair of in-memory repositories for each test.
func newTestRepos() (port.PlaythroughRepository, port.RulesRepository) {
	return storage.NewMemorySessionRepo(), &memoryRulesRepo{}
}

// buildResolveTurn wires up a ResolveTurn from repos without touching disk.
func buildResolveTurn(playRepo port.PlaythroughRepository, rulesRepo port.RulesRepository) *usecase.ResolveTurn {
	impactRes := service.NewImpactResolverService()
	pickup := service.NewPickupService()
	pathfinder := service.NewPathfinderService()
	moveResolver := service.NewMoveResolverService()

	return usecase.NewResolveTurn(
		playRepo, rulesRepo,
		impactRes, pickup,
		service.NewInteractorService(),
		service.NewItemUsageService(),
		service.NewCombatService(impactRes),
		service.NewMovementService(impactRes, pickup),
		pathfinder, moveResolver,
		service.NewMonsterControllerService(pathfinder, moveResolver),
		service.NewRaycaster(),
		service.NewTopologyGenerator(),
		service.NewDoorLocker(),
	)
}

// buildResolveState wires up the full ResolveState from repos without touching disk.
func buildResolveState(playRepo port.PlaythroughRepository, rulesRepo port.RulesRepository) *usecase.ResolveState {
	rt := buildResolveTurn(playRepo, rulesRepo)
	moveResolver := service.NewMoveResolverService()
	si := usecase.NewSubmitIntent(playRepo, moveResolver)
	return usecase.NewResolveState(playRepo, rulesRepo, rt, si, service.NewRaycaster())
}

// joinCmd returns a command that creates a new session for playerUUID.
func joinCmd(playerUUID string) *dto.Command {
	return &dto.Command{
		Action:     dto.ActionJoin,
		PlayerUUID: playerUUID,
		PlaythroughParams: &dto.PlaythroughParams{
			RulesId: int64(model.DefaultRulesId),
			Seed:    42,
		},
	}
}

// startSession joins a player and returns the created PlaythroughId.
func startSession(t *testing.T, rs *usecase.ResolveState, playerUUID string) model.PlaythroughId {
	t.Helper()
	_, id := rs.Resolve(joinCmd(playerUUID))
	if id == model.InvalidPlaythroughId {
		t.Fatal("Expected valid PlaythroughId after join, got empty")
	}
	return id
}

// startGame joins a player, then sends ActionYes to generate the dungeon.
// Returns the PlaythroughId of the started session.
func startGame(t *testing.T, rs *usecase.ResolveState, playerUUID string) model.PlaythroughId {
	t.Helper()
	id := startSession(t, rs, playerUUID)

	_, newId := rs.Resolve(&dto.Command{
		PlaythroughId: string(id),
		Action:        dto.ActionYes,
		PlayerUUID:    playerUUID,
	})
	if newId == model.InvalidPlaythroughId {
		t.Fatal("Expected valid PlaythroughId after ActionYes, got empty")
	}
	return newId
}

// startGameWithGuest joins the host, then the guest (both in Lobby), then starts
// the game so both players are placed in the dungeon. Returns the PlaythroughId.
// Note: joining during Playing state is a no-op in SubmitIntent (no ActionJoin case).
func startGameWithGuest(t *testing.T, rs *usecase.ResolveState, hostUUID, guestUUID string) model.PlaythroughId {
	t.Helper()
	id := startSession(t, rs, hostUUID)

	rs.Resolve(&dto.Command{
		PlaythroughId: string(id),
		Action:        dto.ActionJoin,
		PlayerUUID:    guestUUID,
	})

	_, newId := rs.Resolve(&dto.Command{
		PlaythroughId: string(id),
		Action:        dto.ActionYes,
		PlayerUUID:    hostUUID,
	})
	if newId == model.InvalidPlaythroughId {
		t.Fatal("Expected valid PlaythroughId after ActionYes with guest, got empty")
	}
	return newId
}
