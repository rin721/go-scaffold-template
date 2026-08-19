// Package composition 显式选择并安装当前进程使用的底层 App 组件。
package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel"
	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	cacheapp "github.com/rin721/go-scaffold-template/internal/kernel/app/cache"
	databaseapp "github.com/rin721/go-scaffold-template/internal/kernel/app/database"
	executionapp "github.com/rin721/go-scaffold-template/internal/kernel/app/execution"
	storageapp "github.com/rin721/go-scaffold-template/internal/kernel/app/storage"
	pkgclock "github.com/rin721/go-scaffold-template/pkg/clock"
	pkgi18n "github.com/rin721/go-scaffold-template/pkg/i18n"
	pkgidgen "github.com/rin721/go-scaffold-template/pkg/idgen"
	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
	pkgvalidation "github.com/rin721/go-scaffold-template/pkg/validation"
)

// LoggerSelection 表示当前 composition 是否用配置化 App 替换 Kernel 内置 Logger。
type LoggerSelection uint8

const (
	// KernelBuiltinLogger 只使用 Kernel 构造时注入的基线 Logger。
	KernelBuiltinLogger LoggerSelection = iota
	// ConfiguredLoggerReplacement 显式加入配置化 Logger replacement App。
	ConfiguredLoggerReplacement
)

// Options 配置 Service composition 的显式底层能力选择。
type Options struct {
	Logger LoggerSelection
}

// Capabilities 保存当前进程已经显式选择的稳定能力入口。
type Capabilities struct {
	Logger      pkglogger.Logger
	Clock       pkgclock.Clock
	IDGenerator pkgidgen.Generator
	Validator   pkgvalidation.Validator
	Database    databaseapp.Access
	Cache       cacheapp.Access
	I18n        pkgi18n.Translator
	Storage     storageapp.Access
	Execution   executionapp.Access
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
	cacheOutput, err := composeCache(plan)
	if err != nil {
		return Capabilities{}, err
	}
	i18nOutput, err := composeI18n(plan)
	if err != nil {
		return Capabilities{}, err
	}
	storageOutput, err := composeStorage(plan)
	if err != nil {
		return Capabilities{}, err
	}
	executionOutput, err := composeExecution(plan, loggerOutput.Binding)
	if err != nil {
		return Capabilities{}, err
	}
	frozen, err := plan.Freeze()
	if err != nil {
		return Capabilities{}, fmt.Errorf("freeze component plan: %w", err)
	}
	if err := runtime.Install(frozen); err != nil {
		return Capabilities{}, fmt.Errorf("install component plan: %w", err)
	}
	return Capabilities{
		Logger: loggerOutput.Output.Logger(), Clock: clockOutput.Output,
		IDGenerator: idOutput.Output, Validator: validatorOutput.Output,
		Database: databaseOutput.Output, Cache: cacheOutput.Output,
		I18n: i18nOutput.Output, Storage: storageOutput.Output,
		Execution: executionOutput.Output,
	}, nil
}
