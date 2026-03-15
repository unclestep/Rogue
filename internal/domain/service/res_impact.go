package service

import (
	"github.com/unclestep/Rogue/internal/domain/model"
	"math/rand"
)

type ImpactResolver struct{}

func NewImpactResolverService() *ImpactResolver {
	return &ImpactResolver{}
}

func (e *ImpactResolver) ResolveReactions(trigger model.TriggerType, subj *model.ActorImpact, obj *model.ActorImpact, rng *rand.Rand) {
	subjReactions := subj.CollectActorReactions(trigger)
	e.ResolveSpecificReactions(subjReactions, subj, obj, rng)
}

func (e *ImpactResolver) ResolveSpecificReactions(reactions []*model.Reaction, subj *model.ActorImpact, obj *model.ActorImpact, rng *rand.Rand) {
	for _, reaction := range reactions {
		if rng.Intn(model.Guaranteed) < reaction.Chance {
			var source, target *model.ActorImpact
			if reaction.Target == model.TargetSource {
				source = subj
				target = subj
			} else {
				if obj == nil {
					continue
				}
				source = subj
				target = obj
			}

			reaction.Perform(source, target)
		}
	}
}

// TickEffects - updates duration of all effects related to actor.
// If effects modify something every turn, it will be applied.
// Permanent effects stay permanent, unlimited charges are not spending.
func (e *ImpactResolver) TickEffects(ctx model.SessionContext, a *model.Actor) {
	impact := model.NewActorImpact(a)
	effects := a.CollectActiveEffects()

	e.ResolveReactions(model.TriggerEffectOnTurn, impact, impact, ctx.Rng())
	impact.DecrementAllRelatedCharges(model.TriggerEffectOnTurn)

	for _, effect := range effects {
		if effect.IsTemp() {
			effect.ExtendDuration(-1)
		}
		if effect.IsExpired() {
			impact.EffectsToRemove[effect] = true
		}
	}

	e.ApplyImpact(impact, ctx.Rng())
	a.RecomputeStats()

	if a.Vitals[model.VitalHP] <= 0 {
		ctx.Playthrough.KillActor(a)
	}

}

// Apply - applies all changes and removes expired, ran out or marked as needed to remove effects.
func (e *ImpactResolver) ApplyImpact(impact *model.ActorImpact, rng *rand.Rand) {
	actor := impact.Actor

	// Remove effects before updating the stats to take into account TriggerOnExpire instructions

	// Update effects charges and mark as needed to remove effects whose charges have run out
	for effect, change := range impact.EffectChargesChange {
		if effect.Charges != -1 {
			effect.Charges += change
			if effect.Charges <= 0 {
				impact.EffectsToRemove[effect] = true
			}
		}
	}

	// Resolve trigger on expire before removing
	for effect := range impact.EffectsToRemove {
		if expireReactions, exists := effect.Procs[model.TriggerEffectOnExpire]; exists {
			e.ResolveSpecificReactions(expireReactions, impact, impact, rng)
		}
	}

	// Remove all effects marked as needed to remove
	impact.RemoveEffects()

	// Following lines are updating actor's stats
	actor.Pos = actor.Pos.Add(impact.PosChange)

	for vital, change := range impact.VitalsChange {
		actor.Vitals[vital] += change
	}

	for attr, change := range impact.BaseAttrsChange {
		actor.BaseAttrs[attr] += change
	}

	for status, change := range impact.StatusesChange {
		actor.Statuses[status] += change
	}

	// Add applied effects
	for _, effect := range impact.AppliedEffects {
		actor.AddEffect(effect)
	}
}
