// Package cli 声明 Kernel 默认 CLI baseline 组件。
package cli

import (
	"context"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	kernelcli "github.com/rin721/go-scaffold2/internal/kernel/cli"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
)

const Role app.RoleID = "kernel.cli"

// Options 声明选中 CLI 模式时使用的固定 App 配置。
type Options struct{ App pkgcli.Config }

// Definition 返回 PreStart 可选、KernelOnly 的 CLI Factory baseline 声明。
func Definition(options Options) (app.BuiltinDefinition[kernelcli.Factory, pkgcli.App], error) {
	return app.DeferredStartupBuiltin[kernelcli.Factory, pkgcli.App](
		Role, app.PreStart, app.AssemblyOwnedBaseline,
		func() (kernelcli.Factory, error) { return factory{config: options.App}, nil },
		func(context.Context, kernelcli.Factory) error { return nil },
	)
}

type factory struct{ config pkgcli.Config }

func (f factory) Build(contracts []kernelcli.Contract) (pkgcli.App, error) {
	return kernelcli.NewApp(f.config, contracts...)
}

var _ kernelcli.Factory = factory{}
