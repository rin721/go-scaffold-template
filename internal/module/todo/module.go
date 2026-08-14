// Package todo 负责 Todo 模块的局部纯内存装配。
package todo

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/module"
	configbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/config"
	httpbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/http"
	modelbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/model"
	"github.com/rin721/go-scaffold-template/internal/module/todo/handler"
	"github.com/rin721/go-scaffold-template/internal/module/todo/repo"
	"github.com/rin721/go-scaffold-template/internal/module/todo/service"
	"github.com/rin721/go-scaffold-template/pkg/clock"
	"github.com/rin721/go-scaffold-template/pkg/i18n"
	"github.com/rin721/go-scaffold-template/pkg/idgen"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

const moduleID module.ID = "todo"

// Dependencies 是 Todo 模块实际使用的稳定能力。
type Dependencies struct {
	Database    repo.Access
	Clock       clock.Clock
	IDGenerator idgen.Generator
	I18n        i18n.Translator
	Config      configbinding.Config
}

// Module 是 Todo 局部装配的完成结果。
type Module struct {
	Service      service.UseCases
	Contribution module.Contribution
}

// New 纯内存构造 Todo Service、Adapter 与 contribution。
func New(dependencies Dependencies) (Module, error) {
	repository, err := repo.New(dependencies.Database, modelbinding.Schema())
	if err != nil {
		return Module{}, fmt.Errorf("compose todo repository: %w", err)
	}
	todoService, err := service.New(
		repository, dependencies.Clock, dependencies.IDGenerator, dependencies.Config.Policy(),
	)
	if err != nil {
		return Module{}, fmt.Errorf("compose todo service: %w", err)
	}
	todoHandler, err := handler.New(todoService, dependencies.I18n)
	if err != nil {
		return Module{}, fmt.Errorf("compose todo HTTP handler: %w", err)
	}
	routes, err := httpbinding.Routes(todoHandler)
	if err != nil {
		return Module{}, fmt.Errorf("compose todo HTTP routes: %w", err)
	}
	migrator, err := modelbinding.NewMigrator(dependencies.Database)
	if err != nil {
		return Module{}, fmt.Errorf("compose todo schema migrator: %w", err)
	}
	contribution := module.Contribution{
		ID: moduleID, Routes: routes, Participants: []supervisor.Participant{migrator},
	}
	if err := module.ValidateContributions(contribution); err != nil {
		return Module{}, fmt.Errorf("validate todo contribution: %w", err)
	}
	return Module{Service: todoService, Contribution: contribution}, nil
}
