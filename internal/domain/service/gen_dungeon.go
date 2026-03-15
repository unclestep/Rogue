package service

import (
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/algorithm"
	"github.com/unclestep/Rogue/pkg/conv"
	"slices"
)

type DungeonGenerator struct {
	rules             *model.GameRules
	topologyGenerator *TopologyGenerator
	objectSpawner     *ObjectSpawner
	doorLocker        *DoorLocker
}

func NewDungeonGeneratorService(rules *model.GameRules) *DungeonGenerator {
	return &DungeonGenerator{
		rules:             rules,
		topologyGenerator: NewTopologyGenerator(),
		objectSpawner:     NewObjectSpawner(rules),
		doorLocker:        NewDoorLocker(),
	}
}

func (d *DungeonGenerator) Gen(ctx *model.SessionContext) {
	genParams := d.prepareNextDung(ctx.Playthrough)
	ctx.Playthrough.DungParams = genParams

	d.topologyGenerator.Gen(ctx, d.rules.DungeonWidth, d.rules.DungeonHeight, d.rules.MaxHorizontalRoomCount, d.rules.MaxVerticalRoomCount)

	if ctx.Playthrough.Depth >= genParams.LockedDoorsStartDepth {
		doorCount := genParams.MinLockedDoors + ctx.Rng().Intn(genParams.MaxLockedDoors-genParams.MinLockedDoors+1)
		d.doorLocker.LockDoors(ctx, doorCount, doorCount)
	}

	itemCount := genParams.MinItems + ctx.Rng().Intn(genParams.MaxItems-genParams.MinItems+1)
	itemWeights := conv.MapToKV(genParams.ItemWeights)
	d.objectSpawner.GenerateItems(ctx, itemCount, itemWeights)

	monsterCount := genParams.MinMonsters + ctx.Rng().Intn(genParams.MaxMonsters-genParams.MinMonsters+1)
	monsterWeights := conv.MapToKV(genParams.MonsterWeights)
	d.objectSpawner.GenerateMonsters(ctx, monsterCount, monsterWeights)

	d.showPlayers(ctx)
}

func (d *DungeonGenerator) prepareNextDung(session *model.Playthrough) *model.DungParams {
	session.Depth++

	genParams := d.rules.DiffCurve.At(session.Depth, d.rules.MaxDungeonCount, session.DynamicDifficulty)
	updateItemWeights, updateActorWeights := session.AdaptDifficulty()
	for label, change := range updateItemWeights {
		genParams.ItemWeights[label] += change
	}
	for label, change := range updateActorWeights {
		genParams.MonsterWeights[label] += change
	}

	session.ClearMetrics()

	return genParams
}

// ShowPlayers - shows all players in new dungeon level.
// Use this function when move players to the new dungeon level.
func (d *DungeonGenerator) showPlayers(ctx *model.SessionContext) {
	m := ctx.Playthrough.Map
	rooms := slices.Clone(m.GetRooms())
	i := 0

	for r, room := range rooms {
		if room.Id == m.EntranceRoomId {
			i = r
			break
		}
	}

	for _, player := range ctx.Playthrough.Players {
		if player.Pos == model.NewInvalidPoint() {
			for len(rooms) > 0 {
				room := rooms[i]
				p, ok := m.TakeRandomActorPoint(room, ctx.Rng())

				if ok {
					player.Pos = p
					m.SetActor(p, int64(player.Id))
					if player.Vitals[model.VitalHP] <= 0 {
						player.Vitals[model.VitalHP] = int(d.rules.HpRestore * float64(player.BaseAttrs[model.AttrMaxHP]))
					}
					break
				}

				rooms = algorithm.Remove(rooms, i)
				i = 0
			}
		}
	}
}
