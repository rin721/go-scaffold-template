package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/app"
	"github.com/rin721/go-scaffold2/internal/kernel/composition"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
	"github.com/rin721/go-scaffold2/pkg/cli"
	pkghttpx "github.com/rin721/go-scaffold2/pkg/httpx"
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

	baseline, err := pkglogger.New(nil)
	if err != nil {
		return reportProcessError(stderr, fmt.Errorf("create baseline logger: %w", err), cli.ExitError)
	}
	manager, err := kernellogging.New(baseline)
	if err != nil {
		closeErr := baseline.Close()
		return reportProcessError(stderr, errors.Join(fmt.Errorf("create kernel logging manager: %w", err), closeErr), cli.ExitError)
	}

	process := newProcess(stdin, stdout, stderr, manager)
	exitCode := execute(ctx, process, args)
	if err := baseline.Close(); err != nil {
		return reportProcessError(stderr, fmt.Errorf("close baseline logger: %w", err), cli.ExitError)
	}
	return exitCode
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
	logging           *kernellogging.Manager
}

func newProcess(stdin io.Reader, stdout, stderr io.Writer, logging *kernellogging.Manager) process {
	return process{
		configPath:        defaultConfigPath,
		environmentPrefix: environmentPrefix,
		stdin:             stdin,
		stdout:            stdout,
		stderr:            stderr,
		logging:           logging,
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
	if p.logging == nil {
		return fmt.Errorf("application logging manager is nil")
	}
	if len(args) > 0 {
		bootstrap, err := composition.ComposeBootstrap(cli.Config{
			Name:                   applicationName,
			Description:            applicationDescription,
			Stdin:                  p.stdin,
			Stdout:                 p.stdout,
			Stderr:                 p.stderr,
			DisableInteractiveHome: true,
		})
		if err != nil {
			return fmt.Errorf("compose application bootstrap: %w", err)
		}
		if err := bootstrap.CLI.Run(ctx, args); err != nil {
			return fmt.Errorf("run application CLI: %w", err)
		}
		return nil
	}

	loader := config.New(
		config.FileSource(p.configPath),
		config.EnvSource(p.environmentPrefix),
	)
	runtime, err := kernel.New(loader, kernel.Options{Logging: p.logging})
	if err != nil {
		return fmt.Errorf("create kernel: %w", err)
	}

	capabilities, err := composition.Compose(runtime, composition.Options{Logger: composition.ConfiguredLoggerReplacement})
	if err != nil {
		return fmt.Errorf("compose application capabilities: %w", err)
	}

	httpBinding := composition.HTTPConfiguration()
	coordinator, err := kernel.NewCoordinator(runtime, httpBinding)
	if err != nil {
		return fmt.Errorf("create configuration coordinator: %w", err)
	}
	candidate, err := coordinator.Prepare(ctx)
	if err != nil {
		return fmt.Errorf("prepare application configuration: %w", err)
	}
	httpConfig, err := composition.HTTPServerConfig(candidate)
	if err != nil {
		return fmt.Errorf("compose HTTP configuration: %w", err)
	}
	httpServer, err := pkghttpx.NewServer(&httpConfig, http.NotFoundHandler())
	if err != nil {
		return fmt.Errorf("compose HTTP server: %w", err)
	}
	host, err := kernel.NewHost(
		coordinator,
		serviceHostOptions(capabilities.Logger, httpServer),
		applicationLifecycle{logging: capabilities.Logger},
		httpServer,
	)
	if err != nil {
		return fmt.Errorf("create application host: %w", err)
	}
	if err := host.Run(ctx); err != nil {
		return fmt.Errorf("run application host: %w", err)
	}
	return nil
}

func serviceHostOptions(logging pkglogger.Logger, server *pkghttpx.Server) kernel.HostOptions {
	options := kernel.HostOptions{
		Watch: &kernel.WatchOptions{OnReloadError: reloadErrorReporter(logging)},
	}
	if server != nil {
		options.Runners = []supervisor.Task{{
			Name: "http-server.serve", Run: server.Run, Ready: server.Running(),
		}}
	}
	return options
}

func reloadErrorReporter(logging pkglogger.Logger) func(error) {
	return func(err error) {
		if logging == nil || err == nil {
			return
		}
		var committed *kernel.CommittedCleanupError
		fields := []pkglogger.Field{pkglogger.String("error_type", fmt.Sprintf("%T", err))}
		switch {
		case errors.As(err, &committed):
			logging.Error("kernel reload applied but previous resources failed to close", fields...)
		case errors.Is(err, app.ErrRestartRequired):
			logging.Warn("kernel reload requires process restart; previous configuration remains active", fields...)
		default:
			logging.Error("kernel reload rejected; previous configuration remains active", fields...)
		}
	}
}

type applicationLifecycle struct {
	logging pkglogger.Logger
}

func (applicationLifecycle) Name() string { return "application" }

func (l applicationLifecycle) Start(ctx context.Context) error {
	return l.write(ctx, "application started")
}

func (l applicationLifecycle) Stop(ctx context.Context) error {
	return l.write(ctx, "application stopping")
}

func (l applicationLifecycle) write(ctx context.Context, message string) error {
	if ctx == nil {
		return fmt.Errorf("application logger context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if l.logging == nil {
		return fmt.Errorf("application logger is nil")
	}
	l.logging.Info(message, pkglogger.String("application", applicationName))
	return nil
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

func reportProcessError(stderr io.Writer, err error, exitCode int) int {
	if _, writeErr := fmt.Fprintf(stderr, "%s: %v\n", applicationName, err); writeErr != nil {
		return cli.ExitError
	}
	return exitCode
}
