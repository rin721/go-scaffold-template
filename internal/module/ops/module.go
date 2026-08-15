// Package ops 收口 management、健康探针、构建信息、trace 与 metrics。
package ops

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rin721/go-scaffold-template/internal/module"
	oteladapter "github.com/rin721/go-scaffold-template/internal/module/ops/adapter/otel"
	prometheusadapter "github.com/rin721/go-scaffold-template/internal/module/ops/adapter/prometheus"
	configbinding "github.com/rin721/go-scaffold-template/internal/module/ops/binding/config"
	httpbinding "github.com/rin721/go-scaffold-template/internal/module/ops/binding/http"
	"github.com/rin721/go-scaffold-template/internal/module/ops/middleware"
	"github.com/rin721/go-scaffold-template/internal/module/ops/model"
	"github.com/rin721/go-scaffold-template/internal/module/ops/service"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

const moduleID module.ID = "ops"

// Dependencies 是 composition 连接给 Ops module 的稳定进程能力。
type Dependencies struct {
	Runtime    service.RuntimeSource
	Build      model.BuildInfo
	Config     configbinding.Config
	Metrics    *prometheusadapter.Registry
	Access     httpbinding.Access
	Operations []middleware.Operation
}

// Module 是 Ops 局部装配后交给 composition 的完整成果。
type Module struct {
	Service        *service.Service
	ManagementHTTP http.Handler
	HTTPMiddleware func(http.Handler) http.Handler
	Telemetry      *oteladapter.Provider
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
	telemetry, err := oteladapter.New(ctx, dependencies.Config.Observability, dependencies.Metrics)
	if err != nil {
		return Module{}, fmt.Errorf("compose ops telemetry: %w", err)
	}
	managementHTTP, err := httpbinding.New(opsService, dependencies.Metrics.Handler(), dependencies.Access, dependencies.Config.Management.MetricsAccess)
	if err != nil {
		return Module{}, fmt.Errorf("compose management HTTP: %w", err)
	}
	httpMiddleware := middleware.HTTP(telemetry.Tracer(), dependencies.Metrics, dependencies.Operations)
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
	contribution := module.Contribution{ID: moduleID, Participants: []supervisor.Participant{telemetry}}
	if err := module.ValidateContributions(contribution); err != nil {
		return Module{}, fmt.Errorf("validate ops contribution: %w", err)
	}
	return Module{Service: opsService, ManagementHTTP: managementHTTP, HTTPMiddleware: httpMiddleware, Telemetry: telemetry, Management: serverConfig, Contribution: contribution}, nil
}
