package service_test

import (
	"math/rand"
	"testing"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/geometry"
)

//
// -- HELPERS --
//

func newImpactResolver() *service.ImpactResolver {
	return service.NewImpactResolverService()
}

func newDeterministicRng() *rand.Rand {
	return rand.New(rand.NewSource(0))
}

func setupImpactPlayer() (*model.Playthrough, *model.Actor) {
	play := model.NewPlaythrough("", 0, 0)
	actor := model.NewDefaultPlayer(1, geometry.Point{X: 5, Y: 5}, 9)
	play.Players[actor.Id] = actor
	return play, actor
}

func setupImpactMonsterWithMap() (*model.SessionContext, *model.Actor) {
	play := model.NewPlaythrough("", 0, 0)

	grid := make([][]model.Cell, 5)
	for i := range grid {
		grid[i] = make([]model.Cell, 5)
		for j := range grid[i] {
			grid[i][j] = model.Cell{Type: model.Floor, RoomId: 1}
		}
	}

	blueprint := &model.MapBlueprint{
		Width:    5,
		Height:   5,
		TileGrid: grid,
		Rooms: []*model.Room{
			{Id: 1, Pos: geometry.Point{X: 0, Y: 0}, Width: 5, Height: 5},
		},
		EntranceRoomId: 1,
		ExitRoomId:     1,
		ExitPoint:      model.NewInvalidPoint(),
	}

	play.Map = model.NewMapFromBlueprint(blueprint)

	monster := model.NewDefaultZombie(2, geometry.Point{X: 2, Y: 2})
	play.Monsters[monster.Id] = monster
	play.Map.SetActor(monster.Pos, int64(monster.Id))

	ctx := model.NewSessionContext(play)
	return ctx, monster
}

//
// -- APPLY IMPACT --
//

//
// -- VitalsChange --
//

func TestApplyImpactVitalsChange(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	initialHP := actor.Vitals[model.VitalHP]

	impact := model.NewActorImpact(actor)
	impact.VitalsChange[model.VitalHP] = -30

	resolver.ApplyImpact(impact, rng)

	if actor.Vitals[model.VitalHP] != initialHP-30 {
		t.Errorf("Expected HP=%d, got %d", initialHP-30, actor.Vitals[model.VitalHP])
	}
}

func TestApplyImpactMultipleVitalsAccumulate(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	initialHP := actor.Vitals[model.VitalHP]
	initialStamina := actor.Vitals[model.VitalStamina]

	impact := model.NewActorImpact(actor)
	impact.VitalsChange[model.VitalHP] = -10
	impact.VitalsChange[model.VitalStamina] = -25

	resolver.ApplyImpact(impact, rng)

	if actor.Vitals[model.VitalHP] != initialHP-10 {
		t.Errorf("Expected HP=%d, got %d", initialHP-10, actor.Vitals[model.VitalHP])
	}
	if actor.Vitals[model.VitalStamina] != initialStamina-25 {
		t.Errorf("Expected Stamina=%d, got %d", initialStamina-25, actor.Vitals[model.VitalStamina])
	}
}

//
// -- BaseAttrsChange --
//

func TestApplyImpactBaseAttrsChangeUpdatesBaseAndDerived(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	baseStrength := actor.BaseAttrs[model.AttrStrength]

	impact := model.NewActorImpact(actor)
	impact.BaseAttrsChange[model.AttrStrength] = 10

	resolver.ApplyImpact(impact, rng)

	if actor.BaseAttrs[model.AttrStrength] != baseStrength+10 {
		t.Errorf("Expected BaseAttrs Strength=%d, got %d", baseStrength+10, actor.BaseAttrs[model.AttrStrength])
	}
	// RecomputeStats must propagate the new base into derived.
	if actor.DerivedAttrs[model.AttrStrength] != baseStrength+10 {
		t.Errorf("Expected DerivedAttrs Strength=%d after recompute, got %d", baseStrength+10, actor.DerivedAttrs[model.AttrStrength])
	}
}

//
// -- StatusesChange --
//

