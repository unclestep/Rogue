package usecase_test

import (
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/dto"
)

const (
	hostUUID  = "host-uuid"
	guestUUID = "guest-uuid"
)

// TestResolveLobbyCreatedOnJoin verifies that sending ActionJoin with an empty
// PlaythroughId creates a new session and returns LobbyGameState.
func TestResolveLobbyCreatedOnJoin(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)

	state, id := rs.Resolve(joinCmd(hostUUID))

	if state != model.LobbyGameState {
		t.Errorf("Expected LobbyGameState, got %v", state)
	}
	if id == model.InvalidPlaythroughId {
		t.Error("Expected non-empty PlaythroughId after join, got empty")
	}
}

// TestResolveHostYesTransitionsToPlaying verifies that the host sending ActionYes
// transitions the session from Lobby to Playing and generates the dungeon.
func TestResolveHostYesTransitionsToPlaying(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startSession(t, rs, hostUUID)

	state, _ := rs.Resolve(&dto.Command{
		PlaythroughId: string(id),
		Action:        dto.ActionYes,
		PlayerUUID:    hostUUID,
	})

	if state != model.PlayingGameState {
		t.Errorf("Expected PlayingGameState after host ActionYes, got %v", state)
	}

	p, err := playRepo.Get(id)
	if err != nil {
		t.Fatalf("Playthrough not found after start: %v", err)
	}
	if p.Map == nil {
		t.Error("Expected dungeon to be generated after ActionYes, but Map is nil")
	}
}

// TestResolveNonHostYesStaysLobby verifies that a guest sending ActionYes
// does not change the state — only the host can start the game.
func TestResolveNonHostYesStaysLobby(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startSession(t, rs, hostUUID)

	// Guest joins the existing session.
	rs.Resolve(&dto.Command{
		PlaythroughId: string(id),
		Action:        dto.ActionJoin,
		PlayerUUID:    guestUUID,
	})

	state, _ := rs.Resolve(&dto.Command{
		PlaythroughId: string(id),
		Action:        dto.ActionYes,
		PlayerUUID:    guestUUID,
	})

	if state != model.LobbyGameState {
		t.Errorf("Expected LobbyGameState when non-host sends ActionYes, got %v", state)
	}
}

// TestResolveHostLeaveInPlayingDisconnectsAll verifies that when the host leaves
// during a game all players are hidden and the state reverts to Lobby.
func TestResolveHostLeaveInPlayingDisconnectsAll(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	// Both players join in Lobby then host starts the game so both are placed.
	id := startGameWithGuest(t, rs, hostUUID, guestUUID)

	state, _ := rs.Resolve(&dto.Command{
		PlaythroughId: string(id),
		Action:        dto.ActionLeave,
		PlayerUUID:    hostUUID,
	})

	if state != model.LobbyGameState {
		t.Errorf("Expected LobbyGameState after host leave, got %v", state)
	}

	p, err := playRepo.Get(id)
	if err != nil {
		t.Fatalf("Playthrough not found after host leave: %v", err)
	}
	for uuid, actorId := range p.PlayersUuid {
		if actor, exists := p.Players[actorId]; exists {
			if actor.Pos != model.NewInvalidPoint() {
				t.Errorf("Player %s should be hidden after DisconnectAll, but Pos is %v", uuid, actor.Pos)
			}
		}
	}
}

// TestResolveGuestLeaveDisconnectsOnlyGuest verifies that a guest leaving
// is hidden while the host remains on the map.
// Both players must join in Lobby before the game starts — joining during
// Playing state is a no-op (SubmitIntent has no ActionJoin case).
func TestResolveGuestLeaveDisconnectsOnlyGuest(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startGameWithGuest(t, rs, hostUUID, guestUUID)

	state, _ := rs.Resolve(&dto.Command{
		PlaythroughId: string(id),
		Action:        dto.ActionLeave,
		PlayerUUID:    guestUUID,
	})

	if state != model.PlayingGameState {
		t.Errorf("Expected PlayingGameState after guest leave, got %v", state)
	}

	p, err := playRepo.Get(id)
	if err != nil {
		t.Fatalf("Playthrough not found after guest leave: %v", err)
	}

	if guest := p.GetPlayer(guestUUID); guest != nil {
		if guest.Pos != model.NewInvalidPoint() {
			t.Errorf("Guest should be hidden after leave, but Pos is %v", guest.Pos)
		}
	}

	if host := p.GetPlayer(hostUUID); host != nil {
		if host.Pos == model.NewInvalidPoint() {
			t.Error("Host should remain visible after guest leave")
		}
	}
}

// TestResolveTurnExecuteAfterDisconnectDoesNotPanic verifies that Execute does
// not panic when players have been hidden (Pos = {-1,-1}) before the turn resolves.
// buildChaseMap now skips hidden players, so out-of-bounds access no longer occurs.
func TestResolveTurnExecuteAfterDisconnectDoesNotPanic(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	rt := buildResolveTurn(playRepo, rulesRepo)
	id := startGame(t, rs, hostUUID)

	// Force the host to a hidden position, simulating a mid-game disconnect.
	p, _ := playRepo.Get(id)
	p.DisconnectAll()
	playRepo.Save(p)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Execute panicked after player disconnect: %v", r)
		}
	}()

	rt.Execute(id)
}

