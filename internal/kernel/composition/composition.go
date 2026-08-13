// Package composition 显式选择并安装当前进程使用的底层 App 组件。
package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/app"
	databaseapp "github.com/rin721/go-scaffold2/internal/kernel/app/database"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
	pkgclock "github.com/rin721/go-scaffold2/pkg/clock"
	pkgidgen "github.com/rin721/go-scaffold2/pkg/idgen"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
	pkgvalidation "github.com/rin721/go-scaffold2/pkg/validation"
)

// LoggerSelection 表示当前 composition 是否用配置化 App 替换 Kernel 内置 Logger。
type LoggerSelection uint8

const (
	// KernelBuiltinLogger 只使用 Kernel 构造时注入的基线 Logger。
	KernelBuiltinLogger LoggerSelection = iota
	// ConfiguredLoggerReplacement 显式加入配置化 Logger replacement App。
	ConfiguredLoggerReplacement
)

// Options 配置 composition 可选的启动前能力。
type Options struct {
	Logger LoggerSelection
	CLI    *CLIOptions
}

// CLIOptions 配置可选的启动前 CLI App。
type CLIOptions struct{ App pkgcli.Config }

// Capabilities 保存当前进程已经显式选择的稳定能力入口。
type Capabilities struct {
	Logger        pkglogger.Logger
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
	loggerOutput, err := composeBuiltinLogger(plan, runtime.LoggerTarget())
	if err != nil {
		return Capabilities{}, err
	}
	switch options.Logger {
	case KernelBuiltinLogger:
	case ConfiguredLoggerReplacement:
		if err := composeLoggerReplacement(plan, loggerOutput.Binding); err != nil {
			return Capabilities{}, err
		}
	default:
		return Capabilities{}, fmt.Errorf("unsupported logger selection %d", options.Logger)
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
		Logger: loggerOutput.Output.Logger(), Clock: clockOutput.Output,
		IDGenerator: idOutput.Output, Validator: validatorOutput.Output,
		Database: databaseOutput.Output, Configuration: configurationOutput.manager, CLI: cliOutput,
	}, nil
}
