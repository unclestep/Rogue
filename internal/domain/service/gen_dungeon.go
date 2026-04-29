package service

import (
	"slices"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/algorithm"
	"github.com/unclestep/Rogue/pkg/conv"
)

type DungeonGenerator struct {
	rules             *model.GameRules
	topologyGenerator *TopologyGenerator
	objectSpawner     *ObjectSpawner
	doorLocker        *DoorLocker
}

func NewDungeonGeneratorService(rules *model.GameRules, topGen *TopologyGenerator, objGen *ObjectSpawner, doorLock *DoorLocker) *DungeonGenerator {
	return &DungeonGenerator{
		rules:             rules,
		topologyGenerator: topGen,
		objectSpawner:     objGen,
		doorLocker:        doorLock,
	}
}

func (d *DungeonGenerator) Gen(ctx *model.SessionContext) {
	genParams := d.prepareNextDung(ctx.Playthrough)
	ctx.Playthrough.DungParams = genParams
	ctx.Playthrough.Map = nil

	d.topologyGenerator.Gen(ctx, d.rules.DungeonWidth, d.rules.DungeonHeight, d.rules.MaxHorizontalRoomCount, d.rules.MaxVerticalRoomCount, d.rules.ExtraDungeonConnections)

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

	d.ShowPlayers(ctx)
}

func (d *DungeonGenerator) prepareNextDung(play *model.Playthrough) *model.DungParams {
	d.clearPrevDung(play)
	play.Depth++

	genParams := d.rules.DiffCurve.At(play.Depth, d.rules.MaxDungeonCount, play.DynamicDifficulty)
	updateItemWeights, updateActorWeights := play.AdaptDifficulty()
	for label, change := range updateItemWeights {
		genParams.ItemWeights[label] += change
	}
	for label, change := range updateActorWeights {
		genParams.MonsterWeights[label] += change
	}

	play.ClearMetrics()

	return genParams
}

func (d *DungeonGenerator) clearPrevDung(play *model.Playthrough) {
	clear(play.Monsters)
	clear(play.DeadMonsters)
	clear(play.Items)
	play.PendingIntents = play.PendingIntents[:0]
	play.TurnEvents = play.TurnEvents[:0]
}

// ShowPlayers places hidden-but-alive players onto the entrance room.
// Called during Gen() for a new dungeon level and when resuming a saved game.
func (d *DungeonGenerator) ShowPlayers(ctx *model.SessionContext) {
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
