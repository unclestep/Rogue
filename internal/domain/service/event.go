package service

import (
	"math/rand"

	"github.com/unclestep/Rogue/internal/domain/entity"
)

type Event interface {
	Perform(gs *entity.GameSession, rng *rand.Rand)
}
