package port

import (
	"github.com/unclestep/Rogue/internal/domain/model"
)

type PlaythroughRepository interface {
	Get(playthroughId model.PlaythroughId) (*model.Playthrough, error)
	Create(rulesId model.RulesId, seed int64) *model.Playthrough
	Save(playthrough *model.Playthrough) error
	Delete(playthroughId model.PlaythroughId)
}

type RulesRepository interface {
	Get(rulesId model.RulesId) (*model.GameRules, error)
	// TODO: Create own rules in lobby (adjust difficulty etc.)
	Save(rules *model.GameRules) error
}
