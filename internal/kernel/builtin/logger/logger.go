// Package logger 声明 Kernel 默认 Logger baseline 组件。
package logger

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

const Role app.RoleID = "kernel.logger"

// Options 声明 baseline Logger 的固定配置。
type Options struct{ Config *pkglogger.Config }

// Definition 返回 Bootstrap 必选、AppVisible、可事务替换的 Logger baseline 声明。
func Definition(options Options) (app.BuiltinDefinition[pkglogger.Logger, pkglogger.Access], error) {
	var cfg *pkglogger.Config
	if options.Config != nil {
		copied := *options.Config
		copied.OutputPaths = append([]string(nil), options.Config.OutputPaths...)
		copied.ErrorOutputPaths = append([]string(nil), options.Config.ErrorOutputPaths...)
		cfg = &copied
	}
	return app.RuntimeBuiltin(
		Role, app.Bootstrap, app.RequiredActivation, app.AppVisible, app.AssemblyOwnedBaseline,
		func() (pkglogger.Logger, error) { return pkglogger.New(cfg) },
		app.Leased(func(lease app.Lease[pkglogger.Logger]) (pkglogger.Access, error) {
			if lease == nil {
				return nil, fmt.Errorf("builtin logger lease is nil")
			}
			return &access{lease: lease}, nil
		}),
		func(_ context.Context, current pkglogger.Logger) error {
			resource, ok := current.(pkglogger.Resource)
			if !ok {
				return fmt.Errorf("builtin logger does not own a resource")
			}
			return resource.Close()
		},
	)
}

type access struct{ lease app.Lease[pkglogger.Logger] }

func (a *access) Use(ctx context.Context, use func(pkglogger.Logger) error) error {
	if ctx == nil {
		return app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if use == nil {
		return fmt.Errorf("logger access callback is nil")
	}
	return a.lease.Use(ctx, use)
}

var _ pkglogger.Access = (*access)(nil)
