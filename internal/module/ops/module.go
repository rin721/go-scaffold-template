// Package ops 收口 management、健康探针、构建信息与诊断用例。
package ops

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rin721/go-scaffold-template/internal/module"
	configbinding "github.com/rin721/go-scaffold-template/internal/module/ops/binding/config"
	httpbinding "github.com/rin721/go-scaffold-template/internal/module/ops/binding/http"
	"github.com/rin721/go-scaffold-template/internal/module/ops/middleware"
	"github.com/rin721/go-scaffold-template/internal/module/ops/model"
	"github.com/rin721/go-scaffold-template/internal/module/ops/service"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	pkgobservability "github.com/rin721/go-scaffold-template/pkg/observability"
)

const moduleID module.ID = "ops"

// Dependencies 是 composition 连接给 Ops module 的稳定进程能力。
type Dependencies struct {
	Runtime       service.RuntimeSource
	Build         model.BuildInfo
	Config        configbinding.Config
	Observability pkgobservability.Capabilities
	Access        httpbinding.Access
	Operations    []pkgobservability.Operation
}

// Module 是 Ops 局部装配后交给 composition 的完整成果。
type Module struct {
	Service        *service.Service
	ManagementHTTP http.Handler
	HTTPMiddleware func(http.Handler) http.Handler
	Management     httpx.ServerConfig
	Contribution   module.Contribution
}

// New 构造 generation-owned Ops module，不绑定物理 listener。
func New(ctx context.Context, dependencies Dependencies) (Module, error) {
	if ctx == nil {
		return Module{}, fmt.Errorf("ops module context is nil")
	}
	opsService, err := service.New(dependencies.Runtime, dependencies.Build)
	if err != nil {
		return Module{}, fmt.Errorf("compose ops service: %w", err)
	}
	if dependencies.Observability.Metrics == nil || dependencies.Observability.Telemetry == nil {
		return Module{}, fmt.Errorf("ops observability capabilities are incomplete")
	}
	managementHTTP, err := httpbinding.New(opsService, dependencies.Observability.Metrics.Handler(), dependencies.Access, dependencies.Config.Management.MetricsAccess)
	if err != nil {
		return Module{}, fmt.Errorf("compose management HTTP: %w", err)
	}
	httpMiddleware := dependencies.Observability.Telemetry.HTTP(dependencies.Operations)
	management := dependencies.Config.Management
	managementHTTP = middleware.Management(managementHTTP, management.RequestTimeout, management.MaxRequestBodyBytes, management.MaxInFlight)
	serverConfig := httpx.ServerConfig{
		Addr: management.Addr, ReadHeaderTimeout: management.ReadHeaderTimeout,
		ReadTimeout: management.ReadTimeout, WriteTimeout: management.WriteTimeout,
		IdleTimeout: management.IdleTimeout, RequestTimeout: management.RequestTimeout,
		MaxHeaderBytes: management.MaxHeaderBytes, MaxRequestBodyBytes: management.MaxRequestBodyBytes,
		MaxInFlight: management.MaxInFlight,
	}
	if err := httpx.ValidateServerConfig(&serverConfig); err != nil {
		return Module{}, fmt.Errorf("validate management HTTP server: %w", err)
	}
	contribution := module.Contribution{ID: moduleID}
	if err := module.ValidateContributions(contribution); err != nil {
		return Module{}, fmt.Errorf("validate ops contribution: %w", err)
	}
	return Module{Service: opsService, ManagementHTTP: managementHTTP, HTTPMiddleware: httpMiddleware, Management: serverConfig, Contribution: contribution}, nil
}