// TestResolveUnknownIdReturnsNotFound verifies that joining with a non-existent
// PlaythroughId returns InvalidPlaythroughId — no silent session creation.
func TestResolveUnknownIdReturnsNotFound(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)

	state, id := rs.Resolve(&dto.Command{
		PlaythroughId: "non-existent-id-00000",
		Action:        dto.ActionJoin,
		PlayerUUID:    hostUUID,
		PlaythroughParams: &dto.PlaythroughParams{
			RulesId: int64(model.DefaultRulesId),
			Seed:    1,
		},
	})

	if state != model.LobbyGameState {
		t.Errorf("Expected LobbyGameState state value, got %v", state)
	}
	if id != model.InvalidPlaythroughId {
		t.Errorf("Expected InvalidPlaythroughId for unknown session, got %v", id)
	}
}

// TestResolveGameOverDeletesPlaythrough verifies that a playthrough in GameOver
// state is deleted from the repository after Resolve is called.
func TestResolveGameOverDeletesPlaythrough(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startSession(t, rs, hostUUID)

	// Force the session into GameOver state.
	p, _ := playRepo.Get(id)
	p.State = model.GameOverGameState
	playRepo.Save(p)

	rs.Resolve(&dto.Command{
		PlaythroughId: string(id),
		Action:        dto.ActionNo,
		PlayerUUID:    hostUUID,
	})

	_, err := playRepo.Get(id)
	if err == nil {
		t.Error("Expected playthrough to be deleted after GameOver resolution, but it still exists")
	}
}

// TestResolveGameOverYesReturnsLobby verifies that ActionYes in GameOver state
// transitions back to Lobby (play again flow).
func TestResolveGameOverYesReturnsLobby(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startSession(t, rs, hostUUID)

	p, _ := playRepo.Get(id)
	p.State = model.GameOverGameState
	playRepo.Save(p)

	state, _ := rs.Resolve(&dto.Command{
		PlaythroughId: string(id),
		Action:        dto.ActionYes,
		PlayerUUID:    hostUUID,
	})

	if state != model.LobbyGameState {
		t.Errorf("Expected LobbyGameState after ActionYes in GameOver, got %v", state)
	}
}

// TestLobbyJoinSamePlayerNotDuplicated verifies that joining with the same UUID
// twice does not create duplicate entries in PlayersUuid or Players.
func TestLobbyJoinSamePlayerNotDuplicated(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startSession(t, rs, hostUUID)

	// Second join with the same UUID — should be a no-op.
	rs.Resolve(&dto.Command{
		PlaythroughId: string(id),
		Action:        dto.ActionJoin,
		PlayerUUID:    hostUUID,
	})

	p, err := playRepo.Get(id)
	if err != nil {
		t.Fatalf("Playthrough not found: %v", err)
	}
	if len(p.PlayersUuid) != 1 {
		t.Errorf("Expected 1 player after double join, got %d", len(p.PlayersUuid))
	}
	if len(p.Players) != 1 {
		t.Errorf("Expected 1 actor in Players after double join, got %d", len(p.Players))
	}
}

// TestLobbyFirstPlayerIsHost verifies that the first player to join a session
// is assigned as the host.
func TestLobbyFirstPlayerIsHost(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startSession(t, rs, hostUUID)

	p, err := playRepo.Get(id)
	if err != nil {
		t.Fatalf("Playthrough not found: %v", err)
	}
	if !p.IsHost(hostUUID) {
		t.Error("Expected the first joining player to be the host")
	}
}

// TestLobbyTickDoesNotCreatePhantomPlayer verifies that an ActionTick sent by the game
// loop does not silently create a phantom player with an empty UUID in the lobby.
func TestLobbyTickDoesNotCreatePhantomPlayer(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startSession(t, rs, hostUUID)

	rs.Resolve(&dto.Command{
		PlaythroughId: string(id),
		Action:        dto.ActionTick,
	})

	p, err := playRepo.Get(id)
	if err != nil {
		t.Fatalf("Playthrough not found after tick: %v", err)
	}
	if len(p.PlayersUuid) != 1 {
		t.Errorf("Expected 1 player in lobby after tick, got %d (phantom player created)", len(p.PlayersUuid))
	}
}

// TestLobbyHostLeaveDeletesSession verifies that when the host leaves in Lobby
// state the session is deleted — without a host no one can start the game.
// Resolve returns InvalidPlaythroughId and a subsequent Get returns an error.
func TestLobbyHostLeaveDeletesSession(t *testing.T) {
	playRepo, rulesRepo := newTestRepos()
	rs := buildResolveState(playRepo, rulesRepo)
	id := startSession(t, rs, hostUUID)

	_, returnedId := rs.Resolve(&dto.Command{
		PlaythroughId: string(id),
		Action:        dto.ActionLeave,
		PlayerUUID:    hostUUID,
	})

	if returnedId != model.InvalidPlaythroughId {
		t.Errorf("Expected InvalidPlaythroughId after host leaves Lobby, got %v", returnedId)
	}

	_, err := playRepo.Get(id)
	if err == nil {
		t.Error("Expected session to be deleted after host leaves Lobby, but it still exists")
	}
}
