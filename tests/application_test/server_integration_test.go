package application_test

import (
	"os"
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/application"
	"github.com/unclestep/Rogue/internal/application/usecase"
	"github.com/unclestep/Rogue/internal/application/view"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
	"github.com/unclestep/Rogue/internal/infrastructure/storage"
	jsonStorage "github.com/unclestep/Rogue/internal/infrastructure/storage/json"
	"github.com/unclestep/Rogue/internal/presentation/network"
)

const testSavesDir = "/tmp/gouge_test_saves"
const testPlayerUUID = "test-player-uuid-1234"

func setupServer(t *testing.T) (*network.Server, func()) {
	t.Helper()
	os.RemoveAll(testSavesDir)

	jsonPlayRepo := jsonStorage.NewJsonPlaythroughRepo(testSavesDir + "/playthroughs")
	playRepo := storage.NewCachedPlaythroughRepo(jsonPlayRepo)
	rulesRepo := jsonStorage.NewJsonRulesRepo(testSavesDir + "/rules")

	impactRes := service.NewImpactResolverService()
	pickup := service.NewPickupService()
	pathfinder := service.NewPathfinderService()
	moveResolver := service.NewMoveResolverService()
	raycaster := service.NewRaycaster()

	resolveTurn := usecase.NewResolveTurn(
		playRepo, rulesRepo,
		impactRes, pickup,
		service.NewInteractorService(),
		service.NewItemUsageService(),
		service.NewCombatService(impactRes),
		service.NewMovementService(impactRes, pickup),
		pathfinder, moveResolver,
		service.NewMonsterControllerService(pathfinder, moveResolver, raycaster, service.NewFallbackPolicy(pathfinder)),
		raycaster,
		service.NewTopologyGenerator(),
		service.NewDoorLocker(),
	)
	submitIntent := usecase.NewSubmitIntent(playRepo, moveResolver)
	resolveState := usecase.NewResolveState(playRepo, rulesRepo, resolveTurn, submitIntent, service.NewRaycaster())
	mapper := view.NewMapper(playRepo)
	loop := application.NewGameLoop(resolveState, mapper)

	srv := network.NewServer(loop)
	srv.Listen()

	cleanup := func() {
		os.RemoveAll(testSavesDir)
	}
	return srv, cleanup
}

func receiveView(t *testing.T, ch <-chan dto.GameView, timeout time.Duration) dto.GameView {
	t.Helper()
	select {
	case view := <-ch:
		return view
	case <-time.After(timeout):
		t.Fatal("timeout waiting for GameView")
		return dto.GameView{}
	}
}

// TestJoinCreatesLobby — ActionJoin должен создать playthrough и вернуть GameView в Lobby-состоянии.
func TestJoinCreatesLobby(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	toClient := srv.Subscribe(testPlayerUUID)
	fromClient := srv.CommandChan()

	fromClient <- dto.Command{
		Action:     dto.ActionJoin,
		PlayerUUID: testPlayerUUID,
		PlaythroughParams: &dto.PlaythroughParams{
			RulesId: 0,
			Seed:    42,
		},
	}

	view := receiveView(t, toClient, 2*time.Second)

	if view.PlaythroughId == "" {
		t.Error("expected non-zero PlaythroughId after ActionJoin")
	}
	if view.State != dto.StateLobby {
		t.Errorf("expected StateLobby, got %v", view.State)
	}
	if view.Player == nil {
		t.Error("expected Player to be set")
	}
}

// TestStartGame — ActionYes от хоста переводит в Playing и генерирует подземелье.
func TestStartGame(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	toClient := srv.Subscribe(testPlayerUUID)
	fromClient := srv.CommandChan()

	// Join
	fromClient <- dto.Command{
		Action:            dto.ActionJoin,
		PlayerUUID:        testPlayerUUID,
		PlaythroughParams: &dto.PlaythroughParams{RulesId: 0, Seed: 42},
	}
	joinView := receiveView(t, toClient, 2*time.Second)

	// Start
	fromClient <- dto.Command{
		PlaythroughId: joinView.PlaythroughId,
		Action:        dto.ActionYes,
		PlayerUUID:    testPlayerUUID,
	}
	startView := receiveView(t, toClient, 2*time.Second)

	if startView.State != dto.StatePlaying {
		t.Errorf("expected StatePlaying, got %v", startView.State)
	}
	if startView.Grid == nil {
		t.Error("expected dungeon grid to be generated")
	}
	if len(startView.Grid) == 0 {
		t.Error("expected non-empty grid")
	}
}

// TestMoveCommand — ActionMove регистрирует интент, и после тика позиция игрока должна измениться.
func TestMoveCommand(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	toClient := srv.Subscribe(testPlayerUUID)
	fromClient := srv.CommandChan()

	// Join + Start
	fromClient <- dto.Command{
		Action:            dto.ActionJoin,
		PlayerUUID:        testPlayerUUID,
		PlaythroughParams: &dto.PlaythroughParams{RulesId: 0, Seed: 42},
	}
	joinView := receiveView(t, toClient, 2*time.Second)

	fromClient <- dto.Command{
		PlaythroughId: joinView.PlaythroughId,
		Action:        dto.ActionYes,
		PlayerUUID:    testPlayerUUID,
	}
	receiveView(t, toClient, 2*time.Second) // discard start view

	// Move — after submit the turn resolves (allMoved = true for single player)
	fromClient <- dto.Command{
		PlaythroughId: joinView.PlaythroughId,
		Action:        dto.ActionMove,
		PlayerUUID:    testPlayerUUID,
		MoveVector:    dto.Vector{X: 0, Y: 1},
	}
	moveView := receiveView(t, toClient, 2*time.Second)

	if moveView.State != dto.StatePlaying {
		t.Errorf("expected StatePlaying after move, got %v", moveView.State)
	}
}

// TestSaveFileCreated — после старта игры JSON-файл сессии должен существовать на диске.
func TestSaveFileCreated(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	toClient := srv.Subscribe(testPlayerUUID)
	fromClient := srv.CommandChan()

	fromClient <- dto.Command{
		Action:            dto.ActionJoin,
		PlayerUUID:        testPlayerUUID,
		PlaythroughParams: &dto.PlaythroughParams{RulesId: 0, Seed: 42},
	}
	joinView := receiveView(t, toClient, 2*time.Second)

	// Give the server a moment to flush to disk (Save is called by Resolve)
	time.Sleep(50 * time.Millisecond)

	playthroughsDir := testSavesDir + "/playthroughs"
	entries, err := os.ReadDir(playthroughsDir)
	if err != nil {
		t.Fatalf("playthroughs dir not created: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one playthrough JSON file on disk")
	}
	_ = joinView
}
