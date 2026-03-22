package usecase

import (
	"github.com/unclestep/Rogue/internal/application/port"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
	"github.com/unclestep/Rogue/pkg/geometry"
)

type SubmitIntent struct {
	repo         port.PlaythroughRepository
	moveResolver *service.MoveResolver
}

func NewSubmitIntent(repo port.PlaythroughRepository, moveResolver *service.MoveResolver) *SubmitIntent {
	return &SubmitIntent{
		repo:         repo,
		moveResolver: moveResolver,
	}
}

func (s *SubmitIntent) Submit(id model.PlaythroughId, cmd *dto.Command) {
	play, err := s.repo.Get(id)
	if err != nil || play == nil {
		// Session not found: intent cannot be submitted without a valid playthrough.
		return
	}
	player := play.GetPlayer(cmd.PlayerUUID)
	if player == nil {
		return
	}

	if play.HasPendingIntent(player.Id) {
		return
	}

	var intent *model.Intent
	switch cmd.Action {
	case dto.ActionMove:
		vector := geometry.Point{X: cmd.MoveVector.X, Y: cmd.MoveVector.Y}
		intent = s.moveResolver.Resolve(play, player, vector)
	case dto.ActionConsumeItem:
		intent = &model.Intent{
			IntentType: model.IntentConsume,
			Actor:      player.Id,
			ItemId:     model.ItemId(cmd.ItemID),
		}
	case dto.ActionEquipWeapon:
		intent = &model.Intent{
			IntentType: model.IntentEquip,
			Actor:      player.Id,
			ItemId:     model.ItemId(cmd.ItemID),
		}
	case dto.ActionUnequipWeapon:
		intent = &model.Intent{
			IntentType: model.IntentUnequip,
			Actor:      player.Id,
			ItemId:     model.ItemId(cmd.ItemID),
		}
	case dto.ActionWait:
		intent = &model.Intent{
			IntentType: model.IntentUnknown,
			Actor:      player.Id,
		}
	}

	if intent != nil {
		play.PendingIntents = append(play.PendingIntents, intent)
	}

	s.repo.Save(play)
}