// ApplyImpact writes impact.StatusesChange into actor.Statuses, but the
// immediately following RecomputeStats resets statuses to zero and recomputes
// only from active effects. A direct status change that is not backed by an
// effect is therefore overwritten. The test below documents this behavior and
// ensures the more reliable path — applying a status via AppliedEffects — works
// correctly.
func TestApplyImpactStatusViaEffectPersistsAfterRecompute(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	impact := model.NewActorImpact(actor)
	impact.AppliedEffects[model.EffectSleep] = model.NewSleepEffect(5)

	resolver.ApplyImpact(impact, rng)

	if actor.Statuses[model.StatusSleep] != 1 {
		t.Errorf("Expected StatusSleep=1 after applying sleep effect via AppliedEffects, got %d", actor.Statuses[model.StatusSleep])
	}
}

func TestApplyImpactDirectStatusChangeIsResetByRecomputeStats(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	impact := model.NewActorImpact(actor)
	impact.StatusesChange[model.StatusSleep] = 1

	resolver.ApplyImpact(impact, rng)

	// Direct status change through impact.StatusesChange is applied first,
	// then immediately overwritten by RecomputeStats which rebuilds statuses
	// from active effects only. With no sleep effect on the actor, the result
	// must be 0.
	if actor.Statuses[model.StatusSleep] != 0 {
		t.Errorf("Expected StatusSleep=0 after RecomputeStats with no backing effect, got %d", actor.Statuses[model.StatusSleep])
	}
}

//
// -- PosChange --
//

func TestApplyImpactPosChange(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	actor.Pos = geometry.Point{X: 3, Y: 4}

	impact := model.NewActorImpact(actor)
	impact.PosChange = geometry.Point{X: 2, Y: -1}

	resolver.ApplyImpact(impact, rng)

	expected := geometry.Point{X: 5, Y: 3}
	if actor.Pos != expected {
		t.Errorf("Expected Pos=%v, got %v", expected, actor.Pos)
	}
}

func TestApplyImpactZeroPosChangeKeepsPosition(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	original := actor.Pos

	impact := model.NewActorImpact(actor)
	// PosChange defaults to {0,0}

	resolver.ApplyImpact(impact, rng)

	if actor.Pos != original {
		t.Errorf("Expected position unchanged %v, got %v", original, actor.Pos)
	}
}

//
// -- AppliedEffects --
//

func TestApplyImpactAppliedEffectsAddedToActor(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	impact := model.NewActorImpact(actor)
	impact.AppliedEffects[model.EffectSleep] = model.NewSleepEffect(3)

	resolver.ApplyImpact(impact, rng)

	if _, exists := actor.Effects[model.EffectSleep]; !exists {
		t.Errorf("Expected EffectSleep to be added to actor after ApplyImpact")
	}
}

//
// -- EffectsToRemove --
//

func TestApplyImpactEffectsToRemoveDeletesEffectFromActor(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	eff := model.NewFatigueEffect(5)
	actor.Effects[model.EffectFatigue] = eff

	impact := model.NewActorImpact(actor)
	impact.EffectsToRemove[eff] = true

	resolver.ApplyImpact(impact, rng)

	if _, exists := actor.Effects[model.EffectFatigue]; exists {
		t.Errorf("Expected EffectFatigue to be removed from actor")
	}
}

func TestApplyImpactEffectChargesDecrementedToZeroTriggersRemoval(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	eff := &model.Effect{
		Kind:     model.EffectFatigue,
		Duration: -1,
		Charges:  1,
	}
	actor.Effects[model.EffectFatigue] = eff

	impact := model.NewActorImpact(actor)
	impact.EffectChargesChange[eff] = -1

	resolver.ApplyImpact(impact, rng)

	if _, exists := actor.Effects[model.EffectFatigue]; exists {
		t.Errorf("Expected EffectFatigue to be removed after charges reach 0")
	}
}

func TestApplyImpactUnlimitedChargesNotAffectedByChargesChange(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	eff := &model.Effect{
		Kind:     model.EffectFatigue,
		Duration: -1,
		Charges:  -1, // unlimited
	}
	actor.Effects[model.EffectFatigue] = eff

	impact := model.NewActorImpact(actor)
	impact.EffectChargesChange[eff] = -5

	resolver.ApplyImpact(impact, rng)

	// Unlimited effect must stay and its charges must remain -1.
	got, exists := actor.Effects[model.EffectFatigue]
	if !exists {
		t.Fatalf("Expected unlimited effect to remain in actor")
	}
	if got.Charges != -1 {
		t.Errorf("Expected unlimited charges to stay -1, got %d", got.Charges)
	}
}

