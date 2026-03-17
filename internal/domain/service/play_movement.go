package service

import (
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

//
//
// --- MOVEMENT SERVICE ---
//
//

type Movement struct {
	impactResolver *ImpactResolver
	pickupService  *Pickup
}

//
// -- CONSTRUCTOR --
//

func NewMovementService(ir *ImpactResolver, ps *Pickup) *Movement {
	m := &Movement{
		impactResolver: ir,
		pickupService:  ps,
	}

	return m
}

//
// -- MOVE EVENT --
//

type MoveEvent struct {
	Mover          *model.ActorImpact
	Outcome        MoveOutcome
	impactResolver *ImpactResolver
}

type MoveOutcome int

const (
	MoveOutcomeSuccess MoveOutcome = iota
	MoveOutcomeNoStamina
	MoveOutcomeCantMove
	MoveOutcomeNoSpace
)

func NewMoveEvent(actor *model.Actor, ir *ImpactResolver) *MoveEvent {
	return &MoveEvent{
		Mover:          model.NewActorImpact(actor),
		impactResolver: ir,
	}
}

func (event *MoveEvent) WasPerformed() bool {
	return event.Outcome == MoveOutcomeSuccess
}

func (event *MoveEvent) Perform(ctx *model.SessionContext) {
	if event == nil || !event.WasPerformed() {
		return
	}

	mover := event.Mover.Actor
	oldPos := mover.Pos

	event.impactResolver.ApplyImpact(event.Mover, ctx.Rng())

	newPos := mover.Pos
	ctx.Playthrough.Map.Move(oldPos, newPos)

	if newPos == ctx.Playthrough.Map.ExitPoint {
		if ctx.Playthrough.IsPlayer(event.Mover.Actor.Id) {
			ctx.Playthrough.HidePlayer(event.Mover.Actor.Id)
		}
	}
}

//
// -- MAIN MOVE FUNCTION --
//

func (m *Movement) ExecuteMove(ctx *model.SessionContext, mover *model.Actor, vector geometry.Point) []model.Event {
	if mover == nil {
		return nil
	}

	event := NewMoveEvent(mover, m.impactResolver)
	target := mover.Pos.Add(vector)

	if !mover.CanMove() {
		event.Outcome = MoveOutcomeCantMove
		return []model.Event{event}
	}

	if !ctx.Playthrough.Map.CanMoveTo(target) {
		event.Outcome = MoveOutcomeCantMove
		return []model.Event{event}
	}

	if !mover.HasStaminaForMove() {
		event.Outcome = MoveOutcomeNoStamina
		return []model.Event{event}
	}

	event.Mover.PosChange = vector
	event.Mover.VitalsChange[model.VitalStamina] -= mover.DerivedAttrs[model.AttrMoveStaminaCost]
	events := []model.Event{event}
	m.impactResolver.ResolveReactions(model.TriggerOnMove, event.Mover, nil, ctx.Rng())

	if itemId, _ := ctx.Playthrough.Map.GetItemID(target); itemId > 0 && mover.Kind == model.ActorPlayer {
		item := ctx.Playthrough.GetItem(model.ItemId(itemId))
		pickupEvent := m.pickupService.Pickup(mover, item)
		events = append(events, pickupEvent)
	}

	return events
}
