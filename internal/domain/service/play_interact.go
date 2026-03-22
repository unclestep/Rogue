package service

import (
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

type Interactor struct{}

func NewInteractorService() *Interactor {
	return &Interactor{}
}

type InteractionEvent struct {
	Outcome    InteractionEventOutcome
	DoorToOpen geometry.Point
}

func (event *InteractionEvent) Perform(ctx *model.SessionContext) {
	if event.Outcome != InteractionEventOutcomeSuccess {
		return
	}

	if event.DoorToOpen != model.NewInvalidPoint() {
		ctx.Playthrough.Map.OpenDoor(event.DoorToOpen)
	}
}

type InteractionEventOutcome int

const (
	InteractionEventOutcomeFailed InteractionEventOutcome = iota
	InteractionEventOutcomeSuccess
)

func NewInteractionEvent() *InteractionEvent {
	return &InteractionEvent{
		DoorToOpen: model.NewInvalidPoint(),
	}
}

func (i *Interactor) Execute(ctx *model.SessionContext, actor *model.Actor, target geometry.Point) *InteractionEvent {
	if actor == nil {
		return nil
	}

	m := ctx.Playthrough.Map
	event := NewInteractionEvent()

	if m.IsClosedDoor(target) {
		keySlot := actor.Backpack.GetSlot(model.ItemTypeKey)
		for _, key := range keySlot {
			doorKeyhole, ok := m.GetDoorKeyhole(target)
			if ok && doorKeyhole == key.Keyhole || key.Keyhole == model.MasterKeyhole {
				event.Outcome = InteractionEventOutcomeSuccess
				event.DoorToOpen = target
				break
			}
		}
	}

	return event
}
