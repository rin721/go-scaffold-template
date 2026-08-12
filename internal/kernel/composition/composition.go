// Package composition 显式选择并安装当前进程使用的底层 App 组件。
package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/app"
	databaseapp "github.com/rin721/go-scaffold2/internal/kernel/app/database"
	loggerapp "github.com/rin721/go-scaffold2/internal/kernel/app/logger"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
	pkgclock "github.com/rin721/go-scaffold2/pkg/clock"
	pkgidgen "github.com/rin721/go-scaffold2/pkg/idgen"
	pkgvalidation "github.com/rin721/go-scaffold2/pkg/validation"
)

// Options 配置 composition 可选的启动前能力。
type Options struct{ CLI *CLIOptions }

// CLIOptions 配置可选的启动前 CLI App。
type CLIOptions struct{ App pkgcli.Config }

// Capabilities 保存当前进程已经显式选择的稳定能力入口。
type Capabilities struct {
	Logger        loggerapp.Access
	Clock         pkgclock.Clock
	IDGenerator   pkgidgen.Generator
	Validator     pkgvalidation.Validator
	Database      databaseapp.Access
	Configuration config.DefaultManager
	CLI           pkgcli.App
}

// Compose 在本地完整构造并校验 Plan，最后一次性安装到 Kernel。
func Compose(runtime *kernel.Kernel, options Options) (Capabilities, error) {
	if runtime == nil {
		return Capabilities{}, fmt.Errorf("compose runtime is nil")
	}
	plan := app.NewPlan()
	loggerOutput, err := composeLogger(plan, runtime.LoggingManager())
	if err != nil {
		return Capabilities{}, err
	}
	clockOutput, err := composeClock(plan)
	if err != nil {
		return Capabilities{}, err
	}
	idOutput, err := composeIDGenerator(plan)
	if err != nil {
		return Capabilities{}, err
	}
	validatorOutput, err := composeValidator(plan)
	if err != nil {
		return Capabilities{}, err
	}
	databaseOutput, err := composeDatabase(plan)
	if err != nil {
		return Capabilities{}, err
	}
	frozen, err := plan.Freeze()
	if err != nil {
		return Capabilities{}, fmt.Errorf("freeze component plan: %w", err)
	}
	configurationOutput, err := composeConfiguration(frozen.Defaults()...)
	if err != nil {
		return Capabilities{}, err
	}
	contracts := append(configurationOutput.cliContracts, frozen.CLIContracts()...)
	cliOutput, err := composeCLI(options.CLI, contracts...)
	if err != nil {
		return Capabilities{}, err
	}
	if err := runtime.Install(frozen); err != nil {
		return Capabilities{}, fmt.Errorf("install component plan: %w", err)
	}
	return Capabilities{
		Logger: loggerOutput.Output, Clock: clockOutput.Output,
		IDGenerator: idOutput.Output, Validator: validatorOutput.Output,
		Database: databaseOutput.Output, Configuration: configurationOutput.manager, CLI: cliOutput,
	}, nil
}
