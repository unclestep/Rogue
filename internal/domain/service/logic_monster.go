package service

import (
	"cmp"
	"log"
	"slices"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/conv"
	"github.com/unclestep/Rogue/pkg/geometry"
)

type MonsterController struct {
	behaviors    map[model.BehaviorType]MonsterBehavior
	moveResolver *MoveResolver
}

func NewMonsterControllerService(pathfinder *Pathfinder, moveResolver *MoveResolver) *MonsterController {
	ctrl := &MonsterController{
		behaviors:    make(map[model.BehaviorType]MonsterBehavior),
		moveResolver: moveResolver,
	}
	ctrl.RegisterBehaviors(pathfinder)
	return ctrl
}

func (m *MonsterController) RegisterBehaviors(pathfinder *Pathfinder) {
	m.behaviors[model.BehaviorWander] = NewWanderBehavior(pathfinder)
	m.behaviors[model.BehaviorChase] = NewChaseBehavior(pathfinder)
}

type MonsterBehavior interface {
	Update(ctx *model.SessionContext, actor *model.Actor) (model.BehaviorType, *MonsterDecision)
}

type MonsterDecision struct {
	MoveVector geometry.Point
}

type Transition struct {
	Condition   func(ctx *model.SessionContext, monster *model.Actor) bool
	TargetState model.BehaviorType
}

func (m *MonsterController) Tick(ctx *model.SessionContext) map[model.ActorId]*model.Intent {
	monsters := conv.MapValsToSlice(ctx.Playthrough.Monsters)
	slices.SortFunc(monsters, func(a, b *model.Actor) int {
		return cmp.Compare(b.DerivedAttrs[model.AttrDexterity], a.DerivedAttrs[model.AttrDexterity])
	})

	intents := make(map[model.ActorId]*model.Intent, len(monsters))

	for _, monster := range monsters {
		if monster.Vitals[model.VitalHP] <= 0 {
			ctx.Playthrough.KillActor(monster)
			continue
		}

		// Handle chain of transitions
		for {
			initState := monster.State
			behavior, ok := m.behaviors[initState]
			if !ok {
				log.Fatalf("Unrecognized monster behavior: %v", initState)
			}
			nextState, decision := behavior.Update(ctx, monster)

			if decision != nil {
				intent := m.moveResolver.Resolve(ctx.Playthrough, monster, decision.MoveVector)
				intents[monster.Id] = intent
			}

			if nextState == initState {
				break
			}
			monster.State = nextState
		}
	}

	return intents
}

type WanderBehavior struct {
	pathfinder  *Pathfinder
	transitions []*Transition
}

func NewWanderBehavior(pathfinder *Pathfinder) *WanderBehavior {
	return &WanderBehavior{
		pathfinder: pathfinder,
		transitions: []*Transition{
			{
				Condition: func(ctx *model.SessionContext, m *model.Actor) bool {
					for _, p := range ctx.Playthrough.Players {
						dist := m.Pos.EuclideanDistance(p.Pos)
						if dist <= float64(m.DerivedAttrs[model.AttrHostility]) {
							return true
						}
					}
					return false
				},
				TargetState: model.BehaviorChase,
			},
		},
	}
}

func (wander *WanderBehavior) Update(ctx *model.SessionContext, monster *model.Actor) (model.BehaviorType, *MonsterDecision) {
	for _, transition := range wander.transitions {
		if transition.Condition(ctx, monster) {
			return transition.TargetState, nil
		}
	}

	scentMap := ctx.ScentMaps[model.ScentMapWander]
	newPos := wander.pathfinder.DijkstraFind(ctx, scentMap.Scent, monster)

	return model.BehaviorWander, &MonsterDecision{MoveVector: newPos.Sub(monster.Pos)}
}

type ChaseBehavior struct {
	pathfinder  *Pathfinder
	transitions []*Transition
}

func NewChaseBehavior(pathfinder *Pathfinder) *ChaseBehavior {
	return &ChaseBehavior{
		pathfinder: pathfinder,
		transitions: []*Transition{
			{
				Condition: func(ctx *model.SessionContext, m *model.Actor) bool {
					for _, p := range ctx.Playthrough.Players {
						dist := m.Pos.EuclideanDistance(p.Pos)
						if dist >= 1.5*float64(m.DerivedAttrs[model.AttrHostility]) {
							return true
						}
					}
					return false
				},
				TargetState: model.BehaviorWander,
			},
		},
	}
}

func (chase *ChaseBehavior) Update(ctx *model.SessionContext, monster *model.Actor) (model.BehaviorType, *MonsterDecision) {
	for _, transition := range chase.transitions {
		if transition.Condition(ctx, monster) {
			return transition.TargetState, nil
		}
	}

	scentMap := ctx.ScentMaps[model.ScentMapChase]
	newPos := chase.pathfinder.DijkstraFind(ctx, scentMap.Scent, monster)

	return model.BehaviorChase, &MonsterDecision{MoveVector: newPos.Sub(monster.Pos)}
}
