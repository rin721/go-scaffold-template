// Package todo 负责 Todo 模块的局部纯内存装配。
package todo

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/module"
	configbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/config"
	httpbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/http"
	"github.com/rin721/go-scaffold-template/internal/module/todo/repo"
	"github.com/rin721/go-scaffold-template/internal/module/todo/service"
	"github.com/rin721/go-scaffold-template/pkg/clock"
	"github.com/rin721/go-scaffold-template/pkg/i18n"
	"github.com/rin721/go-scaffold-template/pkg/idgen"
)

const moduleID module.ID = "todo"

// Dependencies 是 Todo 模块实际使用的稳定能力。
type Dependencies struct {
	Database    repo.Access
	Clock       clock.Clock
	IDGenerator idgen.Generator
	Config      configbinding.Config
	Authorizer  service.Authorizer
}

// Module 是 Todo 局部装配的完成结果。
type Module struct {
	Service      service.UseCases
	Contribution module.Contribution
}

// HTTPDependencies 是 Todo HTTP profile 在 core 依赖之外需要的请求边界能力。
type HTTPDependencies struct {
	Dependencies
	Translator i18n.Translator
	Actors     httpbinding.ActorAccess
}

// HTTPModule 是 Todo 长期 Service profile 的完整输出。
type HTTPModule struct {
	Module
	Operations httpbinding.Operations
}

// New 纯内存构造 Todo Service、Adapter 与 contribution。
func New(dependencies Dependencies) (Module, error) {
	repository, err := repo.New(dependencies.Database, repo.Schema())
	if err != nil {
		return Module{}, fmt.Errorf("compose todo repository: %w", err)
	}
	todoService, err := service.New(
		repository, dependencies.Clock, dependencies.IDGenerator, dependencies.Config.Policy(), dependencies.Authorizer,
	)
	if err != nil {
		return Module{}, fmt.Errorf("compose todo service: %w", err)
	}
	contribution := module.Contribution{ID: moduleID}
	if err := module.ValidateContributions(contribution); err != nil {
		return Module{}, fmt.Errorf("validate todo contribution: %w", err)
	}
	return Module{Service: todoService, Contribution: contribution}, nil
}

// NewHTTP 纯内存构造 Todo core 与模块自有 HTTP binding。
func NewHTTP(dependencies HTTPDependencies) (HTTPModule, error) {
	core, err := New(dependencies.Dependencies)
	if err != nil {
		return HTTPModule{}, err
	}
	handler, err := httpbinding.NewHandler(core.Service, dependencies.Translator, dependencies.Actors)
	if err != nil {
		return HTTPModule{}, fmt.Errorf("compose todo HTTP binding: %w", err)
	}
	return HTTPModule{Module: core, Operations: handler}, nil
}
