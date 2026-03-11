package usecase

import (
	"math/rand"

	"github.com/unclestep/Rogue/internal/application/port"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
)

type Combat struct {
	repo          port.SessionRepository
	combatService *service.Combat
}

func NewCombat(repo port.SessionRepository) *Combat {
	return &Combat{
		repo:          repo,
		combatService: &service.Combat{},
	}
}

func (c *Combat) Execute(sessionId model.SessionId, attackerId model.ActorId, defenderId model.ActorId) error {
	session, err := c.repo.Get(sessionId)
	if err != nil {
		return err
	}

	rng := rand.New(rand.NewSource(session.Seed))

	attacker := session.GetActor(attackerId)
	defender := session.GetActor(defenderId)

	event := c.combatService.ExecuteAttack(session, attacker, defender, rng)

	session.Seed = rng.Int63()
}