//
// -- TriggerEffectOnExpire --
//

func TestApplyImpactOnExpireProcFiresBeforeRemoval(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	expireReaction := &model.Reaction{
		Trigger: model.TriggerEffectOnExpire,
		Target:  model.TargetSource,
		Chance:  model.Guaranteed,
		EffectsToApply: map[model.EffectType]*model.Effect{
			model.EffectSleep: model.NewSleepEffect(2),
		},
	}

	expiringEffect := &model.Effect{
		Kind:     model.EffectFatigue,
		Duration: -1,
		Charges:  -1,
		Procs: map[model.TriggerType][]*model.Reaction{
			model.TriggerEffectOnExpire: {expireReaction},
		},
	}
	actor.Effects[model.EffectFatigue] = expiringEffect

	impact := model.NewActorImpact(actor)
	impact.EffectsToRemove[expiringEffect] = true

	resolver.ApplyImpact(impact, rng)

	if _, exists := actor.Effects[model.EffectFatigue]; exists {
		t.Errorf("Expected EffectFatigue to be removed")
	}
	if _, exists := actor.Effects[model.EffectSleep]; !exists {
		t.Errorf("Expected EffectSleep to be applied via TriggerEffectOnExpire proc")
	}
}

func TestApplyImpactOnExpireProcZeroChanceDoesNotFire(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	expireReaction := &model.Reaction{
		Trigger: model.TriggerEffectOnExpire,
		Target:  model.TargetSource,
		Chance:  0, // never fires
		EffectsToApply: map[model.EffectType]*model.Effect{
			model.EffectSleep: model.NewSleepEffect(2),
		},
	}

	expiringEffect := &model.Effect{
		Kind:     model.EffectFatigue,
		Duration: -1,
		Charges:  -1,
		Procs: map[model.TriggerType][]*model.Reaction{
			model.TriggerEffectOnExpire: {expireReaction},
		},
	}
	actor.Effects[model.EffectFatigue] = expiringEffect

	impact := model.NewActorImpact(actor)
	impact.EffectsToRemove[expiringEffect] = true

	resolver.ApplyImpact(impact, rng)

	if _, exists := actor.Effects[model.EffectSleep]; exists {
		t.Errorf("Expected EffectSleep NOT to be applied when OnExpire proc has 0 chance")
	}
}

//
//
// --- TICK EFFECTS ---
//
//

//
// -- Duration ticking --
//

func TestTickEffectsTempEffectDurationDecremented(t *testing.T) {
	play, actor := setupImpactPlayer()
	ctx := model.NewSessionContext(play)
	resolver := newImpactResolver()

	eff := &model.Effect{Kind: model.EffectFatigue, Duration: 3, Charges: -1}
	actor.Effects[model.EffectFatigue] = eff

	resolver.TickEffects(*ctx, actor)

	got, exists := actor.Effects[model.EffectFatigue]
	if !exists {
		t.Fatalf("Expected EffectFatigue to still be present")
	}
	if got.Duration != 2 {
		t.Errorf("Expected Duration=2 after one tick, got %d", got.Duration)
	}
}

func TestTickEffectsEffectWithDuration1IsRemovedAfterTick(t *testing.T) {
	play, actor := setupImpactPlayer()
	ctx := model.NewSessionContext(play)
	resolver := newImpactResolver()

	eff := &model.Effect{Kind: model.EffectFatigue, Duration: 1, Charges: -1}
	actor.Effects[model.EffectFatigue] = eff

	resolver.TickEffects(*ctx, actor)

	if _, exists := actor.Effects[model.EffectFatigue]; exists {
		t.Errorf("Expected EffectFatigue to be removed after Duration reached 0")
	}
}

