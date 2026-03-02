package fsm

import (
	"github.com/unclestep/Rogue/internal/domain/entity"
)

type GameLogic struct {
	States  map[entity.GameState]GameBehavior
	Session entity.GameSession
}
