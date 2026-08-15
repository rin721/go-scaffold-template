// Package composition 是进程唯一的 application composition root。
package composition

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"

	kernelcli "github.com/rin721/go-scaffold-template/internal/kernel/cli"
	kernelcomposition "github.com/rin721/go-scaffold-template/internal/kernel/composition"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	"github.com/rin721/go-scaffold-template/internal/kernel/logging"
	authconfig "github.com/rin721/go-scaffold-template/internal/module/auth/binding/config"
	migrationcli "github.com/rin721/go-scaffold-template/internal/module/migration/binding/cli"
	migrationconfig "github.com/rin721/go-scaffold-template/internal/module/migration/binding/config"
	opsconfig "github.com/rin721/go-scaffold-template/internal/module/ops/binding/config"
	opsmodel "github.com/rin721/go-scaffold-template/internal/module/ops/model"
	todocli "github.com/rin721/go-scaffold-template/internal/module/todo/binding/cli"
	configbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/config"
	"github.com/rin721/go-scaffold-template/pkg/cli"
	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
)

const (
	ExitSuccess     = cli.ExitSuccess
	ExitError       = cli.ExitError
	ExitUsage       = cli.ExitUsage
	ExitConfig      = cli.ExitConfig
	ExitInterrupted = cli.ExitInterrupted
)

// Config 保存进程组合需要的固定输入，不包含运行期配置值。
type Config struct {
	Name              string
	Description       string
	ConfigPath        string
	EnvironmentPrefix string
	Stdin             io.Reader
	Stdout            io.Writer
	Stderr            io.Writer
	Logging           *logging.Manager
	Build             BuildInfo
}

// EntryConfig 保存进程入口允许提交给 application composition 的固定输入。
type EntryConfig struct {
	Name              string
	Description       string
	ConfigPath        string
	EnvironmentPrefix string
	Stdin             io.Reader
	Stdout            io.Writer
	Stderr            io.Writer
	Build             BuildInfo
}

// BuildInfo 是进程入口提交给 application composition 的构建元数据。
type BuildInfo struct {
	Version   string
	Commit    string
	BuildTime string
	GoVersion string
	Dirty     bool
}

// Application 按参数选择 Bootstrap/Application CLI 或长期 Service 模式。
type Application struct{ config Config }

// Run 从进程入口输入构造基线日志与 Application，并完整释放入口拥有的日志资源。
func Run(ctx context.Context, cfg EntryConfig, args []string) (runErr error) {
	baseline, err := pkglogger.New(nil)
	if err != nil {
		return fmt.Errorf("create baseline logger: %w", err)
	}
	defer func() {
		if closeErr := baseline.Close(); closeErr != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close baseline logger: %w", closeErr))
		}
	}()
	manager, err := logging.New(baseline)
	if err != nil {
		return fmt.Errorf("create kernel logging manager: %w", err)
	}
	application, err := New(Config{
		Name: cfg.Name, Description: cfg.Description,
		ConfigPath: cfg.ConfigPath, EnvironmentPrefix: cfg.EnvironmentPrefix,
		Stdin: cfg.Stdin, Stdout: cfg.Stdout, Stderr: cfg.Stderr,
		Logging: manager, Build: cfg.Build,
	})
	if err != nil {
		return err
	}
	return application.Run(ctx, args)
}

// ExitCode 把应用错误转换为稳定进程退出码。
func ExitCode(err error) int { return cli.GetExitCode(err) }

// New 创建尚未执行的 Application。
func New(cfg Config) (*Application, error) {
	if cfg.Name == "" || cfg.Description == "" {
		return nil, fmt.Errorf("application identity is incomplete")
	}
	if cfg.ConfigPath == "" || cfg.EnvironmentPrefix == "" {
		return nil, fmt.Errorf("application configuration source is incomplete")
	}
	if cfg.Logging == nil {
		return nil, fmt.Errorf("application logging manager is nil")
	}
	if cfg.Build.Version == "" && cfg.Build.Commit == "" && cfg.Build.BuildTime == "" && cfg.Build.GoVersion == "" {
		cfg.Build = BuildInfo{Version: "dev", Commit: "unknown", BuildTime: "unknown", GoVersion: runtime.Version(), Dirty: true}
	}
	if cfg.Build.Version == "" || cfg.Build.Commit == "" || cfg.Build.BuildTime == "" || cfg.Build.GoVersion == "" {
		return nil, fmt.Errorf("application build information is incomplete")
	}
	return &Application{config: cfg}, nil
}

func (b BuildInfo) opsModel() opsmodel.BuildInfo {
	return opsmodel.BuildInfo{
		Version: b.Version, Commit: b.Commit, BuildTime: b.BuildTime, GoVersion: b.GoVersion, Dirty: b.Dirty,
	}
}

// Run 根据参数选择 CLI 或长期 Service。
func (a *Application) Run(ctx context.Context, args []string) error {
	if ctx == nil {
		return fmt.Errorf("application context is nil")
	}
	if len(args) == 0 {
		if err := a.runService(ctx); err != nil {
			return fmt.Errorf("run application service: %w", err)
		}
		return nil
	}
	todoContract, err := todocli.New(todoExecutor{application: a})
	if err != nil {
		return fmt.Errorf("compose todo CLI contract: %w", err)
	}
	migrationContract, err := migrationcli.New(migrationExecutor{application: a})
	if err != nil {
		return fmt.Errorf("compose migration CLI contract: %w", err)
	}
	configuration := []config.Binding{authconfig.Binding(), migrationconfig.Binding(), configbinding.Binding()}
	configuration = append(configuration, opsconfig.Binding(), kernelcomposition.ObservabilityConfiguration())
	bootstrap, err := kernelcomposition.ComposeBootstrap(cli.Config{
		Name:                   a.config.Name,
		Description:            a.config.Description,
		Stdin:                  a.config.Stdin,
		Stdout:                 a.config.Stdout,
		Stderr:                 a.config.Stderr,
		DisableInteractiveHome: true,
	}, kernelcomposition.BootstrapOptions{
		Configuration: configuration,
		Commands:      []kernelcli.Contract{todoContract, migrationContract},
	})
	if err != nil {
		return fmt.Errorf("compose application bootstrap: %w", err)
	}
	if err := bootstrap.CLI.Run(ctx, args); err != nil {
		return fmt.Errorf("run application CLI: %w", err)
	}
	return nil
}
