// Package builtin 建立 Kernel 随仓库提供的封闭内置能力 catalog。
package builtin

import (
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	builtincli "github.com/rin721/go-scaffold2/internal/kernel/builtin/cli"
	builtinconfig "github.com/rin721/go-scaffold2/internal/kernel/builtin/config"
	builtinlogger "github.com/rin721/go-scaffold2/internal/kernel/builtin/logger"
	kernelcli "github.com/rin721/go-scaffold2/internal/kernel/cli"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

// Catalog 是当前 Kernel 唯一允许登记的内置能力清单。
type Catalog struct {
	Config app.BuiltinDefinition[config.Provider, config.Provider]
	Logger app.BuiltinDefinition[pkglogger.Logger, pkglogger.Access]
	CLI    app.BuiltinDefinition[kernelcli.Factory, pkgcli.App]
}

// NewCatalog 构造 Config、Logger、CLI 三项封闭 baseline Definition。
func NewCatalog(configOptions builtinconfig.Options, loggerOptions builtinlogger.Options, cliOptions builtincli.Options) (Catalog, error) {
	configDefinition, err := builtinconfig.Definition(configOptions)
	if err != nil {
		return Catalog{}, fmt.Errorf("define builtin config: %w", err)
	}
	loggerDefinition, err := builtinlogger.Definition(loggerOptions)
	if err != nil {
		return Catalog{}, fmt.Errorf("define builtin logger: %w", err)
	}
	cliDefinition, err := builtincli.Definition(cliOptions)
	if err != nil {
		return Catalog{}, fmt.Errorf("define builtin CLI: %w", err)
	}
	return Catalog{Config: configDefinition, Logger: loggerDefinition, CLI: cliDefinition}, nil
}