func TestTickEffectsPermanentEffectNotDecremented(t *testing.T) {
	play, actor := setupImpactPlayer()
	ctx := model.NewSessionContext(play)
	resolver := newImpactResolver()

	eff := &model.Effect{Kind: model.EffectFatigue, Duration: -1, Charges: -1}
	actor.Effects[model.EffectFatigue] = eff

	resolver.TickEffects(*ctx, actor)

	got, exists := actor.Effects[model.EffectFatigue]
	if !exists {
		t.Fatalf("Expected permanent EffectFatigue to remain")
	}
	if got.Duration != -1 {
		t.Errorf("Expected Duration to stay -1 for permanent effect, got %d", got.Duration)
	}
}

//
// -- Charge consumption on turn --
//

func TestTickEffectsConsumeOnTurnDecrementsCharges(t *testing.T) {
	play, actor := setupImpactPlayer()
	ctx := model.NewSessionContext(play)
	resolver := newImpactResolver()

	eff := &model.Effect{
		Kind:      model.EffectFatigue,
		Duration:  -1,
		Charges:   3,
		ConsumeOn: model.TriggerEffectOnTurn,
	}
	actor.Effects[model.EffectFatigue] = eff

	resolver.TickEffects(*ctx, actor)

	got, exists := actor.Effects[model.EffectFatigue]
	if !exists {
		t.Fatalf("Expected EffectFatigue to still be present after one tick")
	}
	if got.Charges != 2 {
		t.Errorf("Expected Charges=2 after one tick, got %d", got.Charges)
	}
}

func TestTickEffectsConsumeOnTurnRemovesEffectAtLastCharge(t *testing.T) {
	play, actor := setupImpactPlayer()
	ctx := model.NewSessionContext(play)
	resolver := newImpactResolver()

	eff := &model.Effect{
		Kind:      model.EffectFatigue,
		Duration:  -1,
		Charges:   1,
		ConsumeOn: model.TriggerEffectOnTurn,
	}
	actor.Effects[model.EffectFatigue] = eff

	resolver.TickEffects(*ctx, actor)

	if _, exists := actor.Effects[model.EffectFatigue]; exists {
		t.Errorf("Expected EffectFatigue to be removed when last charge is consumed on turn")
	}
}

//
// -- Actor death during tick --
//

func TestTickEffectsActorKilledWhenHPDropsToZero(t *testing.T) {
	ctx, monster := setupImpactMonsterWithMap()
	resolver := newImpactResolver()

	// Effect fires TriggerEffectOnTurn and drains HP completely.
	hpDrainReaction := &model.Reaction{
		Trigger: model.TriggerEffectOnTurn,
		Target:  model.TargetSource,
		Chance:  model.Guaranteed,
		VitalsChange: map[model.VitalType]model.Change{
			model.VitalHP: {
				Holder: model.TargetSource,
				Vital:  model.VitalHP,
				Amount: 0,
				Scale:  -1.0, // drain 100% of current HP
			},
		},
	}

	poison := &model.Effect{
		Kind:     model.EffectFatigue,
		Duration: -1,
		Charges:  -1,
		Procs: map[model.TriggerType][]*model.Reaction{
			model.TriggerEffectOnTurn: {hpDrainReaction},
		},
	}
	monster.Effects[model.EffectFatigue] = poison

	resolver.TickEffects(*ctx, monster)

	if monster.Vitals[model.VitalHP] > 0 {
		t.Errorf("Expected monster HP to be <= 0 after full drain, got %d", monster.Vitals[model.VitalHP])
	}
	if _, exists := ctx.Playthrough.Monsters[monster.Id]; exists {
		t.Errorf("Expected dead monster to be removed from Monsters map")
	}
	if _, exists := ctx.Playthrough.DeadMonsters[monster.Id]; !exists {
		t.Errorf("Expected dead monster to appear in DeadMonsters map")
	}
}

//
// --- RESOLVE REACTIONS ---
//

//
// -- Chance check --
//

