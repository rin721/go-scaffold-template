package composition

import (
	"fmt"

	kernelcli "github.com/rin721/go-scaffold2/internal/kernel/cli"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

type configurationComposition struct {
	manager      config.DefaultManager
	cliContracts []kernelcli.Contract
}

func composeConfiguration(bindings ...config.Binding) (configurationComposition, error) {
	manager, err := config.NewDefaultManager(bindings...)
	if err != nil {
		return configurationComposition{}, fmt.Errorf("compose default configuration manager: %w", err)
	}
	return configurationComposition{
		manager:      manager,
		cliContracts: []kernelcli.Contract{kernelcli.ConfigCommands(manager)},
	}, nil
}
