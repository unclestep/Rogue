package usecase

import (
	"github.com/unclestep/Rogue/internal/application/port"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
	"github.com/unclestep/Rogue/pkg/geometry"
)

type SubmitIntent struct {
	repo port.PlaythroughRepository
}

func NewSubmitIntent(repo port.PlaythroughRepository) *SubmitIntent {
	return &SubmitIntent{
		repo: repo,
	}
}

func (s *SubmitIntent) Submit(id model.PlaythroughId, cmd *dto.Command) {
	play, _ := s.repo.Get(id)
	player := play.GetPlayer(cmd.PlayerUUID)

	switch cmd.Action {
	case dto.ActionMove:
		moveRes := service.NewMoveResolverService()
		vector := geometry.Point{X: cmd.MoveVector.X, Y: cmd.MoveVector.Y}
		intent := moveRes.Resolve(play, player, vector)
		if intent != nil {
			play.PendingIntents[player.Id] = intent
		}
	case dto.ActionConsumeItem:
		play.PendingIntents[player.Id] = &model.Intent{
			IntentType: model.IntentConsume,
			Actor:      player.Id,
			ItemId:     model.ItemId(cmd.ItemID),
		}
	case dto.ActionEquipWeapon:
		play.PendingIntents[player.Id] = &model.Intent{
			IntentType: model.IntentEquip,
			Actor:      player.Id,
			ItemId:     model.ItemId(cmd.ItemID),
		}
	case dto.ActionUnequipWeapon:
		play.PendingIntents[player.Id] = &model.Intent{
			IntentType: model.IntentUnequip,
			Actor:      player.Id,
			ItemId:     model.ItemId(cmd.ItemID),
		}
	}

	s.repo.Save(play)
}
