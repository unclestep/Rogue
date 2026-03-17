package service_test

import (
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
)

func TestDungeonGeneratorGen(t *testing.T) {
	rules := model.NewDefaultGameRules()
	topGen := service.NewTopologyGenerator()
	objGen := service.NewObjectSpawner(rules)
	doorLock := service.NewDoorLocker()

	t.Run("Complete Generation Flow", func(t *testing.T) {
		seed := time.Now().UnixNano()
		ctx := createTestContext(seed)
		generator := service.NewDungeonGeneratorService(rules, topGen, objGen, doorLock)

		generator.Gen(ctx)
		m := ctx.Playthrough.Map

		if m == nil || len(m.GetRooms()) == 0 {
			t.Errorf("Seed %v: Map and rooms should be generated", seed)
		}

		if m.GetEntranceRoom() == nil || m.GetExitRoom() == nil {
			t.Errorf("Seed %v: Entrance and Exit rooms should be defined", seed)
		}

		if len(ctx.Playthrough.Items) == 0 {
			t.Errorf("Seed %v: Items should be spawned", seed)
		}

		if len(ctx.Playthrough.Monsters) == 0 {
			t.Errorf("Seed %v: Monsters should be spawned", seed)
		}
	})
}
