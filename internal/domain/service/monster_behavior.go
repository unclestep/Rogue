// package service
//
// import (
// 	"math/rand"
//
// 	"github.com/unclestep/Rogue/internal/domain/model"
// 	"github.com/unclestep/Rogue/internal/domain/service"
// 	"github.com/unclestep/Rogue/pkg/geometry"
// )
//
// type MonsterBehaviorContext struct {
// 	Session *model.GameSession
// 	Move    *service.Move
// 	Seed    int64
// 	Rng     *rand.Rand
// }
//
// func NewBehaviorContext(session *model.GameSession, seed int64) *MonsterBehaviorContext {
// 	return &MonsterBehaviorContext{
// 		Session: session,
// 		Move:    service.NewMoveService(session, seed),
// 		Seed:    seed,
// 		Rng:     rand.New(rand.NewSource(seed)),
// 	}
// }
//
// func (ctx *MonsterBehaviorContext) SetSeed(seed int64) {
// 	ctx.Seed = seed
// 	ctx.Rng = rand.New(rand.NewSource(ctx.Seed))
// 	ctx.Move.SetSeed(seed)
// }
//
// type MonsterBehavior interface {
// 	Update(actor *model.Actor, ctx *MonsterBehaviorContext) (model.ActorStateType, []service.Event)
// 	Clone() MonsterBehavior
// }
//
// type WanderBehavior struct {
// 	RoomCenters           *ScentMap
// 	RoomEdges             *ScentMap
// 	ReverseScentMapChance int
// 	WanderToCenter        bool
// }
//
// func NewWanderBehavior(roomCenters, roomEdges *ScentMap) *WanderBehavior {
// 	return &WanderBehavior{
// 		RoomCenters: roomCenters,
// 		RoomEdges:   roomEdges,
// 	}
// }
//
// func (wander *WanderBehavior) Clone() MonsterBehavior {
// 	return &WanderBehavior{
// 		RoomCenters:           wander.RoomCenters,
// 		RoomEdges:             wander.RoomEdges,
// 		ReverseScentMapChance: wander.ReverseScentMapChance,
// 		WanderToCenter:        wander.WanderToCenter,
// 	}
// }
//
// func (wander *WanderBehavior) Update(monster *model.Actor, ctx *MonsterBehaviorContext) (model.ActorStateType, []service.Event) {
// 	if CheckHostility(ctx, func(playerPos geometry.Point) bool {
// 		return monster.Pos.EuclideanDistance(playerPos) <= float64(monster.DerivedAttrs[model.AttrHostility])
// 	}) {
// 		return model.ActorStateChase, nil
// 	}
//
// 	scentMap := wander.GetCurScentMap(ctx)
//
// 	return model.ActorStateWander, ctx.Move.ExecuteMove(monster, scentMap, ctx.Session.Map)
// }
//
// func CheckHostility(ctx *MonsterBehaviorContext, cond func(playerPos geometry.Point) bool) bool {
// 	for _, player := range ctx.Session.Players {
// 		if cond(player.Pos) {
// 			return true
// 		}
// 	}
// 	return false
// }
//
// func (wander *WanderBehavior) GetCurScentMap(ctx *MonsterBehaviorContext) [][]int {
// 	wander.ReverseScentMapChance += 5
//
// 	if ctx.Rng.Intn(model.Guaranteed) < wander.ReverseScentMapChance {
// 		wander.WanderToCenter = !wander.WanderToCenter
// 		wander.ReverseScentMapChance = 0
// 	}
//
// 	if wander.WanderToCenter {
// 		return wander.RoomCenters.Scent
// 	}
//
// 	return wander.RoomEdges.Scent
// }
//
// type ChaseBehavior struct {
// 	Player *ScentMap
// }
//
// func NewChaseBehavior(player *ScentMap) *ChaseBehavior {
// 	return &ChaseBehavior{
// 		Player: player,
// 	}
// }
//
// func (chase *ChaseBehavior) Clone() MonsterBehavior {
// 	return &ChaseBehavior{
// 		Player: chase.Player,
// 	}
// }
//
// func (chase *ChaseBehavior) Update(monster *model.Actor, ctx *MonsterBehaviorContext) (model.ActorStateType, []service.Event) {
// 	if CheckHostility(ctx, func(playerPos geometry.Point) bool {
// 		return monster.Pos.EuclideanDistance(playerPos) >= 1.5*float64(monster.DerivedAttrs[model.AttrHostility])
// 	}) {
// 		return model.ActorStateWander, nil
// 	}
//
// 	return model.ActorStateWander, ctx.Move.ExecuteMove(monster, chase.Player.Scent, ctx.Session.Map)
// }
