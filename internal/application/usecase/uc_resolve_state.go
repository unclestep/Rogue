package usecase

import (
	"log"
	"time"

	"github.com/unclestep/Rogue/internal/application/port"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
)

type ResolveState struct {
	states    map[model.GameState]StateBehavior
	playRepo  port.PlaythroughRepository
	rulesRepo port.RulesRepository
}

func NewResolveState(
	playRepo port.PlaythroughRepository,
	rulesRepo port.RulesRepository,
	resolveTurn *ResolveTurn,
	submitIntent *SubmitIntent,
	raycaster *service.Raycaster,
) *ResolveState {
	r := &ResolveState{
		states:    make(map[model.GameState]StateBehavior),
		playRepo:  playRepo,
		rulesRepo: rulesRepo,
	}
	r.registerStates(resolveTurn, submitIntent, raycaster)
	return r
}

func (uc *ResolveState) registerStates(resolveTurn *ResolveTurn, submitIntent *SubmitIntent, raycaster *service.Raycaster) {
	if uc.states == nil {
		uc.states = make(map[model.GameState]StateBehavior)
	}
	uc.states[model.LobbyGameState] = NewLobby(uc.playRepo, uc.rulesRepo)
	uc.states[model.PlayingGameState] = NewPlaying(uc.rulesRepo, resolveTurn, submitIntent, raycaster)
	uc.states[model.GameOverGameState] = NewGameOver(uc.playRepo)
}

// StateBehavior returns the new game state and the (possibly newly created) playthrough.
type StateBehavior interface {
	Update(playthrough *model.Playthrough, usrInp *dto.Command) (model.GameState, *model.Playthrough)
}

// Resolve dispatches the command to the correct state handler and returns
// the resulting game state plus the playthrough ID that was acted upon.
func (uc *ResolveState) Resolve(usrInp *dto.Command) (model.GameState, model.PlaythroughId) {
	requestedId := model.PlaythroughId(usrInp.PlaythroughId)

	// Attempt to load existing playthrough; nil means either "new session" (empty ID)
	// or "not found" (non-empty ID) — Lobby.Update distinguishes the two cases.
	var playthrough *model.Playthrough
	if requestedId != model.InvalidPlaythroughId {
		playthrough, _ = uc.playRepo.Get(requestedId)
	}

	var curState model.GameState
	if playthrough != nil {
		curState = playthrough.State
	} else {
		curState = model.LobbyGameState
	}

	stateBehavior, ok := uc.states[curState]
	if !ok {
		log.Fatalf("[ERROR] State behavior %v is not implemented", curState)
	}
	newState, updatedPlaythrough := stateBehavior.Update(playthrough, usrInp)

	if updatedPlaythrough == nil {
		return newState, model.InvalidPlaythroughId
	}

	if newState != curState {
		updatedPlaythrough.State = newState
	}

	if curState != model.GameOverGameState {
		_ = uc.playRepo.Save(updatedPlaythrough)
	}

	return newState, updatedPlaythrough.PlaythroughId
}

//
// --- LOBBY STATE ---
//

type Lobby struct {
	playRepo  port.PlaythroughRepository
	rulesRepo port.RulesRepository
}

func NewLobby(playRepo port.PlaythroughRepository, rulesRepo port.RulesRepository) *Lobby {
	return &Lobby{
		playRepo:  playRepo,
		rulesRepo: rulesRepo,
	}
}

func (behavior *Lobby) Update(playthrough *model.Playthrough, usrInp *dto.Command) (model.GameState, *model.Playthrough) {
	newState := model.LobbyGameState

	if playthrough == nil {
		if model.PlaythroughId(usrInp.PlaythroughId) != model.InvalidPlaythroughId {
			// Non-empty ID supplied but session does not exist — signal "not found" to the caller.
			return newState, nil
		}
		if usrInp.PlaythroughParams == nil {
			// New game requested without parameters — nothing to create.
			return newState, nil
		}
		playthrough = behavior.Create(usrInp.PlayerUUID, usrInp.PlaythroughParams, usrInp.PlayerNickname)
	} else if usrInp.Action == dto.ActionYes && playthrough.IsHost(usrInp.PlayerUUID) {
		newState = model.PlayingGameState
		behavior.Start(playthrough)
	} else if usrInp.Action == dto.ActionLeave {
		if playthrough.IsHost(usrInp.PlayerUUID) {
			// Host leaving destroys the lobby — no one else can start the game.
			behavior.playRepo.Delete(playthrough.PlaythroughId)
			return newState, nil
		}
		playthrough.DisconnectPlayer(usrInp.PlayerUUID)
	} else if usrInp.Action == dto.ActionJoin {
		behavior.Join(playthrough, usrInp.PlayerUUID, usrInp.PlayerNickname)
	}
	// ActionTick and any other no-op in lobby state are silently ignored.

	return newState, playthrough
}

func (behavior *Lobby) Create(playerUuid string, playthroughParams *dto.PlaythroughParams, nickname string) *model.Playthrough {
	rules, _ := behavior.rulesRepo.Get(model.RulesId(playthroughParams.RulesId))
	playthrough := behavior.playRepo.Create(rules.Id, playthroughParams.Seed)
	behavior.Join(playthrough, playerUuid, nickname)
	return playthrough
}

