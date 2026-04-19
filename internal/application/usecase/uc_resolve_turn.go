package usecase

import (
	"time"

	"github.com/unclestep/Rogue/internal/application/port"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
)

type ResolveTurn struct {
	playRepo  port.PlaythroughRepository
	rulesRepo port.RulesRepository

	// Stateless services — created once, reused every turn.
	impactRes    *service.ImpactResolver
	pickup       *service.Pickup
	interactor   *service.Interactor
	itemUsage    *service.ItemUsage
	combat       *service.Combat
	movement     *service.Movement
	pathfinder   *service.Pathfinder
	moveResolver *service.MoveResolver
	monsterCtrl  *service.MonsterController
	raycaster    *service.Raycaster

	// Stateless generator helpers — combined with runtime rules to build DungeonGenerator per turn.
	topologyGenerator *service.TopologyGenerator
	doorLocker        *service.DoorLocker
}

func NewResolveTurn(
	playRepo port.PlaythroughRepository,
	rulesRepo port.RulesRepository,
	impactRes *service.ImpactResolver,
	pickup *service.Pickup,
	interactor *service.Interactor,
	itemUsage *service.ItemUsage,
	combat *service.Combat,
	movement *service.Movement,
	pathfinder *service.Pathfinder,
	moveResolver *service.MoveResolver,
	monsterCtrl *service.MonsterController,
	raycaster *service.Raycaster,
	topologyGenerator *service.TopologyGenerator,
	doorLocker *service.DoorLocker,
) *ResolveTurn {
	return &ResolveTurn{
		playRepo:          playRepo,
		rulesRepo:         rulesRepo,
		impactRes:         impactRes,
		pickup:            pickup,
		interactor:        interactor,
		itemUsage:         itemUsage,
		combat:            combat,
		movement:          movement,
		pathfinder:        pathfinder,
		moveResolver:      moveResolver,
		monsterCtrl:       monsterCtrl,
		raycaster:         raycaster,
		topologyGenerator: topologyGenerator,
		doorLocker:        doorLocker,
	}
}

