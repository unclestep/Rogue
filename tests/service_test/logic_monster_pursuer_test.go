package service_test

import (
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/geometry"
)

// Pursuer is registered with BehaviorPursuer → ChaseBehavior during PR 1.
// This test verifies the plumbing: Tick does not panic for a Pursuer monster
// and emits an intent for it (ChaseBehavior produces a move every turn).
func TestMonsterControllerTickPursuerEmitsIntent(t *testing.T) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildMonsterTestMap()

	player := model.NewDefaultPlayer(1, geometry.Point{X: 2, Y: 2}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	monster := model.NewDefaultPursuer(2, geometry.Point{X: 6, Y: 6})
	play.Monsters[monster.Id] = monster
	play.Map.SetActor(monster.Pos, int64(monster.Id))

	ctrl := newMonsterController()
	ctx := model.NewSessionContext(play)

	done := make(chan struct{})
	var intents []*model.Intent
	go func() {
		defer close(done)
		intents = ctrl.Tick(ctx)
	}()

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("Tick hung for Pursuer — BehaviorPursuer likely unregistered in controller")
	}

	if !hasIntentForActor(intents, monster.Id) {
		t.Errorf("Expected an intent for Pursuer, got none")
	}
}

// Fresh Pursuer from NewDefaultPursuer must start in BehaviorPursuer so that
// Tick dispatches to the registered behavior instead of falling into the nil
// BehaviorIdle bucket.
func TestNewDefaultPursuerStartsInPursuerState(t *testing.T) {
	p := model.NewDefaultPursuer(1, geometry.NewDefaultPoint())
	if p.State != model.BehaviorPursuer {
		t.Errorf("Expected State=BehaviorPursuer, got %v", p.State)
	}
}

// With a ScriptedPolicy the Pursuer's outgoing intent must carry the exact
// vector dictated by the scripted action — proving Policy.Predict is wired
// through PursuerBehavior.Update to moveResolver.Resolve and that
// ActionToVector maps the five-way action space correctly.
func TestPursuerScriptedPolicyControlsMovement(t *testing.T) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildMonsterTestMap()

	player := model.NewDefaultPlayer(1, geometry.Point{X: 2, Y: 2}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	monster := model.NewDefaultPursuer(2, geometry.Point{X: 6, Y: 6})
	play.Monsters[monster.Id] = monster
	play.Map.SetActor(monster.Pos, int64(monster.Id))

	ctrl := newMonsterControllerWithPolicy(service.NewScriptedPolicy([]int{service.ActionUp}))
	ctx := model.NewSessionContext(play)

	intents := ctrl.Tick(ctx)

	var got *model.Intent
	for _, it := range intents {
		if it.Actor == monster.Id {
			got = it
			break
		}
	}
	if got == nil {
		t.Fatalf("Expected an intent for Pursuer, got none")
	}
	if got.IntentType != model.IntentMove {
		t.Errorf("Expected IntentMove, got %v", got.IntentType)
	}
	if (got.Vector != geometry.Point{X: 0, Y: -1}) {
		t.Errorf("Expected Vector=(0,-1) from ActionUp, got %v", got.Vector)
	}
}

// When a Pursuer is killed (removed from Playthrough.Monsters) the next
// Update call must prune its entry from the behavior's memory map. This
// guards against an unbounded leak across a long playthrough.
func TestPursuerBehaviorMemoryPrunedOnMonsterRemoval(t *testing.T) {
	play := model.NewPlaythrough("", 0, 0)
	play.Map = buildMonsterTestMap()

	player := model.NewDefaultPlayer(1, geometry.Point{X: 2, Y: 2}, 9)
	play.Players[player.Id] = player
	play.Map.SetActor(player.Pos, int64(player.Id))

	m1 := model.NewDefaultPursuer(10, geometry.Point{X: 5, Y: 5})
	m2 := model.NewDefaultPursuer(11, geometry.Point{X: 6, Y: 6})
	play.Monsters[m1.Id] = m1
	play.Monsters[m2.Id] = m2
	play.Map.SetActor(m1.Pos, int64(m1.Id))
	play.Map.SetActor(m2.Pos, int64(m2.Id))

	ctx := model.NewSessionContext(play)
	pb := service.NewPursuerBehavior(service.NewRaycaster(),
		service.NewScriptedPolicy([]int{service.ActionWait}))

	pb.Update(ctx, m1)
	pb.Update(ctx, m2)
	if pb.MemorySize() != 2 {
		t.Fatalf("Expected 2 memory entries before death, got %d", pb.MemorySize())
	}

	delete(play.Monsters, m1.Id)

	pb.Update(ctx, m2)
	if pb.MemorySize() != 1 {
		t.Errorf("Expected 1 memory entry after m1 removal, got %d", pb.MemorySize())
	}
}
