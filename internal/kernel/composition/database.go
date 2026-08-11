package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel"
	databasecapability "github.com/rin721/go-scaffold2/internal/kernel/capability/database"
	kernelcli "github.com/rin721/go-scaffold2/internal/kernel/cli"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgdatabase "github.com/rin721/go-scaffold2/pkg/database"
)

type databaseComposition struct {
	access       databasecapability.Access
	defaults     config.Binding
	cliContracts []kernelcli.Contract
}

func composeDatabase(runtime *kernel.Kernel) (databaseComposition, error) {
	return registerDatabase(runtime, databasecapability.Definition())
}

func registerDatabase(
	runtime *kernel.Kernel,
	definition kernel.Definition[databasecapability.Config, pkgdatabase.Client],
) (databaseComposition, error) {
	registration, err := kernel.Register(runtime, definition)
	if err != nil {
		return databaseComposition{}, fmt.Errorf("compose database capability: %w", err)
	}
	return databaseComposition{
		access:   registration.Access,
		defaults: registration.Defaults,
	}, nil
}
