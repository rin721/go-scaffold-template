package composition

import (
	"context"
	"errors"
	"fmt"

	databaseapp "github.com/rin721/go-scaffold-template/internal/kernel/app/database"
	kernelcomposition "github.com/rin721/go-scaffold-template/internal/kernel/composition"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	"github.com/rin721/go-scaffold-template/internal/module/migration"
	migrationconfig "github.com/rin721/go-scaffold-template/internal/module/migration/binding/config"
	todomigration "github.com/rin721/go-scaffold-template/internal/module/todo/binding/migration"
	"github.com/rin721/go-scaffold-template/pkg/logger"
)

type migrationExecutor struct{ application *Application }

func (e migrationExecutor) MigrationStatus(ctx context.Context) (migration.Status, error) {
	return e.application.executeMigration(ctx, "db.migrate.status", func(ctx context.Context, service *migration.Service) (migration.Status, error) {
		return service.Status(ctx)
	})
}

func (e migrationExecutor) MigrationUp(ctx context.Context, legacyOwnerSubject string) (migration.Status, error) {
	return e.application.executeMigration(ctx, "db.migrate.up", func(ctx context.Context, service *migration.Service) (migration.Status, error) {
		return service.Up(ctx, legacyOwnerSubject)
	})
}

func (a *Application) executeMigration(
	ctx context.Context,
	operationName string,
	operation func(context.Context, *migration.Service) (migration.Status, error),
) (migration.Status, error) {
	if ctx == nil {
		return migration.Status{}, fmt.Errorf("migration application context is nil")
	}
	if operationName == "" {
		return migration.Status{}, fmt.Errorf("migration application operation name is empty")
	}
	if operation == nil {
		return migration.Status{}, fmt.Errorf("migration application operation is nil")
	}
	logging := a.config.Logging.Logger()
	logMigrationStarted(logging, operationName)
	bindings, err := kernelcomposition.ConfigurationBindings(applicationOwnedConfigurationBindings()...)
	if err != nil {
		logMigrationFailed(logging, operationName, "compose-config", err)
		return migration.Status{}, fmt.Errorf("compose migration configuration bindings: %w", err)
	}
	loader := config.New(
		config.FileSource(a.config.ConfigPath),
		config.EnvSource(a.config.EnvironmentPrefix),
	)
	snapshot, err := loader.Load(ctx)
	if err != nil {
		logMigrationFailed(logging, operationName, "load-config", err)
		return migration.Status{}, fmt.Errorf("load migration configuration: %w", err)
	}
	if err := config.ValidateCandidate(snapshot, bindings...); err != nil {
		logMigrationFailed(logging, operationName, "validate-config", err)
		return migration.Status{}, fmt.Errorf("validate migration configuration: %w", err)
	}
	databaseConfig, err := databaseapp.Decode(snapshot)
	if err != nil {
		logMigrationFailed(logging, operationName, "decode-database", err)
		return migration.Status{}, fmt.Errorf("decode migration database configuration: %w", err)
	}
	moduleConfig, err := migrationconfig.Decode(snapshot)
	if err != nil {
		logMigrationFailed(logging, operationName, "decode-migration", err)
		return migration.Status{}, err
	}
	completion, err := todomigration.NewCompletion(databaseConfig.PackageConfig())
	if err != nil {
		logMigrationFailed(logging, operationName, "compose-completion", err)
		return migration.Status{}, err
	}
	module, err := migration.NewModule(
		databaseConfig.PackageConfig(), moduleConfig, todomigration.Set(), migration.NewDefaultFactory, completion,
	)
	if err != nil {
		logMigrationFailed(logging, operationName, "compose-service", err)
		return migration.Status{}, err
	}
	status, err := operation(ctx, module.Service)
	if err != nil {
		logMigrationFailed(logging, operationName, "run", err)
		return status, err
	}
	logMigrationCompleted(logging, operationName, status)
	return status, nil
}

func logMigrationStarted(logging logger.Logger, operation string) {
	if logging == nil {
		return
	}
	logging.Debug("migration operation started",
		logger.String("owner", "migration"),
		logger.String("phase", "start"),
		logger.String("operation", operation),
	)
}

func logMigrationCompleted(logging logger.Logger, operation string, status migration.Status) {
	if logging == nil {
		return
	}
	fields := []logger.Field{
		logger.String("owner", "migration"),
		logger.String("phase", "completed"),
		logger.String("operation", operation),
		logger.Any("current_version", status.Current),
		logger.Any("target_version", status.Target),
		logger.Bool("dirty", status.Dirty),
		logger.Bool("empty", status.Empty),
		logger.Bool("compatible", status.Compatible),
	}
	if status.Dirty || !status.Compatible {
		logging.Warn("migration operation completed with action required", fields...)
		return
	}
	logging.Info("migration operation completed", fields...)
}

func logMigrationFailed(logging logger.Logger, operation string, phase string, err error) {
	if logging == nil || err == nil {
		return
	}
	logging.Error("migration operation failed",
		logger.String("owner", "migration"),
		logger.String("phase", phase),
		logger.String("operation", operation),
		logger.String("error_type", migrationErrorType(err)),
		logger.String("cause_type", fmt.Sprintf("%T", err)),
	)
}

func migrationErrorType(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "context_deadline_exceeded"
	case errors.Is(err, migration.ErrCompletionRequired):
		return "migration_completion_required"
	default:
		return fmt.Sprintf("%T", err)
	}
}
