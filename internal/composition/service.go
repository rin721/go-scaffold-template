package composition

import (
	"context"
	"errors"
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel"
	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	kernelcomposition "github.com/rin721/go-scaffold-template/internal/kernel/composition"
	"github.com/rin721/go-scaffold-template/internal/module"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"github.com/rin721/go-scaffold-template/pkg/logger"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

func (a *Application) runService(ctx context.Context) error {
	prepared, err := a.prepareTodo(ctx)
	if err != nil {
		return err
	}
	httpConfig, err := kernelcomposition.HTTPServerConfig(prepared.candidate)
	if err != nil {
		return fmt.Errorf("compose HTTP configuration: %w", err)
	}
	router, err := applicationRouter(prepared.capabilities, prepared.module.Contribution)
	if err != nil {
		return err
	}
	httpServer, err := httpx.NewServer(&httpConfig, router)
	if err != nil {
		return fmt.Errorf("compose HTTP server: %w", err)
	}
	participants := make([]supervisor.Participant, 0, len(prepared.module.Contribution.Participants)+2)
	participants = append(participants, prepared.module.Contribution.Participants...)
	participants = append(participants, applicationLifecycle{
		applicationName: a.config.Name,
		logging:         prepared.capabilities.Logger,
	}, httpServer)
	host, err := kernel.NewHost(
		prepared.coordinator,
		serviceHostOptions(prepared.capabilities.Logger, httpServer),
		participants...,
	)
	if err != nil {
		return fmt.Errorf("create application host: %w", err)
	}
	if err := host.Run(ctx); err != nil {
		return fmt.Errorf("run application host: %w", err)
	}
	return nil
}

func applicationRouter(capabilities kernelcomposition.Capabilities, contributions ...module.Contribution) (httpx.Router, error) {
	if err := module.ValidateContributions(contributions...); err != nil {
		return nil, fmt.Errorf("validate module contributions: %w", err)
	}
	router := httpx.NewRouter(nil)
	router.Use(
		httpx.Recovery(capabilities.Logger),
		httpx.RequestID(capabilities.IDGenerator),
		httpx.AccessLog(capabilities.Logger),
		httpx.SecureHeaders(),
	)
	for _, contribution := range contributions {
		for _, route := range contribution.Routes {
			router.Handle(route.Method, route.Path, route.Handler, route.Middlewares...)
		}
	}
	return router, nil
}

func serviceHostOptions(logging logger.Logger, server *httpx.Server) kernel.HostOptions {
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

func reloadErrorReporter(logging logger.Logger) func(error) {
	return func(err error) {
		if logging == nil || err == nil {
			return
		}
		var committed *kernel.CommittedCleanupError
		fields := []logger.Field{logger.String("error_type", fmt.Sprintf("%T", err))}
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
