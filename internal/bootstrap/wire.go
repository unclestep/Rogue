//go:build wireinject

package bootstrap

// Standalone injector composed from the shared provider sets.
// Run `go generate ./internal/bootstrap/` to regenerate wire_gen.go.

//go:generate go run github.com/google/wire/cmd/wire

import (
	"github.com/google/wire"
	"github.com/unclestep/Rogue/internal/application"
	"github.com/unclestep/Rogue/internal/application/usecase"
	"github.com/unclestep/Rogue/internal/application/view"
	"github.com/unclestep/Rogue/internal/presentation/network"
)

// InitializeApp is the composition root for the standalone binary.
// Wire generates the function body in wire_gen.go.
func InitializeApp(cfg Config) *network.App {
	wire.Build(
		InfrastructureSet,
		ServicesSet,
		usecase.NewResolveTurn,
		usecase.NewSubmitIntent,
		usecase.NewResolveState,
		view.NewMapper,
		application.NewGameLoop,
		network.NewServer,
		network.NewClient,
		network.NewApp,
	)
	return nil
}
