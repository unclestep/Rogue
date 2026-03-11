// package service
//
// import (
// 	"log"
// 	"math/rand"
//
// 	"github.com/unclestep/Rogue/internal/domain/model"
// 	"github.com/unclestep/Rogue/pkg/algorithm"
// 	"github.com/unclestep/Rogue/pkg/conv"
// 	"github.com/unclestep/Rogue/pkg/geometry"
// )
//
// // ObjectSpawner - manages the placement of items and actors across the dungeon map.
// type ObjectSpawner struct {
// 	GameRules *model.GameRules
// }
//
// // PrimaryPoolMultiplier - determines the initial fraction of room capacity used for spawning.
// // This helps distribute objects more evenly across the map before filling rooms to capacity.
// const PrimaryPoolMultiplier = 0.5
//
// // GenerateItems - spawns items in all rooms except the entrance.
// // If n is greater than the total available points, it places as many items as possible.
// func (o *ObjectSpawner) GenerateItems(session *model.GameSession, n int, weights []conv.KV[model.ItemLabel, int], rng *rand.Rand) {
// 	o.generateObjects(session, n,
// 		func(r *model.Room) int { return len(r.EmptyItemPoints) },
// 		session.Map.TakeRandomItemPoint,
// 		func(pos geometry.Point, rng *rand.Rand) {
// 			itemLabel := getWeightedResult(weights, rng)
// 			item := o.GameRules.ItemsConf[itemLabel].Clone()
//
// 			id := session.GetId()
// 			item.Id = model.ItemId(id)
// 			item.Pos = pos
//
// 			session.AddItem(item)
// 		},
// 		rng,
// 	)
// }
//
// // GenerateMonsters - spawns monsters in all rooms except the entrance.
// // If n is greater than the total available points, it places as many actors as possible.
// func (o *ObjectSpawner) GenerateMonsters(session *model.GameSession, n int, weights []conv.KV[model.ActorLabel, int], rng *rand.Rand) {
// 	o.generateObjects(session, n,
// 		func(r *model.Room) int { return len(r.EmptyActorPoints) },
// 		session.Map.TakeRandomActorPoint,
// 		func(pos geometry.Point, rng *rand.Rand) {
// 			actorLabel := getWeightedResult(weights, rng)
// 			monster := o.GameRules.ActorsConf[actorLabel].Clone()
//
// 			id := session.GetId()
// 			monster.Id = model.ActorId(id)
// 			monster.Pos = pos
//
// 			session.AddMonster(monster)
// 		},
// 		rng,
// 	)
// }
//
// // generateObjects - a generic algorithm that distributes objects across the map in two phases:
// // 1. Fill rooms up to the PrimaryPoolMultiplier to ensure even distribution.
// // 2. Fill the remaining requested objects using the reserve capacity.
// func (o *ObjectSpawner) generateObjects(session *model.GameSession, n int,
// 	getRoomCapacity func(*model.Room) int,
// 	getRandomPoint func(*model.Room, *rand.Rand) (geometry.Point, bool),
// 	createObj func(geometry.Point, *rand.Rand),
// 	rng *rand.Rand,
// ) {
// 	if n <= 0 {
// 		log.Printf("[INFO] No objects generated: n=%v\n", n)
// 		return
// 	}
//
// 	m := session.Map
//
// 	primaryRoomList := make([]*model.Room, 0, len(m.Rooms))
// 	availableRoomsPrime := make(map[*model.Room]int, len(m.Rooms))
//
// 	resRoomList := make([]*model.Room, 0, len(m.Rooms))
// 	availableRoomsRes := make(map[*model.Room]int, len(m.Rooms))
//
// 	// Categorize room capacities into primary and reserve pools
// 	for _, room := range m.Rooms {
// 		if room.Id == m.EntranceRoomId {
// 			continue
// 		}
//
// 		total := getRoomCapacity(room)
// 		if total == 0 {
// 			continue
// 		}
//
// 		primePoints := int(float64(total) * PrimaryPoolMultiplier)
// 		reservePoints := total - primePoints
//
// 		if primePoints > 0 {
// 			primaryRoomList = append(primaryRoomList, room)
// 			availableRoomsPrime[room] = primePoints
// 		}
//
// 		if reservePoints > 0 {
// 			resRoomList = append(resRoomList, room)
// 			availableRoomsRes[room] = reservePoints
// 		}
// 	}
//
// 	spawned := 0
// 	spawn := func(roomList *[]*model.Room, availPoints map[*model.Room]int) {
// 		if len(*roomList) == 0 {
// 			return
// 		}
//
// 		idx := rng.Intn(len(*roomList))
// 		room := (*roomList)[idx]
//
// 		point, ok := getRandomPoint(room, rng)
// 		if !ok {
// 			// Room is actually full or point is invalid
// 			*roomList = algorithm.Remove(*roomList, idx)
// 			delete(availPoints, room)
// 			return
// 		}
//
// 		createObj(point, rng)
//
// 		spawned++
// 		availPoints[room]--
//
// 		// Remove room from current pool once its allocated points are exhausted
// 		if availPoints[room] <= 0 {
// 			*roomList = algorithm.Remove(*roomList, idx)
// 			delete(availPoints, room)
// 		}
// 	}
//
// 	// Phase 1: Distributed spawning using the primary pool
// 	for len(availableRoomsPrime) > 0 && spawned < n {
// 		spawn(&primaryRoomList, availableRoomsPrime)
// 	}
//
// 	// Phase 2: Fill remaining requested objects using the reserve pool
// 	for len(availableRoomsRes) > 0 && spawned < n {
// 		spawn(&resRoomList, availableRoomsRes)
// 	}
//
// 	if spawned < n {
// 		log.Printf("[INFO] Could only spawn %d/%d objects: map is full", spawned, n)
// 	}
// }
//
// func getWeightedResult[K interface {
// 	model.ActorLabel | model.ItemLabel
// }, V ~int](weights []conv.KV[K, V], rng *rand.Rand) K {
// 	var totalWeight V = 0
// 	for _, w := range weights {
// 		totalWeight += w.Val
// 	}
//
// 	r := V(rng.Intn(int(totalWeight)))
//
// 	for _, weight := range weights {
// 		if r < weight.Val {
// 			return weight.Key
// 		}
// 		r -= weight.Val
// 	}
//
// 	return weights[0].Key
// }
//
