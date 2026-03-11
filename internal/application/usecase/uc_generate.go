package usecase

import (
	"math/rand"

	"github.com/unclestep/Rogue/internal/application/port"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
)

type DungGen struct {
	repo      port.SessionRepository
	rules     *model.GameRules
	diffCurve *model.DifficultyCurve
}

func NewDungGen(repo port.SessionRepository, rules *model.GameRules, diffCurve *model.DifficultyCurve) *DungGen {
	return &DungGen{
		repo:      repo,
		rules:     rules,
		diffCurve: diffCurve,
	}
}

func (d *DungGen) Execute(sessionId model.SessionId) error {
	session, err := d.repo.Get(sessionId)
	if err != nil {
		return err
	}

	rng := rand.New(rand.NewSource(session.Seed))

}
