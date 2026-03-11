package service

import (
	"math/rand"

	"github.com/unclestep/Rogue/internal/application/port"
	"github.com/unclestep/Rogue/internal/domain/model"
)

type DungGen struct {
	rules         *model.GameRules
	diffCurve     *model.DifficultyCurve
	objectSpawner *ObjectSpawner
	doorLocker    *DoorLocker
}
