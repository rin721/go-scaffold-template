// Package composition 是进程唯一的 application composition root。
package composition

import (
	"context"
	"fmt"
	"io"

	kernelcli "github.com/rin721/go-scaffold-template/internal/kernel/cli"
	kernelcomposition "github.com/rin721/go-scaffold-template/internal/kernel/composition"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	"github.com/rin721/go-scaffold-template/internal/kernel/logging"
	authconfig "github.com/rin721/go-scaffold-template/internal/module/auth/binding/config"
	migrationcli "github.com/rin721/go-scaffold-template/internal/module/migration/binding/cli"
	migrationconfig "github.com/rin721/go-scaffold-template/internal/module/migration/binding/config"
	todocli "github.com/rin721/go-scaffold-template/internal/module/todo/binding/cli"
	configbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/config"
	"github.com/rin721/go-scaffold-template/pkg/cli"
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
}

// Application 按参数选择 Bootstrap/Application CLI 或长期 Service 模式。
type Application struct{ config Config }

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
	return &Application{config: cfg}, nil
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
	bootstrap, err := kernelcomposition.ComposeBootstrap(cli.Config{
		Name:                   a.config.Name,
		Description:            a.config.Description,
		Stdin:                  a.config.Stdin,
		Stdout:                 a.config.Stdout,
		Stderr:                 a.config.Stderr,
		DisableInteractiveHome: true,
	}, kernelcomposition.BootstrapOptions{
		Configuration: []config.Binding{authconfig.Binding(), migrationconfig.Binding(), configbinding.Binding()},
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