func (uc *ResolveTurn) Execute(playId model.PlaythroughId) model.GameState {
	// Prepare for processing
	playthrough, err := uc.playRepo.Get(playId)
	if err != nil || playthrough == nil {
		// Playthrough not found: create a fallback session and return to Lobby.
		// This guards against panics when storage has no in-memory cache fallback.
		_ = uc.playRepo.Create(model.DefaultRulesId, time.Now().UnixNano())
		return model.LobbyGameState
	}
	if playthrough.State != model.PlayingGameState {
		return playthrough.State
	}

	rules, _ := uc.rulesRepo.Get(playthrough.RulesId)
	ctx := model.NewSessionContext(playthrough)
	newState := model.PlayingGameState

	// Generate first dungeon if this is a new game (safety net).
	if !playthrough.IsDungeonCreated() {
		objSpawner := service.NewObjectSpawner(rules)
		generator := service.NewDungeonGeneratorService(rules, uc.topologyGenerator, objSpawner, uc.doorLocker)
		generator.Gen(ctx)
		ctx.BuildScentMaps()
	}

	// Clear stale events from previous turn.
	playthrough.TurnEvents = nil

	// Process turn.
	uc.restoreStamina(playthrough)
	playthrough.PendingIntents = append(playthrough.PendingIntents, uc.monsterCtrl.Tick(ctx)...)
	uc.processIntents(ctx)
	uc.tickEffects(ctx)

	if len(playthrough.DeadMonsters) > 0 {
		uc.spawnLoots(ctx, service.NewObjectSpawner(rules))
	}

	// Check result of turn.
	if playthrough.AreAllPlayersDead() {
		newState = model.GameOverGameState
	} else if playthrough.AreAllAlivePlayersEscaped() {
		if rules.MaxDungeonCount == playthrough.Depth {
			newState = model.GameOverGameState
		} else {
			// Wipe the previous level's FOW — old grid coordinates have no
			// meaning on the newly generated map (different rooms, same size).
			// Setting entries to nil causes RefreshPlayerFoW to allocate a
			// fresh VisibleArea with the correct new dimensions below.
			for id := range playthrough.PlayersFoW {
				playthrough.PlayersFoW[id] = nil
			}

			objSpawner := service.NewObjectSpawner(rules)
			generator := service.NewDungeonGeneratorService(rules, uc.topologyGenerator, objSpawner, uc.doorLocker)
			generator.Gen(ctx)

			// Update DeepestLevel for every player who survived to this floor.
			for _, player := range playthrough.Players {
				if player.Vitals[model.VitalHP] > 0 {
					if stats, ok := playthrough.PlayersStats[player.Id]; ok && playthrough.Depth > stats.DeepestLevel {
						stats.DeepestLevel = playthrough.Depth
					}
				}
			}
		}
	}

	// Refresh FOW for all alive players after movement and effects are resolved.
	// This keeps the flashlight cone anchored to the player's new position.
	for _, player := range playthrough.Players {
		uc.raycaster.RefreshPlayerFoW(playthrough, player.Id, service.FlashlightHalfFOV, service.FlashlightRange)
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

func (uc *ResolveTurn) processIntents(ctx *model.SessionContext) {
	for _, intent := range ctx.Playthrough.PendingIntents {
		events := uc.processIntent(ctx, intent)
		ctx.Playthrough.TurnEvents = append(ctx.Playthrough.TurnEvents, events...)
	}
	ctx.Playthrough.PendingIntents = ctx.Playthrough.PendingIntents[:0]
}

func (uc *ResolveTurn) processIntent(ctx *model.SessionContext, intent *model.Intent) []model.Event {
	// MonsterController.Tick can store nil when MoveResolver finds no valid action.
	if intent == nil {
		return nil
	}

	events := make([]model.Event, 0)

	switch intent.IntentType {
	case model.IntentAttack:
		attacker, defender := ctx.Playthrough.GetActor(intent.Actor), ctx.Playthrough.GetActor(intent.Defender)
		if event := uc.combat.ExecuteAttack(ctx, attacker, defender); event != nil {
			events = append(events, event)
		}
	case model.IntentMove:
		mover := ctx.Playthrough.GetActor(intent.Actor)
		events = append(events, uc.movement.ExecuteMove(ctx, mover, intent.Vector)...)
	case model.IntentInteract:
		actor := ctx.Playthrough.GetActor(intent.Actor)
		if actor == nil {
			break
		}
		events = append(events, uc.interactor.Execute(ctx, actor, actor.Pos.Add(intent.Vector)))
	case model.IntentConsume:
		user := ctx.Playthrough.GetActor(intent.Actor)
		item := ctx.Playthrough.GetItem(intent.ItemId)
		if user == nil || item == nil {
			break
		}
		if event := uc.itemUsage.ConsumeItem(item, user); event != nil {
			events = append(events, event)
		}
	case model.IntentEquip:
		user := ctx.Playthrough.GetActor(intent.Actor)
		item := ctx.Playthrough.GetItem(intent.ItemId)
		if user == nil || item == nil {
			break
		}
		if event := uc.itemUsage.EquipItem(ctx.Playthrough, item, user); event != nil {
			events = append(events, event)
		}
	case model.IntentUnequip:
		user := ctx.Playthrough.GetActor(intent.Actor)
		item := ctx.Playthrough.GetItem(intent.ItemId)
		if user == nil || item == nil {
			break
		}
		if event := uc.itemUsage.UnequipItem(item, user); event != nil {
			events = append(events, event)
		}
	default:
	}

	for _, event := range events {
		event.Perform(ctx)
	}

	return events
}

func (uc *ResolveTurn) tickEffects(ctx *model.SessionContext) {
	for _, player := range ctx.Playthrough.Players {
		uc.impactRes.TickEffects(*ctx, player)
	}
	for _, monster := range ctx.Playthrough.Monsters {
		uc.impactRes.TickEffects(*ctx, monster)
	}
}

func (uc *ResolveTurn) spawnLoots(ctx *model.SessionContext, objSpawner *service.ObjectSpawner) {
	if len(ctx.Playthrough.DeadMonsters) > 0 {
		for _, monster := range ctx.Playthrough.DeadMonsters {
			objSpawner.SpawnTreasure(ctx, monster)
		}
	}
	clear(ctx.Playthrough.DeadMonsters)
}