func (behavior *Lobby) Start(playthrough *model.Playthrough) {
	rules, _ := behavior.rulesRepo.Get(playthrough.RulesId)
	generator := service.NewDungeonGeneratorService(rules, service.NewTopologyGenerator(), service.NewObjectSpawner(rules), service.NewDoorLocker())
	ctx := model.NewSessionContext(playthrough)
	if !playthrough.IsDungeonCreated() {
		generator.Gen(ctx)
	} else {
		// Resume a saved game: reposition any hidden-but-alive players at the entrance.
		generator.ShowPlayers(ctx)
	}
	playthrough.Seed = ctx.Rng().Int63()
}

func (behavior *Lobby) Join(playthrough *model.Playthrough, playerUuid, nickname string) {
	if _, exists := playthrough.PlayersUuid[playerUuid]; !exists {
		rules, _ := behavior.rulesRepo.Get(playthrough.RulesId)
		objSpawner := service.NewObjectSpawner(rules)
		objSpawner.CreatePlayer(playthrough, playerUuid, model.ActorLabelPlayerCommon)
	}
	// Always refresh the nickname in case the player reconnects with a new one.
	if nickname != "" {
		if playthrough.PlayersNicknames == nil {
			playthrough.PlayersNicknames = make(map[string]string)
		}
		playthrough.PlayersNicknames[playerUuid] = nickname
	}
}

//
// --- PLAYING STATE ---
//

type Playing struct {
	rulesRepo    port.RulesRepository
	resolveTurn  *ResolveTurn
	submitIntent *SubmitIntent
	raycaster    *service.Raycaster
}

func NewPlaying(rulesRepo port.RulesRepository, resolveTurn *ResolveTurn, submitIntent *SubmitIntent, raycaster *service.Raycaster) *Playing {
	return &Playing{
		rulesRepo:    rulesRepo,
		resolveTurn:  resolveTurn,
		submitIntent: submitIntent,
		raycaster:    raycaster,
	}
}

func (behavior *Playing) Update(playthrough *model.Playthrough, usrInp *dto.Command) (model.GameState, *model.Playthrough) {
	if usrInp == nil {
		return model.PlayingGameState, playthrough
	}

	// On the first tick after dungeon generation players have no FOW entry yet.
	// Initialize it here so the very next broadcast shows the flashlight cone
	// rather than a fully dark screen.
	if playthrough.Map != nil {
		for _, actorId := range playthrough.PlayersUuid {
			if playthrough.PlayersFoW[actorId] == nil {
				behavior.raycaster.RefreshPlayerFoW(playthrough, actorId, service.FlashlightHalfFOV, service.FlashlightRange)
			}
		}
	}

	newState := model.PlayingGameState

	if usrInp.Action == dto.ActionLeave {
		if playthrough.IsHost(usrInp.PlayerUUID) {
			playthrough.DisconnectAll()
			newState = model.LobbyGameState
		} else {
			playthrough.DisconnectPlayer(usrInp.PlayerUUID)
		}
	} else if usrInp.Action == dto.ActionAim {
		// Lightweight fast-path: update aim angle and recompute FOW for this
		// player only. Does NOT queue a turn intent or trigger turn resolution.
		player := playthrough.GetPlayer(usrInp.PlayerUUID)
		if player != nil && playthrough.Map != nil {
			if playthrough.PlayerAimAngles == nil {
				playthrough.PlayerAimAngles = make(map[model.ActorId]float64)
			}
			playthrough.PlayerAimAngles[player.Id] = usrInp.AimAngle
			behavior.raycaster.RefreshPlayerFoW(playthrough, player.Id, service.FlashlightHalfFOV, service.FlashlightRange)
		}
		return model.PlayingGameState, playthrough
	} else if usrInp.Action != dto.ActionTick {
		behavior.submitIntent.Submit(playthrough.PlaythroughId, usrInp)
	}

	rules, _ := behavior.rulesRepo.Get(playthrough.RulesId)

	timedMode := rules.TimeForMove > 0
	if timedMode && playthrough.TurnDeadline.IsZero() {
		playthrough.TurnDeadline = time.Now().Add(time.Duration(rules.TimeForMove) * time.Second)
	}

	allMoved := len(playthrough.GetAlivePlayers()) == playthrough.PendingPlayerIntentCount()
	// timeExpired is only relevant in timed mode; with TimeForMove == 0 the turn
	// resolves exclusively when all alive players have submitted their intent.
	timeExpired := timedMode && time.Now().After(playthrough.TurnDeadline)

	if allMoved || timeExpired {
		newState = behavior.resolveTurn.Execute(playthrough.PlaythroughId)
		if timedMode {
			playthrough.TurnDeadline = time.Now().Add(time.Duration(rules.TimeForMove) * time.Second)
		}
	}

	return newState, playthrough
}

//
// --- GAME OVER STATE ---
//

type GameOver struct {
	playRepo port.PlaythroughRepository
}

func NewGameOver(playRepo port.PlaythroughRepository) *GameOver {
	return &GameOver{
		playRepo: playRepo,
	}
}

func (behavior *GameOver) Update(playthrough *model.Playthrough, usrInp *dto.Command) (model.GameState, *model.Playthrough) {
	switch usrInp.Action {
	case dto.ActionYes:
		behavior.playRepo.Delete(playthrough.PlaythroughId)
		return model.LobbyGameState, nil
	case dto.ActionNo, dto.ActionLeave:
		if playthrough.IsHost(usrInp.PlayerUUID) {
			behavior.playRepo.Delete(playthrough.PlaythroughId)
			return model.LobbyGameState, nil
		}
		playthrough.DisconnectPlayer(usrInp.PlayerUUID)
	default:
	}

	return model.GameOverGameState, playthrough
}
