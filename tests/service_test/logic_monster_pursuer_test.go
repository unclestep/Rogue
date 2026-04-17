package service_test

import (
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/domain/model"
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
