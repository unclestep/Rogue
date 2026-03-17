package service

import (
	"math/rand"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/algorithm"
	"github.com/unclestep/Rogue/pkg/conv"
	"github.com/unclestep/Rogue/pkg/geometry"
)

// ObjectSpawner - manages the placement of items and actors across the dungeon map.
type ObjectSpawner struct {
	rules *model.GameRules
}

func NewObjectSpawner(gameRules *model.GameRules) *ObjectSpawner {
	return &ObjectSpawner{
		rules: gameRules,
	}
}

const (
	// PrimaryPoolMultiplier - determines the initial fraction of room capacity used for spawning.
	// This helps distribute objects more evenly across the map before filling rooms to capacity.
	PrimaryPoolMultiplier = 0.5
	PlaceForLootSearchRad = -1
)

func (o *ObjectSpawner) SpawnTreasure(ctx *model.SessionContext, deadActor *model.Actor) {
	itemPos, ok := ctx.Playthrough.Map.FindEmptyPoint(deadActor.Pos, PlaceForLootSearchRad)
	if ok {
		multiplier := ctx.Playthrough.DungParams.TreasureValueMultiplier
		value := o.calcTreasureValue(deadActor, multiplier)
		id := ctx.Playthrough.GetId()

		treasure := model.NewTreasureItem(model.ItemId(id), itemPos, value)
		ctx.Playthrough.AddItem(treasure)
	}
}

func (o *ObjectSpawner) calcTreasureValue(deadActor *model.Actor, multiplier float64) int {
	value := 100

	value += deadActor.DerivedAttrs[model.AttrMaxHP]
	value += deadActor.DerivedAttrs[model.AttrMaxStamina]
	value += deadActor.DerivedAttrs[model.AttrStrength] * 2
	value += deadActor.DerivedAttrs[model.AttrDexterity] * 3
	value += deadActor.DerivedAttrs[model.AttrHostility] * 100

	return int(float64(value) * multiplier)
}

func (o *ObjectSpawner) CreatePlayer(playthrough *model.Playthrough, playerUuid string, actorLabel model.ActorLabel) {
	player := o.rules.ActorsConf[actorLabel].Clone()
	player.Id = model.ActorId(playthrough.GetId())
	player.Pos = model.NewInvalidPoint()
	if len(playthrough.PlayersUuid) == 0 {
		playthrough.HostId = player.Id
	}
	playthrough.PlayersUuid[playerUuid] = player.Id
}

// GenerateItems - spawns items in all rooms except the entrance.
// If n is greater than the total available points, it places as many items as possible.
func (o *ObjectSpawner) GenerateItems(ctx *model.SessionContext, n int, weights []conv.KV[model.ItemLabel, int]) {
	o.generateObjects(ctx, n,
		func(r *model.Room) int { return r.GetItemCapacity() },
		ctx.Playthrough.Map.TakeRandomItemPoint,
		func(pos geometry.Point, rng *rand.Rand) {
			itemLabel := getWeightedResult(weights, rng)
			item := o.rules.ItemsConf[itemLabel].Clone()

			id := ctx.Playthrough.GetId()
			item.Id = model.ItemId(id)
			item.Pos = pos

			ctx.Playthrough.AddItem(item)
		},
	)
}

// GenerateMonsters - spawns monsters in all rooms except the entrance.
// If n is greater than the total available points, it places as many actors as possible.
func (o *ObjectSpawner) GenerateMonsters(ctx *model.SessionContext, n int, weights []conv.KV[model.ActorLabel, int]) {
	o.generateObjects(ctx, n,
		func(r *model.Room) int { return r.GetActorCapacity() },
		ctx.Playthrough.Map.TakeRandomActorPoint,
		func(pos geometry.Point, rng *rand.Rand) {
			actorLabel := getWeightedResult(weights, rng)
			monster := o.rules.ActorsConf[actorLabel].Clone()

			id := ctx.Playthrough.GetId()
			monster.Id = model.ActorId(id)
			monster.Pos = pos

			ctx.Playthrough.AddMonster(monster)
		},
	)
}

// generateObjects - a generic algorithm that distributes objects across the map in two phases:
// 1. Fill rooms up to the PrimaryPoolMultiplier to ensure even distribution.
// 2. Fill the remaining requested objects using the reserve capacity.
func (o *ObjectSpawner) generateObjects(ctx *model.SessionContext, n int,
	getRoomCapacity func(*model.Room) int,
	getRandomPoint func(*model.Room, *rand.Rand) (geometry.Point, bool),
	createObj func(geometry.Point, *rand.Rand),
) {
	if n <= 0 {
		return
	}

	m := ctx.Playthrough.Map

	primaryRoomList := make([]*model.Room, 0, m.GetRoomCount())
	availableRoomsPrime := make(map[*model.Room]int, m.GetRoomCount())

	resRoomList := make([]*model.Room, 0, m.GetRoomCount())
	availableRoomsRes := make(map[*model.Room]int, m.GetRoomCount())

	// Categorize room capacities into primary and reserve pools
	for _, room := range m.GetRooms() {
		if room.Id == m.EntranceRoomId {
			continue
		}

		total := getRoomCapacity(room)
		if total == 0 {
			continue
		}

		primePoints := int(float64(total) * PrimaryPoolMultiplier)
		reservePoints := total - primePoints

		if primePoints > 0 {
			primaryRoomList = append(primaryRoomList, room)
			availableRoomsPrime[room] = primePoints
		}

		if reservePoints > 0 {
			resRoomList = append(resRoomList, room)
			availableRoomsRes[room] = reservePoints
		}
	}

	spawned := 0
	spawn := func(roomList *[]*model.Room, availPoints map[*model.Room]int) {
		if len(*roomList) == 0 {
			return
		}

		idx := ctx.Rng().Intn(len(*roomList))
		room := (*roomList)[idx]

		point, ok := getRandomPoint(room, ctx.Rng())
		if !ok {
			// Room is actually full or point is invalid
			*roomList = algorithm.Remove(*roomList, idx)
			delete(availPoints, room)
			return
		}

		createObj(point, ctx.Rng())

		spawned++
		availPoints[room]--

		// Remove room from current pool once its allocated points are exhausted
		if availPoints[room] <= 0 {
			*roomList = algorithm.Remove(*roomList, idx)
			delete(availPoints, room)
		}
	}

	// Phase 1: Distributed spawning using the primary pool
	for len(availableRoomsPrime) > 0 && spawned < n {
		spawn(&primaryRoomList, availableRoomsPrime)
	}

	// Phase 2: Fill remaining requested objects using the reserve pool
	for len(availableRoomsRes) > 0 && spawned < n {
		spawn(&resRoomList, availableRoomsRes)
	}
}

func getWeightedResult[K interface {
	model.ActorLabel | model.ItemLabel
}, V ~int](weights []conv.KV[K, V], rng *rand.Rand) K {
	var totalWeight V = 0
	for _, w := range weights {
		totalWeight += w.Val
	}

	r := V(rng.Intn(int(totalWeight)))

	for _, weight := range weights {
		if r < weight.Val {
			return weight.Key
		}
		r -= weight.Val
	}

	return weights[0].Key
}
