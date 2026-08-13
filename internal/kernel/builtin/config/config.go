// Package config 声明 Kernel 默认 Config baseline 组件。
package config

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	kernelconfig "github.com/rin721/go-scaffold2/internal/kernel/config"
)

const Role app.RoleID = "kernel.config"

// Options 声明由 Assembly 固定选择的配置来源。
type Options struct{ Sources []kernelconfig.Source }

// Definition 返回 Bootstrap 必选、KernelOnly 的 Config baseline 声明。
func Definition(options Options) (app.BuiltinDefinition[kernelconfig.Provider, kernelconfig.Provider], error) {
	sources := append([]kernelconfig.Source(nil), options.Sources...)
	for index, source := range sources {
		if source == nil {
			return app.BuiltinDefinition[kernelconfig.Provider, kernelconfig.Provider]{}, fmt.Errorf("builtin config source %d is nil", index)
		}
	}
	return app.StartupBuiltin(
		Role, app.Bootstrap, app.RequiredActivation, app.KernelOnly, app.AssemblyOwnedBaseline,
		func() (kernelconfig.Provider, error) { return kernelconfig.New(sources...), nil },
		func(provider kernelconfig.Provider) (kernelconfig.Provider, error) { return provider, nil },
		func(context.Context, kernelconfig.Provider) error { return nil },
	)
}