func TestResolveReactionsGuaranteedChanceAlwaysFires(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	subj := model.NewActorImpact(actor)
	subj.Actor.Effects = make(map[model.EffectType]*model.Effect)

	reaction := &model.Reaction{
		Trigger: model.TriggerOnHit,
		Target:  model.TargetSource,
		Chance:  model.Guaranteed,
		EffectsToApply: map[model.EffectType]*model.Effect{
			model.EffectSleep: model.NewSleepEffect(1),
		},
	}

	resolver.ResolveSpecificReactions([]*model.Reaction{reaction}, subj, nil, rng)

	if _, exists := subj.AppliedEffects[model.EffectSleep]; !exists {
		t.Errorf("Expected guaranteed reaction to fire and add EffectSleep to AppliedEffects")
	}
}

func TestResolveReactionsZeroChanceNeverFires(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	subj := model.NewActorImpact(actor)

	reaction := &model.Reaction{
		Trigger: model.TriggerOnHit,
		Target:  model.TargetSource,
		Chance:  0,
		EffectsToApply: map[model.EffectType]*model.Effect{
			model.EffectSleep: model.NewSleepEffect(1),
		},
	}

	resolver.ResolveSpecificReactions([]*model.Reaction{reaction}, subj, nil, rng)

	if _, exists := subj.AppliedEffects[model.EffectSleep]; exists {
		t.Errorf("Expected zero-chance reaction NOT to fire")
	}
}

//
// -- Target routing --
//

func TestResolveReactionsTargetSourceAppliesToSubject(t *testing.T) {
	_, subjActor := setupImpactPlayer()

	objPlay, objActor := setupImpactPlayer()
	objActor.Id = 99
	objPlay.Players[objActor.Id] = objActor

	resolver := newImpactResolver()
	rng := newDeterministicRng()

	subj := model.NewActorImpact(subjActor)
	obj := model.NewActorImpact(objActor)

	reaction := &model.Reaction{
		Target:         model.TargetSource,
		Chance:         model.Guaranteed,
		StatusesChange: map[model.StatusType]int{model.StatusSleep: 1},
	}

	resolver.ResolveSpecificReactions([]*model.Reaction{reaction}, subj, obj, rng)

	if subj.StatusesChange[model.StatusSleep] != 1 {
		t.Errorf("Expected TargetSource reaction to set StatusSleep=1 on subject, got %d", subj.StatusesChange[model.StatusSleep])
	}
	if obj.StatusesChange[model.StatusSleep] != 0 {
		t.Errorf("Expected TargetSource reaction NOT to affect object, got %d", obj.StatusesChange[model.StatusSleep])
	}
}

func TestResolveReactionsTargetOpponentAppliesToObject(t *testing.T) {
	_, subjActor := setupImpactPlayer()
	_, objActor := setupImpactPlayer()
	objActor.Id = 99

	resolver := newImpactResolver()
	rng := newDeterministicRng()

	subj := model.NewActorImpact(subjActor)
	obj := model.NewActorImpact(objActor)

	reaction := &model.Reaction{
		Target:         model.TargetOpponent,
		Chance:         model.Guaranteed,
		StatusesChange: map[model.StatusType]int{model.StatusSleep: 1},
	}

	resolver.ResolveSpecificReactions([]*model.Reaction{reaction}, subj, obj, rng)

	if obj.StatusesChange[model.StatusSleep] != 1 {
		t.Errorf("Expected TargetOpponent reaction to set StatusSleep=1 on object, got %d", obj.StatusesChange[model.StatusSleep])
	}
	if subj.StatusesChange[model.StatusSleep] != 0 {
		t.Errorf("Expected TargetOpponent reaction NOT to affect subject, got %d", subj.StatusesChange[model.StatusSleep])
	}
}

func TestResolveReactionsTargetOpponentWithNilObjectSkipped(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	subj := model.NewActorImpact(actor)

	reaction := &model.Reaction{
		Target:         model.TargetOpponent,
		Chance:         model.Guaranteed,
		StatusesChange: map[model.StatusType]int{model.StatusSleep: 1},
	}

	// Must not panic when obj is nil.
	resolver.ResolveSpecificReactions([]*model.Reaction{reaction}, subj, nil, rng)

	if subj.StatusesChange[model.StatusSleep] != 0 {
		t.Errorf("Expected no change on subject when object is nil, got %d", subj.StatusesChange[model.StatusSleep])
	}
}

//
// --- ADDITIONAL COVERAGE ---
//

