package usecase

import (
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

func NewResolveState(playRepo port.PlaythroughRepository, rulesRepo port.RulesRepository) *ResolveState {
	r := &ResolveState{
		playRepo:  playRepo,
		rulesRepo: rulesRepo,
	}
	r.RegisterStates()
	return r
}

func (uc *ResolveState) RegisterStates() {
	uc.states[model.LobbyGameState] = NewLobby(uc.playRepo, uc.rulesRepo)
	uc.states[model.PlayingGameState] = NewPlaying(uc.playRepo, uc.rulesRepo)
	uc.states[model.GameOverGameState] = NewGameOver(uc.playRepo)
}

type StateBehavior interface {
	Update(playthrough *model.Playthrough, usrInp *dto.Command) model.GameState
}

func (uc *ResolveState) Resolve(usrInp *dto.Command) {
	playthrough, _ := uc.playRepo.Get(model.PlaythroughId(usrInp.PlaythroughId))
	curState := playthrough.State
	stateBehavior, _ := uc.states[curState]

	newState := stateBehavior.Update(playthrough, usrInp)
	if newState != curState {
		playthrough.State = newState
	}

	uc.playRepo.Save(playthrough)
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

func (behavior *Lobby) Update(playthrough *model.Playthrough, usrInp *dto.Command) model.GameState {
	newState := model.LobbyGameState

	if playthrough == nil || usrInp.PlaythroughId == model.InvalidPlaythroughId {
		behavior.Create(usrInp.PlayerUUID, usrInp.PlaythroughParams)
	} else if usrInp.PlaythroughId != model.InvalidPlaythroughId {
		behavior.Join(playthrough, usrInp.PlayerUUID)
	} else if usrInp.Action == dto.ActionYes && playthrough.IsHost(usrInp.PlayerUUID) {
		newState = model.PlayingGameState
	} else if usrInp.Action == dto.ActionLeave {
		if playthrough.IsHost(usrInp.PlayerUUID) {
			playthrough.DisconnectAll()
			newState = model.LobbyGameState
		} else {
			playthrough.DisconnectPlayer(usrInp.PlayerUUID)
		}
	}

	return newState
}

func (behavior *Lobby) Create(playerUuid string, playthroughParams *dto.PlaythroughParams) {
	rules, _ := behavior.rulesRepo.Get(model.RulesId(playthroughParams.RulesId))
	playthrough := behavior.playRepo.Create(rules.Id, playthroughParams.Seed)
	behavior.Join(playthrough, playerUuid)
}

func (behavior *Lobby) Join(playthrough *model.Playthrough, playerUuid string) {
	if _, exists := playthrough.PlayersUuid[playerUuid]; !exists {
		rules, _ := behavior.rulesRepo.Get(playthrough.RulesId)
		objSpawner := service.NewObjectSpawner(rules)
		objSpawner.CreatePlayer(playthrough, playerUuid, model.ActorLabelPlayerCommon)
	}
}

//
// --- PLAYING STATE ---
//

type Playing struct {
	playRepo     port.PlaythroughRepository
	rulesRepo    port.RulesRepository
	resolveTurn  *ResolveTurn
	submitIntent *SubmitIntent
}

func NewPlaying(playRepo port.PlaythroughRepository, rulesRepo port.RulesRepository) *Playing {
	return &Playing{
		playRepo:     playRepo,
		rulesRepo:    rulesRepo,
		resolveTurn:  NewResolveTurn(playRepo, rulesRepo),
		submitIntent: NewSubmitIntent(playRepo),
	}
}

func (behavior *Playing) Update(playthrough *model.Playthrough, usrInp *dto.Command) model.GameState {
	if usrInp == nil {
		return model.PlayingGameState
	}

	newState := model.PlayingGameState

	if usrInp.Action == dto.ActionLeave {
		if playthrough.IsHost(usrInp.PlayerUUID) {
			playthrough.DisconnectAll()
			newState = model.LobbyGameState
		} else {
			playthrough.DisconnectPlayer(usrInp.PlayerUUID)
		}
	}

	if usrInp.Action != dto.ActionTick {
		behavior.submitIntent.Submit(playthrough.PlaythroughId, usrInp)
	}

	rules, _ := behavior.rulesRepo.Get(model.RulesId(playthrough.RulesId))

	// If after loading save
	if playthrough.TurnDeadline.IsZero() {
		playthrough.TurnDeadline = time.Now().Add(time.Duration(rules.TimeForMove) * time.Second)
	}

	allMoved := len(playthrough.GetAlivePlayers()) == len(playthrough.PendingIntents)
	timeExpired := time.Now().After(playthrough.TurnDeadline)

	if allMoved || timeExpired {
		newState = behavior.resolveTurn.Execute(playthrough.PlaythroughId)

		playthrough.TurnDeadline = time.Now().Add(time.Duration(rules.TimeForMove) * time.Second)
	}

	return newState

}

//
// --- GAMEOVER STATE ---
//

type GameOver struct {
	playRepo port.PlaythroughRepository
}

func NewGameOver(playRepo port.PlaythroughRepository) *GameOver {
	return &GameOver{
		playRepo: playRepo,
	}
}

func (behavior *GameOver) Update(playthrough *model.Playthrough, usrInp *dto.Command) model.GameState {
	behavior.playRepo.Delete(playthrough.PlaythroughId)

	newState := model.GameOverGameState

	if usrInp.Action == dto.ActionYes {
		newState = model.LobbyGameState
	} else if usrInp.Action == dto.ActionNo || usrInp.Action == dto.ActionLeave {
		if playthrough.IsHost(usrInp.PlayerUUID) {
			playthrough.DisconnectAll()
			newState = model.LobbyGameState
		} else {
			playthrough.DisconnectPlayer(usrInp.PlayerUUID)
		}
	}

	return newState
}
