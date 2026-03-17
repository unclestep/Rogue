package service

import (
	"github.com/unclestep/Rogue/internal/domain/model"
)

type Raycaster struct{}

func (f *Raycaster) Flashlight(area *model.VisibleArea) {
	_ = area
	return
}
