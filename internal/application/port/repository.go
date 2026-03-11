package port

import (
	"github.com/unclestep/Rogue/internal/domain/model"
)

type SessionRepository interface {
	Get(sessionId model.SessionId) (*model.GameSession, error)
	Save(session *model.GameSession) error
}
