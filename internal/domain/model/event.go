package model

import (
	"math/rand"
)

type Event interface {
	Perform(gs *GameSession, rng *rand.Rand)
}
