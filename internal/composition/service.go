package composition

import (
	"context"
	"errors"
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel"
	kernelcomposition "github.com/rin721/go-scaffold-template/internal/kernel/composition"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	"github.com/rin721/go-scaffold-template/internal/module/auth"
	authconfig "github.com/rin721/go-scaffold-template/internal/module/auth/binding/config"
	authmodel "github.com/rin721/go-scaffold-template/internal/module/auth/model"
	migrationconfig "github.com/rin721/go-scaffold-template/internal/module/migration/binding/config"
	opsconfig "github.com/rin721/go-scaffold-template/internal/module/ops/binding/config"
	opsmodel "github.com/rin721/go-scaffold-template/internal/module/ops/model"
	configbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/config"
	"github.com/rin721/go-scaffold-template/internal/module/todo/service"
	httptransport "github.com/rin721/go-scaffold-template/internal/transport/http"
	httpapi "github.com/rin721/go-scaffold-template/internal/transport/http/api"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"github.com/rin721/go-scaffold-template/pkg/logger"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

func (a *Application) runService(ctx context.Context) error {
	runtime, err := a.newServiceRuntime()
	if err != nil {
		return err
	}
	if err := runtime.supervisor.Run(ctx); err != nil {
		return fmt.Errorf("run application supervisor: %w", err)
	}
	return nil
}

type serviceRuntime struct {
	supervisor  *supervisor.Supervisor
	coordinator *kernel.GenerationCoordinator
	factory     *applicationGenerationFactory
}

func (a *Application) newServiceRuntime() (*serviceRuntime, error) {
	loader := config.New(
		config.FileSource(a.config.ConfigPath),
		config.EnvSource(a.config.EnvironmentPrefix),
	)
	bindingsInput := []config.Binding{authconfig.Binding(), migrationconfig.Binding(), configbinding.Binding()}
	bindingsInput = append(bindingsInput, opsconfig.Bindings()...)
	bindings, err := kernelcomposition.ConfigurationBindings(bindingsInput...)
	if err != nil {
		return nil, fmt.Errorf("compose service configuration bindings: %w", err)
	}
	factory, err := newApplicationGenerationFactory(a.config.Logging)
	if err != nil {
		return nil, fmt.Errorf("create application generation factory: %w", err)
	}
	factory.build = a.config.Build
	coordinator, err := kernel.NewGenerationCoordinator(loader, bindings, factory, kernel.Options{Logging: a.config.Logging})
	if err != nil {
		return nil, fmt.Errorf("create application generation coordinator: %w", err)
	}
	process, err := supervisor.New(supervisor.Config{},
		coordinator,
		applicationLifecycle{applicationName: a.config.Name, logging: a.config.Logging.Logger()},
	)
	if err != nil {
		return nil, fmt.Errorf("create application supervisor: %w", err)
	}
	if err := factory.opsRuntime.connect(coordinator, process); err != nil {
		return nil, fmt.Errorf("connect ops runtime source: %w", err)
	}
	if err := process.AddTask("application-generation.monitor", coordinator.Monitor); err != nil {
		return nil, fmt.Errorf("register application generation monitor: %w", err)
	}
	watchReady := make(chan struct{})
	if err := process.AddRunner(supervisor.Task{
		Name: "application-config-watch", Ready: watchReady,
		Run: func(ctx context.Context) error {
			return coordinator.Watch(ctx, reloadErrorReporter(a.config.Logging.Logger()), watchReady)
		},
	}); err != nil {
		return nil, fmt.Errorf("register application config watcher: %w", err)
	}
	return &serviceRuntime{supervisor: process, coordinator: coordinator, factory: factory}, nil
}

func applicationRouter(capabilities kernelcomposition.Capabilities, httpConfig httpx.ServerConfig, todoService service.UseCases, authModule auth.Module) (httpx.Router, error) {
	todoHandler, err := httptransport.NewTodoHTTPHandler(todoService, capabilities.I18n, authModule.Service)
	if err != nil {
		return nil, fmt.Errorf("compose generated Todo HTTP transport: %w", err)
	}
	trustedProxy, err := httpx.TrustedProxy(httpConfig.TrustedProxyCIDRs)
	if err != nil {
		return nil, fmt.Errorf("compose trusted proxy policy: %w", err)
	}
	rateLimiter := httpx.NewRateLimiterWithBurst(
		httpConfig.RateLimit.RequestsPerSecond,
		httpConfig.RateLimit.Burst,
	)
	overload := httpx.NewOverloadLimiter(httpConfig.MaxInFlight)
	router := httpx.NewRouter(nil)
	router.Use(
		httpx.RequestID(capabilities.IDGenerator),
		httpx.Recovery(capabilities.Logger),
		httpx.AccessLog(capabilities.Logger),
		trustedProxy,
		httpx.SecureHeaders(),
		httpx.RejectUpgrade(),
		httpx.RequestTimeout(httpConfig.RequestTimeout),
		httpx.BodyLimit(httpConfig.MaxRequestBodyBytes),
		httpx.AcceptJSON(),
		httpx.CORS(httpConfig.CORS),
		rateLimiter.Middleware(),
		overload.Middleware(),
	)
	router.UseHTTP(authModule.HTTPMiddleware)
	router.Mount("/", todoHandler)
	return router, nil
}

func operationPolicies() ([]authmodel.Policy, error) {
	operations := httpapi.Operations()
	policies := make([]authmodel.Policy, len(operations))
	for index, operation := range operations {
		mode := authmodel.PolicyMode(operation.Policy)
		policies[index] = authmodel.Policy{
			Operation: string(operation.ID), Mode: mode,
			Scope: authmodel.Scope(operation.Scope), Action: authmodel.Action(operation.Action),
		}
	}
	if len(policies) == 0 {
		return nil, fmt.Errorf("OpenAPI operation policy inventory is empty")
	}
	policies = append(policies,
		authmodel.Policy{Operation: opsmodel.OperationDiagnostics, Mode: authmodel.PolicyProtected, Scope: "management:read", Action: "ops.diagnostics.read"},
		authmodel.Policy{Operation: opsmodel.OperationMetrics, Mode: authmodel.PolicyProtected, Scope: "management:read", Action: "ops.metrics.read"},
	)
	return policies, nil
}

func reloadErrorReporter(logging logger.Logger) func(error) {
	return func(err error) {
		if logging == nil || err == nil {
			return
		}
		var committed *kernel.CommittedCleanupError
		fields := []logger.Field{logger.String("error_type", fmt.Sprintf("%T", err))}
		var operation *kernel.GenerationOperationError
		if errors.As(err, &operation) {
			fields = append(fields,
				logger.String("phase", operation.Phase),
				logger.String("owner", operation.Owner),
				logger.Any("generation", operation.Generation),
				logger.String("cause_type", fmt.Sprintf("%T", operation.Err)),
			)
		}
		if errors.As(err, &committed) {
			logging.Error("application generation reload applied with cleanup debt", fields...)
			return
		}
		logging.Error("application generation reload rejected; previous generation remains active", fields...)
	}
}

type applicationLifecycle struct {
	applicationName string
	logging         logger.Logger
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
	if l.applicationName == "" {
		return fmt.Errorf("application name is empty")
	}
	l.logging.Info(message, logger.String("application", l.applicationName))
	return nil
}
