package bootstrap

// Provider sets and adapter functions shared across all binaries.
// Each binary's wire.go imports and composes these sets as needed.

import (
	"github.com/google/wire"
	"github.com/unclestep/Rogue/internal/application/port"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/infrastructure/storage"
	jsonStorage "github.com/unclestep/Rogue/internal/infrastructure/storage/json"
)

// provideJsonPlaythroughRepo adapts Config.PlaythroughsDir into the constructor.
func provideJsonPlaythroughRepo(cfg Config) *jsonStorage.JsonPlaythroughRepo {
	return jsonStorage.NewJsonPlaythroughRepo(cfg.PlaythroughsDir)
}

// provideJsonRulesRepo adapts Config.RulesDir into the constructor.
func provideJsonRulesRepo(cfg Config) *jsonStorage.JsonRulesRepo {
	return jsonStorage.NewJsonRulesRepo(cfg.RulesDir)
}

// provideCachedRepo wraps the JSON repo in a cache layer.
// The explicit *JsonPlaythroughRepo parameter lets Wire disambiguate the two
// port.PlaythroughRepository bindings (backend vs. cache layer).
func provideCachedRepo(j *jsonStorage.JsonPlaythroughRepo) *storage.CachedPlaythroughRepo {
	return storage.NewCachedPlaythroughRepo(j)
}

// InfrastructureSet wires the storage layer.
var InfrastructureSet = wire.NewSet(
	provideJsonPlaythroughRepo,
	provideCachedRepo,
	wire.Bind(new(port.PlaythroughRepository), new(*storage.CachedPlaythroughRepo)),
	provideJsonRulesRepo,
	wire.Bind(new(port.RulesRepository), new(*jsonStorage.JsonRulesRepo)),
)

// ServicesSet wires all stateless domain services.
var ServicesSet = wire.NewSet(
	service.NewImpactResolverService,
	service.NewPickupService,
	service.NewInteractorService,
	service.NewItemUsageService,
	service.NewCombatService,
	service.NewMovementService,
	service.NewPathfinderService,
	service.NewMoveResolverService,
	service.NewMonsterControllerService,
	service.NewTopologyGenerator,
	service.NewDoorLocker,
	service.NewRaycaster,
	service.NewFallbackPolicy,
	wire.Bind(new(service.Policy), new(*service.FallbackPolicy)),
)
