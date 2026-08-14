package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	applicationcomposition "github.com/rin721/go-scaffold-template/internal/composition"
	kernellogging "github.com/rin721/go-scaffold-template/internal/kernel/logging"
	"github.com/rin721/go-scaffold-template/pkg/cli"
	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

const (
	applicationName        = "go-scaffold-template"
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
	application, err := applicationcomposition.New(applicationcomposition.Config{
		Name: applicationName, Description: applicationDescription,
		ConfigPath: p.configPath, EnvironmentPrefix: p.environmentPrefix,
		Stdin: p.stdin, Stdout: p.stdout, Stderr: p.stderr, Logging: p.logging,
	})
	if err != nil {
		return fmt.Errorf("compose application: %w", err)
	}
	return application.Run(ctx, args)
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