//
// -- TickEffects: duration edge cases --
//

// TickEffects decrements duration before checking expiry.
// An effect that enters TickEffects with Duration=0 is already logically expired.
// The implementation calls ExtendDuration(-1) first, which changes 0 to -1,
// making the effect permanent. IsExpired() then returns false and the effect
// is never removed.
func TestTickEffectsDurationZeroAtStartBecomesPermanent(t *testing.T) {
	play, actor := setupImpactPlayer()
	ctx := model.NewSessionContext(play)
	resolver := newImpactResolver()

	eff := &model.Effect{Kind: model.EffectFatigue, Duration: 0, Charges: -1}
	actor.Effects[model.EffectFatigue] = eff

	resolver.TickEffects(*ctx, actor)

	if _, exists := actor.Effects[model.EffectFatigue]; exists {
		t.Errorf("Expected pre-expired effect (Duration=0) to be removed by TickEffects, but it survived")
	}
}

// When multiple effects are present, TickEffects must remove only the one that
// has expired and leave others intact with decremented duration.
func TestTickEffectsMultipleEffectsOnlyExpiredOnesRemoved(t *testing.T) {
	play, actor := setupImpactPlayer()
	ctx := model.NewSessionContext(play)
	resolver := newImpactResolver()

	effExpiring := &model.Effect{Kind: model.EffectFatigue, Duration: 1, Charges: -1}
	effPermanent := &model.Effect{Kind: model.EffectSleep, Duration: -1, Charges: -1}
	effLasting := &model.Effect{Kind: model.EffectUntouchable, Duration: 5, Charges: -1}

	actor.Effects[model.EffectFatigue] = effExpiring
	actor.Effects[model.EffectSleep] = effPermanent
	actor.Effects[model.EffectUntouchable] = effLasting

	resolver.TickEffects(*ctx, actor)

	if _, exists := actor.Effects[model.EffectFatigue]; exists {
		t.Errorf("Expected EffectFatigue (Duration=1) to be removed after tick")
	}
	if _, exists := actor.Effects[model.EffectSleep]; !exists {
		t.Errorf("Expected permanent EffectSleep to remain")
	}
	got, exists := actor.Effects[model.EffectUntouchable]
	if !exists {
		t.Errorf("Expected EffectUntouchable to remain after one tick")
	} else if got.Duration != 4 {
		t.Errorf("Expected EffectUntouchable Duration=4, got %d", got.Duration)
	}
}

// An effect that has both a finite Duration and ConsumeOn=TriggerEffectOnTurn
// must have both decremented in the same tick.
func TestTickEffectsEffectWithDurationAndConsumeOnTurnBothDecrement(t *testing.T) {
	play, actor := setupImpactPlayer()
	ctx := model.NewSessionContext(play)
	resolver := newImpactResolver()

	eff := &model.Effect{
		Kind:      model.EffectFatigue,
		Duration:  3,
		Charges:   3,
		ConsumeOn: model.TriggerEffectOnTurn,
	}
	actor.Effects[model.EffectFatigue] = eff

	resolver.TickEffects(*ctx, actor)

	got, exists := actor.Effects[model.EffectFatigue]
	if !exists {
		t.Fatalf("Expected EffectFatigue to still be present after one tick")
	}
	if got.Duration != 2 {
		t.Errorf("Expected Duration=2 after one tick, got %d", got.Duration)
	}
	if got.Charges != 2 {
		t.Errorf("Expected Charges=2 after one tick on ConsumeOnTurn, got %d", got.Charges)
	}
}

