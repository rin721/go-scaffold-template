package kernel

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	"github.com/rin721/go-scaffold2/internal/kernel/builtin"
	builtincli "github.com/rin721/go-scaffold2/internal/kernel/builtin/cli"
	builtinconfig "github.com/rin721/go-scaffold2/internal/kernel/builtin/config"
	builtinlogger "github.com/rin721/go-scaffold2/internal/kernel/builtin/logger"
	kernelcli "github.com/rin721/go-scaffold2/internal/kernel/cli"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

// AssemblyOptions 固定声明当前进程的内置 baseline 与 Kernel 事务边界。
type AssemblyOptions struct {
	Config builtinconfig.Options
	Logger builtinlogger.Options
	CLI    *builtincli.Options
	Kernel Options
}

// Builtins 是 Assembly 返回给 Composition 的封闭 typed Role 集合。
type Builtins struct {
	Config  app.BuiltinRole[config.Provider]
	Logging struct {
		Role   app.BuiltinRole[pkglogger.Logger]
		Output app.BuiltinOutput[pkglogger.Access]
	}
	CLI app.BuiltinRole[kernelcli.Factory]
}

// Assembly 按 Bootstrap、App Plan、PreStart 顺序构造并安装 Kernel。
type Assembly struct {
	options      AssemblyOptions
	plan         *app.Plan
	builtins     Builtins
	logging      pkglogger.Access
	runtime      *Kernel
	planned      bool
	installed    bool
	cliActivated bool
}

// NewAssembly 创建尚未建立 Plan 的 Kernel Assembly。
func NewAssembly(options AssemblyOptions) (*Assembly, error) {
	if options.Kernel.Logging != nil {
		return nil, fmt.Errorf("assembly kernel logging is governed by builtin logger")
	}
	return &Assembly{options: options}, nil
}

// Plan 构造封闭 catalog 的 Bootstrap baseline，并返回唯一可编辑 Plan。
func (a *Assembly) Plan() (*app.Plan, Builtins, error) {
	if a == nil {
		return nil, Builtins{}, fmt.Errorf("kernel assembly is nil")
	}
	if a.planned {
		return nil, Builtins{}, fmt.Errorf("kernel assembly plan already created")
	}
	plan := app.NewPlan()
	cliOptions := builtincli.Options{}
	selectedCLI := a.options.CLI != nil
	if selectedCLI {
		cliOptions = *a.options.CLI
	}
	catalog, err := builtin.NewCatalog(a.options.Config, a.options.Logger, cliOptions)
	if err != nil {
		return nil, Builtins{}, err
	}
	configRole, _, _, err := app.RegisterBuiltin(plan, catalog.Config, true)
	if err != nil {
		return nil, Builtins{}, fmt.Errorf("register builtin config: %w", err)
	}
	loggerRole, loggerOutput, logging, err := app.RegisterBuiltin(plan, catalog.Logger, true)
	if err != nil {
		_ = app.CloseBuiltins(context.Background(), plan)
		return nil, Builtins{}, fmt.Errorf("register builtin logger: %w", err)
	}
	cliRole, _, _, err := app.RegisterBuiltin(plan, catalog.CLI, selectedCLI)
	if err != nil {
		_ = app.CloseBuiltins(context.Background(), plan)
		return nil, Builtins{}, fmt.Errorf("register builtin CLI: %w", err)
	}
	builtins := Builtins{Config: configRole, CLI: cliRole}
	builtins.Logging.Role, builtins.Logging.Output = loggerRole, loggerOutput
	a.plan, a.builtins, a.logging, a.planned = plan, builtins, logging, true
	return plan, builtins, nil
}

// Install 在 Plan Freeze 后一次性安装 Runtime。
func (a *Assembly) Install(frozen app.FrozenPlan) (*Kernel, error) {
	if a == nil || !a.planned || a.plan == nil {
		return nil, fmt.Errorf("kernel assembly plan is not created")
	}
	if a.installed {
		return nil, fmt.Errorf("kernel assembly is already installed")
	}
	if err := frozen.Validate(); err != nil {
		return nil, fmt.Errorf("validate component plan: %w", err)
	}
	if !frozen.BelongsTo(a.plan) {
		return nil, fmt.Errorf("frozen component plan belongs to another assembly")
	}
	if a.options.CLI != nil && !a.cliActivated {
		return nil, fmt.Errorf("selected builtin CLI is not activated")
	}
	provider, err := app.BuiltinTarget(a.builtins.Config)
	if err != nil {
		return nil, fmt.Errorf("resolve builtin config: %w", err)
	}
	options := a.options.Kernel
	options.Logging, options.builtins = a.logging, a.plan
	runtime, err := New(provider, options)
	if err != nil {
		return nil, fmt.Errorf("create kernel runtime: %w", err)
	}
	if err := runtime.Install(frozen); err != nil {
		return nil, fmt.Errorf("install component plan: %w", err)
	}
	a.runtime, a.installed = runtime, true
	return runtime, nil
}

// ActivateCLI 在 Plan Freeze 后构造已选择的 PreStart CLI Factory。
func (a *Assembly) ActivateCLI() (kernelcli.Factory, error) {
	if a == nil || a.options.CLI == nil {
		return nil, nil
	}
	factory, err := app.ActivateSelected(a.builtins.CLI)
	if err != nil {
		return nil, fmt.Errorf("activate builtin CLI: %w", err)
	}
	a.cliActivated = true
	return factory, nil
}

// Runtime 返回成功安装后的 Kernel；安装前返回 nil。
func (a *Assembly) Runtime() *Kernel {
	if a == nil {
		return nil
	}
	return a.runtime
}

// Logging 返回 Assembly 已构造且与 root Binding 相同的稳定 Logger Access。
func (a *Assembly) Logging() pkglogger.Access {
	if a == nil {
		return nil
	}
	return a.logging
}

// Close 释放尚未由 Runtime 停止路径释放的 Assembly-owned baseline。
func (a *Assembly) Close(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	if a == nil || a.plan == nil {
		return nil
	}
	return app.CloseBuiltins(ctx, a.plan)
}
