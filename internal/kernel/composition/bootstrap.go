package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	cacheapp "github.com/rin721/go-scaffold-template/internal/kernel/app/cache"
	databaseapp "github.com/rin721/go-scaffold-template/internal/kernel/app/database"
	i18napp "github.com/rin721/go-scaffold-template/internal/kernel/app/i18n"
	loggerapp "github.com/rin721/go-scaffold-template/internal/kernel/app/logger"
	storageapp "github.com/rin721/go-scaffold-template/internal/kernel/app/storage"
	kernelcli "github.com/rin721/go-scaffold-template/internal/kernel/cli"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold-template/pkg/cli"
)

// Bootstrap 保存 one-shot 启动命令所需的最小装配结果。
type Bootstrap struct {
	Configuration config.DefaultManager
	CLI           pkgcli.App
}

// BootstrapOptions 显式接收 application-owned 配置节和启动前命令契约。
// 构造这些契约不得创建 Kernel、资源、listener 或 goroutine。
type BootstrapOptions struct {
	Configuration []config.Binding
	Commands      []kernelcli.Contract
}

// ComposeBootstrap 只构造配置节契约、默认配置管理器和命令树。
// 它不会创建 Kernel、稳定能力 facade、资源连接、listener 或 goroutine。
func ComposeBootstrap(cfg pkgcli.Config, options BootstrapOptions) (Bootstrap, error) {
	bindings, err := ConfigurationBindings(options.Configuration...)
	if err != nil {
		return Bootstrap{}, err
	}
	manager, err := config.NewDefaultManager(bindings...)
	if err != nil {
		return Bootstrap{}, fmt.Errorf("compose bootstrap configuration: %w", err)
	}
	contracts := make([]kernelcli.Contract, 0, len(options.Commands)+1)
	contracts = append(contracts, kernelcli.ConfigCommands(manager))
	contracts = append(contracts, options.Commands...)
	command, err := kernelcli.NewApp(cfg, contracts...)
	if err != nil {
		return Bootstrap{}, fmt.Errorf("compose bootstrap CLI: %w", err)
	}
	return Bootstrap{Configuration: manager, CLI: command}, nil
}

// ConfigurationBindings 返回当前完整配置文件的底层与 application-owned 契约。
// 本函数只构造校验元数据，不创建资源、listener 或 goroutine。
func ConfigurationBindings(application ...config.Binding) ([]config.Binding, error) {
	bindings := make([]config.Binding, 0, 6+len(application))
	loggerDefinition, err := loggerapp.Replacement()
	if err != nil {
		return nil, fmt.Errorf("define bootstrap logger configuration: %w", err)
	}
	loggerBinding, ok := app.ReplacementConfiguration(loggerDefinition)
	if !ok {
		return nil, fmt.Errorf("bootstrap logger configuration is missing")
	}
	bindings = append(bindings, loggerBinding)

	databaseDefinition, err := databaseapp.Definition()
	if err != nil {
		return nil, fmt.Errorf("define bootstrap database configuration: %w", err)
	}
	cacheDefinition, err := cacheapp.Definition()
	if err != nil {
		return nil, fmt.Errorf("define bootstrap cache configuration: %w", err)
	}
	i18nDefinition, err := i18napp.Definition()
	if err != nil {
		return nil, fmt.Errorf("define bootstrap i18n configuration: %w", err)
	}
	storageDefinition, err := storageapp.Definition()
	if err != nil {
		return nil, fmt.Errorf("define bootstrap storage configuration: %w", err)
	}
	for _, binding := range []struct {
		value config.Binding
		ok    bool
	}{
		bindingOf(databaseDefinition),
		bindingOf(cacheDefinition),
		bindingOf(i18nDefinition),
		bindingOf(storageDefinition),
	} {
		if !binding.ok {
			return nil, fmt.Errorf("bootstrap component configuration is missing")
		}
		bindings = append(bindings, binding.value)
	}
	bindings = append(bindings, HTTPConfiguration())
	bindings = append(bindings, application...)
	return bindings, nil
}

func bindingOf[O any](definition app.Definition[O]) struct {
	value config.Binding
	ok    bool
} {
	binding, ok := app.Configuration(definition)
	return struct {
		value config.Binding
		ok    bool
	}{value: binding, ok: ok}
}
