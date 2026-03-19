package service_test

import (
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
)

func createDungGenEnv(seed int64) (*model.SessionContext, *service.DungeonGenerator) {
	ctx := createTestContext(seed)
	rules := model.NewDefaultGameRules()
	topGen := service.NewTopologyGenerator()
	objGen := service.NewObjectSpawner(rules)
	doorLock := service.NewDoorLocker()
	generator := service.NewDungeonGeneratorService(rules, topGen, objGen, doorLock)

	player1 := model.NewDefaultPlayer(1, model.NewInvalidPoint(), 9)
	player2 := model.NewDefaultPlayer(2, model.NewInvalidPoint(), 9)
	player2.Vitals[model.VitalHP] = 0
	player3 := model.NewDefaultPlayer(3, model.NewInvalidPoint(), 9)
	player3.Vitals[model.VitalHP] = -80

	ctx.Playthrough.Players[player1.Id] = player1
	ctx.Playthrough.Players[player2.Id] = player2
	ctx.Playthrough.Players[player3.Id] = player3

	return ctx, generator
}

func TestDungeonGeneratorGen(t *testing.T) {
	t.Run("Complete Generation Flow", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			seed := time.Now().UnixNano() + int64(i)
			ctx, generator := createDungGenEnv(seed)

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

			for _, player := range ctx.Playthrough.Players {
				if player.Vitals[model.VitalHP] < 0 {
					t.Errorf("Seed %v: Player's HP should be restored", seed)
				}
				if player.Pos == model.NewInvalidPoint() {
					t.Errorf("Seed %v: Player's should have valid position", seed)
				}
				if !ctx.Playthrough.Map.IsActor(player.Pos) {
					t.Errorf("Seed %v: Player's should be on the map", seed)
				}
			}
		}
	})
}
