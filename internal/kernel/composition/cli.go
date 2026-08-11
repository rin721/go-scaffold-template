package composition

import (
	"fmt"

	kernelcli "github.com/rin721/go-scaffold2/internal/kernel/cli"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
)

func composeCLI(options *CLIOptions, contracts ...kernelcli.Contract) (pkgcli.App, error) {
	if options == nil {
		return nil, nil
	}
	app, err := kernelcli.NewApp(options.App, contracts...)
	if err != nil {
		return nil, fmt.Errorf("compose CLI: %w", err)
	}
	return app, nil
}
