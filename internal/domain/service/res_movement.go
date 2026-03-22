package service

import (
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

type MoveResolver struct {
	handlers []ActionHandler
}

func NewMoveResolverService() *MoveResolver {
	resolver := &MoveResolver{
		handlers: make([]ActionHandler, 0),
	}
	resolver.RegisterAll()
	return resolver
}

func (m *MoveResolver) RegisterAll() {
	if m.handlers == nil {
		m.handlers = make([]ActionHandler, 0)
	}
	m.RegisterHandler(&AttackHandler{})
	m.RegisterHandler(&DoorBumpHandler{})
	m.RegisterHandler(&MoveHandler{})
}

func (m *MoveResolver) RegisterHandler(handler ActionHandler) {
	m.handlers = append(m.handlers, handler)
}

func (m *MoveResolver) Resolve(play *model.Playthrough, mover *model.Actor, vector geometry.Point) *model.Intent {
	for _, handler := range m.handlers {
		if intent, handled := handler.Handle(play, mover, vector); handled {
			return intent
		}
	}

	return nil
}

//
// -- HANDLERS --
//

type ActionHandler interface {
	Handle(play *model.Playthrough, actor *model.Actor, vector geometry.Point) (*model.Intent, bool)
}

type AttackHandler struct{}

func (a *AttackHandler) Handle(play *model.Playthrough, actor *model.Actor, vector geometry.Point) (*model.Intent, bool) {
	// Primitive check for enemy on tile
	// All needed checks should be in combat service
	if defenderId, _ := play.Map.GetActorID(actor.Pos.Add(vector)); defenderId > 0 {
		return &model.Intent{
				IntentType: model.IntentAttack,
				Actor:      actor.Id,
				Defender:   model.ActorId(defenderId),
			},
			true
	}

	return nil, false
}

type DoorBumpHandler struct{}

func (d *DoorBumpHandler) Handle(play *model.Playthrough, actor *model.Actor, vector geometry.Point) (*model.Intent, bool) {
	target := actor.Pos.Add(vector)

	if play.Map.IsClosedDoor(target) {
		return &model.Intent{
				IntentType: model.IntentInteract,
				Actor:      actor.Id,
				Vector:     vector,
			},
			true
	}

	return nil, false
}

type MoveHandler struct{}

func (m *MoveHandler) Handle(play *model.Playthrough, actor *model.Actor, vector geometry.Point) (*model.Intent, bool) {
	// Primitive check for tile walkability
	// All needed checks should be in movement service
	if actor.CanMove() && play.Map.IsWalkable(actor.Pos.Add(vector)) {
		return &model.Intent{
				IntentType: model.IntentMove,
				Actor:      actor.Id,
				Vector:     vector,
			},
			true
	}

	return nil, false
}
