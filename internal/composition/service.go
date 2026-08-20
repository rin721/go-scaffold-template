package composition

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/rin721/go-scaffold-template/internal/kernel"
	kernelcomposition "github.com/rin721/go-scaffold-template/internal/kernel/composition"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	authmodel "github.com/rin721/go-scaffold-template/internal/module/auth/model"
	opsmodel "github.com/rin721/go-scaffold-template/internal/module/ops/model"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"github.com/rin721/go-scaffold-template/pkg/logger"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

func (a *Application) runService(ctx context.Context) error {
	logging := a.config.Logging.Logger()
	logging.Debug("application service selected",
		logger.String("application", a.config.Name),
		logger.String("mode", "service"),
		logger.String("phase", "compose"),
	)
	runtime, err := a.newServiceRuntime()
	if err != nil {
		reportServiceFailure(logging, "compose", err)
		return err
	}
	logging.Debug("application service runtime composed",
		logger.String("application", a.config.Name),
		logger.String("mode", "service"),
		logger.String("phase", "start"),
	)
	if err := runtime.supervisor.Run(ctx); err != nil {
		if !expectedServiceShutdown(ctx, err) {
			reportServiceFailure(logging, "run", err)
		}
		return fmt.Errorf("run application supervisor: %w", err)
	}
	logging.Info("application stopped", logger.String("application", a.config.Name))
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
	bindings, err := kernelcomposition.ConfigurationBindings(applicationOwnedConfigurationBindings()...)
	if err != nil {
		return nil, fmt.Errorf("compose service configuration bindings: %w", err)
	}
	factory, err := newApplicationGenerationFactory(a.config.Logging, a.config.Name)
	if err != nil {
		return nil, fmt.Errorf("create application generation factory: %w", err)
	}
	factory.build = a.config.Build.opsModel()
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

func applicationRouter(
	capabilities kernelcomposition.Capabilities,
	httpConfig httpx.ServerConfig,
	authMiddleware func(http.Handler) http.Handler,
	apiRoutes http.Handler,
) (httpx.Router, error) {
	if authMiddleware == nil {
		return nil, fmt.Errorf("application auth HTTP middleware is nil")
	}
	if apiRoutes == nil {
		return nil, fmt.Errorf("application API routes are nil")
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
	router.UseHTTP(authMiddleware)
	router.Mount("/", apiRoutes)
	return router, nil
}

func operationPolicies() ([]authmodel.Policy, error) {
	modules := applicationHTTPModules()
	policies := make([]authmodel.Policy, 0, 8)
	for _, module := range modules {
		for _, operation := range module.Operations {
			mode := authmodel.PolicyMode(operation.Policy.Mode)
			policies = append(policies, authmodel.Policy{
				Operation: string(operation.ID), Mode: mode,
				Scope: authmodel.Scope(operation.Policy.Scope), Action: authmodel.Action(operation.Policy.Action),
			})
		}
	}
	if len(policies) == 0 {
		return nil, fmt.Errorf("module operation policy inventory is empty")
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
		logging.Warn("application generation reload rejected; previous generation remains active", fields...)
	}
}

func reportServiceFailure(logging logger.Logger, phase string, err error) {
	if logging == nil || err == nil {
		return
	}
	fields := []logger.Field{
		logger.String("owner", "application"),
		logger.String("phase", phase),
		logger.String("error_type", fmt.Sprintf("%T", err)),
	}
	var operation *kernel.GenerationOperationError
	if errors.As(err, &operation) {
		fields = append(fields,
			logger.String("generation_phase", operation.Phase),
			logger.String("generation_owner", operation.Owner),
			logger.Any("generation", operation.Generation),
			logger.String("cause_type", fmt.Sprintf("%T", operation.Err)),
		)
	}
	logging.Error("application service failed", fields...)
}

func expectedServiceShutdown(ctx context.Context, err error) bool {
	if ctx == nil || ctx.Err() == nil {
		return false
	}
	return errors.Is(err, ctx.Err()) || errors.Is(err, context.Canceled)
}

type applicationLifecycle struct {
	applicationName string
	logging         logger.Logger
}

func (applicationLifecycle) Name() string { return "application" }

func (l applicationLifecycle) Start(ctx context.Context) error {
	return l.write(ctx, "application ready")
}

func (l applicationLifecycle) Stop(ctx context.Context) error {
	return l.write(ctx, "application draining")
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
