// Package composition 显式选择并安装当前进程使用的底层 App 组件。
package composition

import (
	"context"
	"errors"
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/app"
	databaseapp "github.com/rin721/go-scaffold2/internal/kernel/app/database"
	loggerapp "github.com/rin721/go-scaffold2/internal/kernel/app/logger"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
	pkgclock "github.com/rin721/go-scaffold2/pkg/clock"
	pkgidgen "github.com/rin721/go-scaffold2/pkg/idgen"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
	pkgvalidation "github.com/rin721/go-scaffold2/pkg/validation"
)

const (
	mainLoggerID           app.ID = "logging.main"
	mainLoggerConfigPath          = "logger"
	mainDatabaseID         app.ID = "database.db1"
	mainDatabaseConfigPath        = "database"
)

// Capabilities 保存当前进程已经显式选择的稳定能力入口。
type Capabilities struct {
	Runtime       *kernel.Kernel
	Logger        pkglogger.Access
	Clock         pkgclock.Clock
	IDGenerator   pkgidgen.Generator
	Validator     pkgvalidation.Validator
	Database      databaseapp.Access
	Configuration config.DefaultManager
	CLI           pkgcli.App
}

// Compose 在 Assembly 的唯一 Plan 中选择组件，冻结后一次性安装 Kernel。
func Compose(assembly *kernel.Assembly) (capabilities Capabilities, err error) {
	if assembly == nil {
		return Capabilities{}, fmt.Errorf("compose assembly is nil")
	}
	installed := false
	defer func() {
		if err != nil && !installed {
			err = errors.Join(err, assembly.Close(context.Background()))
		}
	}()
	plan, builtins, err := assembly.Plan()
	if err != nil {
		return Capabilities{}, fmt.Errorf("create assembly plan: %w", err)
	}
	replacement, err := loggerapp.Replacement(app.Spec{ID: mainLoggerID, ConfigPath: mainLoggerConfigPath})
	if err != nil {
		return Capabilities{}, fmt.Errorf("define main logger replacement: %w", err)
	}
	if err := app.Replace(plan, builtins.Logging.Role, replacement); err != nil {
		return Capabilities{}, fmt.Errorf("replace builtin logger: %w", err)
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
	databaseDefinition, err := databaseapp.Definition(app.Spec{ID: mainDatabaseID, ConfigPath: mainDatabaseConfigPath}, app.InputOf(builtins.Logging.Output.Binding()))
	if err != nil {
		return Capabilities{}, fmt.Errorf("define main database: %w", err)
	}
	databaseOutput, err := app.Add(plan, databaseDefinition)
	if err != nil {
		return Capabilities{}, fmt.Errorf("compose main database: %w", err)
	}
	frozen, err := plan.Freeze()
	if err != nil {
		return Capabilities{}, fmt.Errorf("freeze component plan: %w", err)
	}
	configurationOutput, err := composeConfiguration(frozen.Defaults()...)
	if err != nil {
		return Capabilities{}, err
	}
	var cliOutput pkgcli.App
	factory, activateErr := assembly.ActivateCLI()
	if activateErr != nil {
		return Capabilities{}, activateErr
	}
	if factory != nil {
		contracts := append(configurationOutput.cliContracts, frozen.CLIContracts()...)
		cliOutput, err = factory.Build(contracts)
		if err != nil {
			return Capabilities{}, fmt.Errorf("compose CLI: %w", err)
		}
	}
	runtime, err := assembly.Install(frozen)
	if err != nil {
		return Capabilities{}, err
	}
	installed = true
	return Capabilities{Runtime: runtime, Logger: assembly.Logging(), Clock: clockOutput.Output, IDGenerator: idOutput.Output, Validator: validatorOutput.Output, Database: databaseOutput.Output, Configuration: configurationOutput.manager, CLI: cliOutput}, nil
}
