// Package composition 显式选择并登记由 Kernel 托管的能力定义。
package composition

import (
	"github.com/rin721/go-scaffold2/internal/kernel"
	databasecapability "github.com/rin721/go-scaffold2/internal/kernel/capability/database"
	loggercapability "github.com/rin721/go-scaffold2/internal/kernel/capability/logger"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
)

// Options 控制 composition 中的可选启动前能力。
type Options struct {
	CLI *CLIOptions
}

// CLIOptions 配置可选的启动前 CLI App。
type CLIOptions struct {
	App pkgcli.Config
}

// Capabilities 保存已完成组合的稳定能力入口。
type Capabilities struct {
	Logger        loggercapability.Access
	Database      databasecapability.Access
	Configuration config.DefaultManager
	CLI           pkgcli.App
}

// Compose 按固定清单把能力定义显式登记到尚未启动的 Kernel。
//
// Compose 当前按 Logger、Database 顺序登记。调用方必须主动调用本函数；Kernel.New 不会自动
// 发现、选择或登记任何能力。
func Compose(runtime *kernel.Kernel, options Options) (Capabilities, error) {
	loggerBinding, err := composeLogger(runtime)
	if err != nil {
		return Capabilities{}, err
	}
	databaseBinding, err := composeDatabase(runtime)
	if err != nil {
		return Capabilities{}, err
	}
	configurationBinding, err := composeConfiguration(loggerBinding.defaults, databaseBinding.defaults)
	if err != nil {
		return Capabilities{}, err
	}
	contracts := append(configurationBinding.cliContracts, databaseBinding.cliContracts...)
	app, err := composeCLI(options.CLI, contracts...)
	if err != nil {
		return Capabilities{}, err
	}
	return Capabilities{
		Logger:        loggerBinding.access,
		Database:      databaseBinding.access,
		Configuration: configurationBinding.manager,
		CLI:           app,
	}, nil
}
