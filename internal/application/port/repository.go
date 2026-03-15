package port

import (
	"github.com/unclestep/Rogue/internal/domain/model"
)

type PlaythroughRepository interface {
	// TODO: If playthrough does not exist, create new (in uce cases)
	Get(playthroughId model.PlaythroughId) (*model.Playthrough, error)
	Create(rulesId model.RulesId, seed int64) *model.Playthrough
	Save(playthrough *model.Playthrough) error // Should decide save to cache or JSON based on number of connected players (if all are disconnected save session in JSON)
	Delete(playthroughId model.PlaythroughId)
}

type RulesRepository interface {
	// TODO: If rulesId is invalid return default rules (default rules have id = 0) RULES REPOSITORY RESPONSIBILITY
	Get(rulesId model.RulesId) (*model.GameRules, error)
	// TODO: Create own rules in lobby (adjust difficulty etc.)
	Save(rules *model.GameRules)
}