// TriggerEffectOnExpire must fire when an effect reaches Duration=0 through
// the natural tick cycle, not only when manually marked for removal.
func TestTickEffectsOnExpireFiresWhenEffectNaturallyExpires(t *testing.T) {
	play, actor := setupImpactPlayer()
	ctx := model.NewSessionContext(play)
	resolver := newImpactResolver()

	expireReaction := &model.Reaction{
		Trigger: model.TriggerEffectOnExpire,
		Target:  model.TargetSource,
		Chance:  model.Guaranteed,
		EffectsToApply: map[model.EffectType]*model.Effect{
			model.EffectSleep: model.NewSleepEffect(3),
		},
	}

	eff := &model.Effect{
		Kind:     model.EffectFatigue,
		Duration: 1,
		Charges:  -1,
		Procs: map[model.TriggerType][]*model.Reaction{
			model.TriggerEffectOnExpire: {expireReaction},
		},
	}
	actor.Effects[model.EffectFatigue] = eff

	resolver.TickEffects(*ctx, actor)

	if _, exists := actor.Effects[model.EffectFatigue]; exists {
		t.Errorf("Expected EffectFatigue to be removed after Duration reached 0")
	}
	if _, exists := actor.Effects[model.EffectSleep]; !exists {
		t.Errorf("Expected OnExpire proc to fire when effect expires via tick, applying EffectSleep")
	}
}

//
// -- ApplyImpact: charges boundary cases --
//

// Positive EffectChargesChange means charges are restored (e.g. recharge ability).
// It must NOT trigger removal even if the pre-change value was low.
func TestApplyImpactPositiveChargesChangeDoesNotTriggerRemoval(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	eff := &model.Effect{Kind: model.EffectFatigue, Duration: -1, Charges: 2}
	actor.Effects[model.EffectFatigue] = eff

	impact := model.NewActorImpact(actor)
	impact.EffectChargesChange[eff] = +3

	resolver.ApplyImpact(impact, rng)

	got, exists := actor.Effects[model.EffectFatigue]
	if !exists {
		t.Fatalf("Expected EffectFatigue to remain after positive charge change")
	}
	if got.Charges != 5 {
		t.Errorf("Expected Charges=5 after adding 3 to 2, got %d", got.Charges)
	}
}

// A charge decrement larger than the current charges must still trigger removal.
// The code checks <= 0 after adding a negative value, so -10 on Charges=1 gives -9.
func TestApplyImpactChargesDecrementedBeyondZeroStillTriggersRemoval(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	eff := &model.Effect{Kind: model.EffectFatigue, Duration: -1, Charges: 1}
	actor.Effects[model.EffectFatigue] = eff

	impact := model.NewActorImpact(actor)
	impact.EffectChargesChange[eff] = -10

	resolver.ApplyImpact(impact, rng)

	if _, exists := actor.Effects[model.EffectFatigue]; exists {
		t.Errorf("Expected EffectFatigue to be removed when charges drop far below 0 (1 - 10 = -9)")
	}
}

//
// -- ResolveReactions: multiple reactions --
//

// When multiple reactions are present and all have Guaranteed chance, every
// one of them must fire. A common implementation mistake is breaking out of
// the loop after the first match.
func TestResolveReactionsMultipleGuaranteedReactionsAllFire(t *testing.T) {
	_, actor := setupImpactPlayer()
	resolver := newImpactResolver()
	rng := newDeterministicRng()

	subj := model.NewActorImpact(actor)

	reactions := []*model.Reaction{
		{
			Target: model.TargetSource,
			Chance: model.Guaranteed,
			EffectsToApply: map[model.EffectType]*model.Effect{
				model.EffectSleep: model.NewSleepEffect(1),
			},
		},
		{
			Target:         model.TargetSource,
			Chance:         model.Guaranteed,
			StatusesChange: map[model.StatusType]int{model.StatusFatigue: 1},
		},
		{
			Target: model.TargetSource,
			Chance: model.Guaranteed,
			EffectsToApply: map[model.EffectType]*model.Effect{
				model.EffectInfallible: model.NewInfallibleEffect(2, 2),
			},
		},
	}

	resolver.ResolveSpecificReactions(reactions, subj, nil, rng)

	if _, exists := subj.AppliedEffects[model.EffectSleep]; !exists {
		t.Errorf("Expected reaction 1 (EffectSleep) to fire")
	}
	if subj.StatusesChange[model.StatusFatigue] != 1 {
		t.Errorf("Expected reaction 2 (StatusFatigue=1) to fire, got %d", subj.StatusesChange[model.StatusFatigue])
	}
	if _, exists := subj.AppliedEffects[model.EffectInfallible]; !exists {
		t.Errorf("Expected reaction 3 (EffectInfallible) to fire")
	}
}
