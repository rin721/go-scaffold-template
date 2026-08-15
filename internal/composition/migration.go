package composition

import (
	"context"
	"fmt"

	databaseapp "github.com/rin721/go-scaffold-template/internal/kernel/app/database"
	kernelcomposition "github.com/rin721/go-scaffold-template/internal/kernel/composition"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	authconfig "github.com/rin721/go-scaffold-template/internal/module/auth/binding/config"
	"github.com/rin721/go-scaffold-template/internal/module/migration"
	migrationconfig "github.com/rin721/go-scaffold-template/internal/module/migration/binding/config"
	todoconfig "github.com/rin721/go-scaffold-template/internal/module/todo/binding/config"
	todomigration "github.com/rin721/go-scaffold-template/internal/module/todo/binding/migration"
)

type migrationExecutor struct{ application *Application }

func (e migrationExecutor) MigrationStatus(ctx context.Context) (migration.Status, error) {
	return e.application.executeMigration(ctx, func(ctx context.Context, service *migration.Service) (migration.Status, error) {
		return service.Status(ctx)
	})
}

func (e migrationExecutor) MigrationUp(ctx context.Context, legacyOwnerSubject string) (migration.Status, error) {
	return e.application.executeMigration(ctx, func(ctx context.Context, service *migration.Service) (migration.Status, error) {
		return service.Up(ctx, legacyOwnerSubject)
	})
}

func (a *Application) executeMigration(
	ctx context.Context,
	operation func(context.Context, *migration.Service) (migration.Status, error),
) (migration.Status, error) {
	if ctx == nil {
		return migration.Status{}, fmt.Errorf("migration application context is nil")
	}
	if operation == nil {
		return migration.Status{}, fmt.Errorf("migration application operation is nil")
	}
	bindings, err := kernelcomposition.ConfigurationBindings(
		authconfig.Binding(), migrationconfig.Binding(), todoconfig.Binding(),
	)
	if err != nil {
		return migration.Status{}, fmt.Errorf("compose migration configuration bindings: %w", err)
	}
	loader := config.New(
		config.FileSource(a.config.ConfigPath),
		config.EnvSource(a.config.EnvironmentPrefix),
	)
	snapshot, err := loader.Load(ctx)
	if err != nil {
		return migration.Status{}, fmt.Errorf("load migration configuration: %w", err)
	}
	if err := config.ValidateCandidate(snapshot, bindings...); err != nil {
		return migration.Status{}, fmt.Errorf("validate migration configuration: %w", err)
	}
	databaseConfig, err := databaseapp.Decode(snapshot)
	if err != nil {
		return migration.Status{}, fmt.Errorf("decode migration database configuration: %w", err)
	}
	moduleConfig, err := migrationconfig.Decode(snapshot)
	if err != nil {
		return migration.Status{}, err
	}
	completion, err := todomigration.NewCompletion(databaseConfig.PackageConfig())
	if err != nil {
		return migration.Status{}, err
	}
	module, err := migration.NewModule(
		databaseConfig.PackageConfig(), moduleConfig, todomigration.Set(), migration.NewDefaultFactory, completion,
	)
	if err != nil {
		return migration.Status{}, err
	}
	return operation(ctx, module.Service)
}
