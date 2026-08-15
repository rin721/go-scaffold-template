package composition

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel"
	kernelcomposition "github.com/rin721/go-scaffold-template/internal/kernel/composition"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	"github.com/rin721/go-scaffold-template/internal/module/todo"
	configbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/config"
	"github.com/rin721/go-scaffold-template/internal/module/todo/model"
	"github.com/rin721/go-scaffold-template/internal/module/todo/service"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

type preparedTodo struct {
	coordinator  *kernel.Coordinator
	capabilities kernelcomposition.Capabilities
	module       todo.Module
	candidate    config.Snapshot
}

func (a *Application) prepareTodo(ctx context.Context) (preparedTodo, error) {
	loader := config.New(
		config.FileSource(a.config.ConfigPath),
		config.EnvSource(a.config.EnvironmentPrefix),
	)
	runtime, err := kernel.New(loader, kernel.Options{Logging: a.config.Logging})
	if err != nil {
		return preparedTodo{}, fmt.Errorf("create kernel: %w", err)
	}
	capabilities, err := kernelcomposition.Compose(runtime, kernelcomposition.Options{
		Logger: kernelcomposition.ConfiguredLoggerReplacement,
	})
	if err != nil {
		return preparedTodo{}, fmt.Errorf("compose application capabilities: %w", err)
	}
	// CLI 与 Service 共用正式配置文件，因此两种模式都声明 application-owned 配置节。
	// 这里只注册 HTTP 配置契约，不会构造 listener、Host 或 watcher。
	bindings := []config.Binding{kernelcomposition.HTTPConfiguration(), configbinding.Binding()}
	coordinator, err := kernel.NewCoordinator(runtime, bindings...)
	if err != nil {
		return preparedTodo{}, fmt.Errorf("create configuration coordinator: %w", err)
	}
	candidate, err := coordinator.Prepare(ctx)
	if err != nil {
		return preparedTodo{}, fmt.Errorf("prepare application configuration: %w", err)
	}
	todoConfig, err := configbinding.Decode(candidate)
	if err != nil {
		return preparedTodo{}, err
	}
	databaseAccess, err := adaptDatabaseAccess(capabilities.Database)
	if err != nil {
		return preparedTodo{}, err
	}
	module, err := todo.New(todo.Dependencies{
		Database: databaseAccess, Clock: capabilities.Clock, IDGenerator: capabilities.IDGenerator,
		Config: todoConfig,
	})
	if err != nil {
		return preparedTodo{}, fmt.Errorf("compose todo module: %w", err)
	}
	return preparedTodo{
		coordinator: coordinator, capabilities: capabilities, module: module, candidate: candidate,
	}, nil
}

func (a *Application) executeTodo(ctx context.Context, operation func(service.UseCases) error) error {
	if operation == nil {
		return fmt.Errorf("todo application operation is nil")
	}
	prepared, err := a.prepareTodo(ctx)
	if err != nil {
		return err
	}
	participants := make([]supervisor.Participant, 0, len(prepared.module.Contribution.Participants)+1)
	participants = append(participants, prepared.coordinator)
	participants = append(participants, prepared.module.Contribution.Participants...)
	owner, err := newTodoOperationSupervisor(participants)
	if err != nil {
		return fmt.Errorf("create todo operation supervisor: %w", err)
	}
	if err := owner.RunOperation(ctx, func(context.Context) error {
		return operation(prepared.module.Service)
	}); err != nil {
		return fmt.Errorf("execute todo application operation: %w", err)
	}
	return nil
}

func newTodoOperationSupervisor(participants []supervisor.Participant) (*supervisor.Supervisor, error) {
	return supervisor.New(supervisor.Config{}, participants...)
}

type todoExecutor struct{ application *Application }

func (e todoExecutor) Create(ctx context.Context, command service.CreateCommand) (model.Todo, error) {
	var result model.Todo
	err := e.application.executeTodo(ctx, func(useCases service.UseCases) error {
		var err error
		result, err = useCases.Create(ctx, command)
		return err
	})
	return result, err
}

func (e todoExecutor) Get(ctx context.Context, query service.GetQuery) (model.Todo, error) {
	var result model.Todo
	err := e.application.executeTodo(ctx, func(useCases service.UseCases) error {
		var err error
		result, err = useCases.Get(ctx, query)
		return err
	})
	return result, err
}

func (e todoExecutor) List(ctx context.Context, query service.ListQuery) (service.ListResult, error) {
	var result service.ListResult
	err := e.application.executeTodo(ctx, func(useCases service.UseCases) error {
		var err error
		result, err = useCases.List(ctx, query)
		return err
	})
	return result, err
}

func (e todoExecutor) Complete(ctx context.Context, command service.CompleteCommand) (model.Todo, error) {
	var result model.Todo
	err := e.application.executeTodo(ctx, func(useCases service.UseCases) error {
		var err error
		result, err = useCases.Complete(ctx, command)
		return err
	})
	return result, err
}
