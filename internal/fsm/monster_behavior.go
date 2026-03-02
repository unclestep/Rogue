package fsm

import (
	"github.com/unclestep/Rogue/internal/domain/entity"
	"github.com/unclestep/Rogue/internal/domain/service"
	"math/rand"
)

type MonsterBehaviorContext struct {
	Session *entity.GameSession
	Move    *service.Move
	Seed    int64
	Rng     *rand.Rand
}

func NewBehaviorContext(session *entity.GameSession, seed int64) *MonsterBehaviorContext {
	return &MonsterBehaviorContext{
		Session: session,
		Move:    service.NewMoveService(session, seed),
		Seed:    seed,
		Rng:     rand.New(rand.NewSource(seed)),
	}
}

func (ctx *MonsterBehaviorContext) SetSeed(seed int64) {
	ctx.Seed = seed
	ctx.Rng = rand.New(rand.NewSource(ctx.Seed))
	ctx.Move.SetSeed(seed)
}

type MonsterBehavior interface {
	Update(actor *entity.Actor, ctx *MonsterBehaviorContext) (entity.ActorStateType, []service.Event)
}

type WanderBehavior struct {
	CentersScentMap       [][]int
	EdgesScentMap         [][]int
	ReverseScentMapChance int
	CurScentMap           *[][]int
}

func NewWanderBehavior(centersScentMap, edgesScentMap [][]int) *WanderBehavior {
	return &WanderBehavior{
		CentersScentMap: centersScentMap,
		EdgesScentMap:   edgesScentMap,
	}
}

func (wander *WanderBehavior) Update(actor *entity.Actor, ctx *MonsterBehaviorContext) (entity.ActorStateType, []service.Event) {
	for _, player := range ctx.Session.Players {
		distance := actor.Pos.EuclideanDistance(player.Pos)
		if distance <= float64(actor.DerivedAttrs[entity.Hostility]) {
			return entity.AIStateChase, nil
		}
	}

	if ctx.Rng.Intn(entity.Guaranteed) < wander.ReverseScentMapChance {
		if wander.CurScentMap == &wander.CentersScentMap {
			wander.CurScentMap = &wander.EdgesScentMap
		} else {
			wander.CurScentMap = &wander.CentersScentMap
		}
		wander.ReverseScentMapChance = 0
	}
	wander.ReverseScentMapChance += 5
	scentMap := *wander.CurScentMap

	return entity.AIStateWander, ctx.Move.ExecuteMove(actor, scentMap, ctx.Session.Map)
}

type ChaseBehavior struct {
	PlayerScentMap [][]int
}

func NewChaseBehavior(playerScentMap [][]int) *ChaseBehavior {
	return &ChaseBehavior{
		PlayerScentMap: playerScentMap,
	}
}

func (chase *ChaseBehavior) Update(actor *entity.Actor, ctx *MonsterBehaviorContext) (entity.ActorStateType, []service.Event) {
	for _, player := range ctx.Session.Players {
		distance := actor.Pos.EuclideanDistance(player.Pos)
		if distance >= 1.5*float64(actor.DerivedAttrs[entity.Hostility]) {
			return entity.AIStateWander, nil
		}
	}

	return entity.AIStateWander, ctx.Move.ExecuteMove(actor, chase.PlayerScentMap, ctx.Session.Map)
}
