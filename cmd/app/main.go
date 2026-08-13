package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/rin721/go-scaffold2/internal/kernel"
	builtincli "github.com/rin721/go-scaffold2/internal/kernel/builtin/cli"
	builtinconfig "github.com/rin721/go-scaffold2/internal/kernel/builtin/config"
	"github.com/rin721/go-scaffold2/internal/kernel/composition"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	"github.com/rin721/go-scaffold2/pkg/cli"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
	"github.com/rin721/go-scaffold2/pkg/supervisor"
)

const (
	applicationName        = "go-scaffold2"
	applicationDescription = "Go 后端服务与 CLI 工具基础设施脚手架"
	defaultConfigPath      = "config.yaml"
	environmentPrefix      = "APP_"
)

func main() {
	os.Exit(runMain(os.Stdin, os.Stdout, os.Stderr, os.Args[1:]))
}

func runMain(stdin io.Reader, stdout, stderr io.Writer, args []string) int {
	ctx, stop := supervisor.SignalContext(context.Background())
	defer stop()

	return execute(ctx, newProcess(stdin, stdout, stderr), args)
}

// process 保存应用入口拥有的固定装配参数和标准流。
//
// Kernel、CLI 和业务能力仍由各自包拥有；这里仅选择配置来源、组合清单和运行模式。
type process struct {
	configPath        string
	environmentPrefix string
	stdin             io.Reader
	stdout            io.Writer
	stderr            io.Writer
}

func newProcess(stdin io.Reader, stdout, stderr io.Writer) process {
	return process{
		configPath:        defaultConfigPath,
		environmentPrefix: environmentPrefix,
		stdin:             stdin,
		stdout:            stdout,
		stderr:            stderr,
	}
}

// run 根据参数选择启动前 CLI 或长期服务模式。
//
// 有参数时只执行 CLI，允许在配置文件尚不存在时运行 config init；无参数时才加载
// 文件与环境变量、启动 Kernel，并等待进程信号触发优雅退出。
func (p process) run(ctx context.Context, args []string) error {
	if ctx == nil {
		return fmt.Errorf("application context is nil")
	}
	assemblyOptions := kernel.AssemblyOptions{Config: builtinconfig.Options{Sources: []config.Source{
		config.FileSource(p.configPath), config.EnvSource(p.environmentPrefix),
	}}}
	if len(args) > 0 {
		assemblyOptions.CLI = &builtincli.Options{App: cli.Config{
			Name:                   applicationName,
			Description:            applicationDescription,
			Stdin:                  p.stdin,
			Stdout:                 p.stdout,
			Stderr:                 p.stderr,
			DisableInteractiveHome: true,
		}}
	}
	assembly, err := kernel.NewAssembly(assemblyOptions)
	if err != nil {
		return fmt.Errorf("create kernel assembly: %w", err)
	}
	capabilities, err := composition.Compose(assembly)
	if err != nil {
		return fmt.Errorf("compose application capabilities: %w", err)
	}

	if len(args) > 0 {
		if capabilities.CLI == nil {
			return fmt.Errorf("application CLI is nil")
		}
		runErr := capabilities.CLI.Run(ctx, args)
		stopErr := capabilities.Runtime.Stop(context.WithoutCancel(ctx))
		if runErr != nil {
			runErr = fmt.Errorf("run application CLI: %w", runErr)
		}
		if stopErr != nil {
			stopErr = fmt.Errorf("stop application kernel: %w", stopErr)
		}
		return errors.Join(runErr, stopErr)
	}

	host, err := kernel.NewHost(capabilities.Runtime, kernel.HostOptions{}, applicationLifecycle{logging: capabilities.Logger})
	if err != nil {
		return fmt.Errorf("create application host: %w", err)
	}
	if err := host.Run(ctx); err != nil {
		return fmt.Errorf("run application host: %w", err)
	}
	return nil
}

type applicationLifecycle struct {
	logging pkglogger.Access
}

func (applicationLifecycle) Name() string { return "application" }

func (l applicationLifecycle) Start(ctx context.Context) error {
	return l.write(ctx, "application started")
}

func (l applicationLifecycle) Stop(ctx context.Context) error {
	return l.write(ctx, "application stopping")
}

func (l applicationLifecycle) write(ctx context.Context, message string) error {
	if l.logging == nil {
		return fmt.Errorf("application logger access is nil")
	}
	return l.logging.Use(ctx, func(log pkglogger.Logger) error {
		log.Info(message, pkglogger.String("application", applicationName))
		return nil
	})
}

func execute(ctx context.Context, process process, args []string) int {
	err := process.run(ctx, args)
	if err == nil {
		return cli.ExitSuccess
	}
	if _, writeErr := fmt.Fprintf(process.stderr, "%s: %v\n", applicationName, err); writeErr != nil {
		return cli.ExitError
	}
	return cli.GetExitCode(err)
}
