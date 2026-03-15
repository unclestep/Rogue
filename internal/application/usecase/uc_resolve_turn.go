package usecase

import (
	"github.com/unclestep/Rogue/internal/application/port"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"maps"
)

type ResolveTurn struct {
	playRepo  port.PlaythroughRepository
	rulesRepo port.RulesRepository
}

type services struct {
	generator    *service.DungeonGenerator
	impactRes    *service.ImpactResolver
	pickup       *service.Pickup
	interactor   *service.Interactor
	itemUsage    *service.ItemUsage
	combat       *service.Combat
	movement     *service.Movement
	pathfinder   *service.Pathfinder
	moveResolver *service.MoveResolver
	monsterCtrl  *service.MonsterController
}

func NewServices(rules *model.GameRules) *services {
	services := &services{
		generator:    service.NewDungeonGeneratorService(rules),
		impactRes:    service.NewImpactResolverService(),
		pickup:       service.NewPickupService(),
		interactor:   service.NewInteractorService(),
		itemUsage:    service.NewItemUsageService(),
		pathfinder:   service.NewPathfinderService(),
		moveResolver: service.NewMoveResolverService(),
	}

	services.combat = service.NewCombatService(services.impactRes)
	services.movement = service.NewMovementService(services.impactRes, services.pickup)
	services.monsterCtrl = service.NewMonsterControllerService(services.pathfinder, services.moveResolver)

	return services
}

func NewResolveTurn(playRepo port.PlaythroughRepository, rulesRepo port.RulesRepository) *ResolveTurn {
	return &ResolveTurn{
		playRepo:  playRepo,
		rulesRepo: rulesRepo,
	}
}

func (uc *ResolveTurn) Execute(playId model.PlaythroughId) model.GameState {
	// Prepare for processing
	playthrough, _ := uc.playRepo.Get(playId)
	if playthrough.State != model.LobbyGameState {
		return playthrough.State
	}

	rules, _ := uc.rulesRepo.Get(playthrough.RulesId)
	ctx := model.NewSessionContext(playthrough)

	services := NewServices(rules)
	newState := model.PlayingGameState

	// Process turn
	uc.restoreStamina(playthrough)
	maps.Copy(playthrough.PendingIntents, services.monsterCtrl.Tick(ctx))
	uc.processIntents(ctx, services)
	uc.spawnLoots(ctx, service.NewObjectSpawner(rules))

	// Check result of turn
	if playthrough.AreAllPlayersDead() {
		newState = model.GameOverGameState
	} else if playthrough.AreAllAlivePlayersEscaped() {
		if rules.MaxDungeonCount == playthrough.Depth {
			newState = model.GameOverGameState
		} else {
			services.generator.Gen(ctx)
		}
	}

	playthrough.Seed = ctx.Rng().Int63()
	uc.playRepo.Save(playthrough)

	return newState
}

func (uc *ResolveTurn) restoreStamina(playthrough *model.Playthrough) {
	for _, player := range playthrough.Players {
		player.Vitals[model.VitalStamina] = player.DerivedAttrs[model.AttrMaxStamina]
	}

	for _, monster := range playthrough.Monsters {
		monster.Vitals[model.VitalStamina] = monster.DerivedAttrs[model.AttrMaxStamina]
	}
}

func (uc *ResolveTurn) processIntents(ctx *model.SessionContext, services *services) {
	for _, intent := range ctx.Playthrough.PendingIntents {
		uc.processIntent(ctx, services, intent)
	}
	clear(ctx.Playthrough.PendingIntents)
}

func (uc *ResolveTurn) processIntent(ctx *model.SessionContext, services *services, intent *model.Intent) []model.Event {
	events := make([]model.Event, 0)

	switch intent.IntentType {
	case model.IntentAttack:
		attacker, defender := ctx.Playthrough.GetActor(intent.Actor), ctx.Playthrough.GetActor(intent.Defender)
		events = append(events, services.combat.ExecuteAttack(ctx, attacker, defender))
	case model.IntentMove:
		mover := ctx.Playthrough.GetActor(intent.Actor)
		events = append(events, services.movement.ExecuteMove(ctx, mover, intent.Vector)...)
	case model.IntentInteract:
		actor := ctx.Playthrough.GetActor(intent.Actor)
		events = append(events, services.interactor.Execute(ctx, actor, actor.Pos.Add(intent.Vector)))
	case model.IntentConsume:
		user := ctx.Playthrough.GetActor(intent.Actor)
		item := ctx.Playthrough.GetItem(intent.ItemId)
		events = append(events, services.itemUsage.ConsumeItem(ctx.Playthrough, item, user))
	case model.IntentEquip:
		user := ctx.Playthrough.GetActor(intent.Actor)
		item := ctx.Playthrough.GetItem(intent.ItemId)
		events = append(events, services.itemUsage.EquipItem(ctx.Playthrough, item, user))
	case model.IntentUnequip:
		user := ctx.Playthrough.GetActor(intent.Actor)
		item := ctx.Playthrough.GetItem(intent.ItemId)
		events = append(events, services.itemUsage.UnequipItem(item, user))
	}

	for _, event := range events {
		event.Perform(ctx)
	}

	return events
}

func (uc *ResolveTurn) spawnLoots(ctx *model.SessionContext, objSpawner *service.ObjectSpawner) {
	if len(ctx.Playthrough.DeadMonsters) > 0 {
		for _, monster := range ctx.Playthrough.DeadMonsters {
			objSpawner.SpawnTreasure(ctx, monster)
		}
	}
	clear(ctx.Playthrough.DeadMonsters)
}
