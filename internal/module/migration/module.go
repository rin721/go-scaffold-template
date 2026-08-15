// Package migration 编排显式 version/status/up 用例，不拥有任何业务 SQL。
package migration

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/module"
	configbinding "github.com/rin721/go-scaffold-template/internal/module/migration/binding/config"
	"github.com/rin721/go-scaffold-template/pkg/database"
	dbmigrate "github.com/rin721/go-scaffold-template/pkg/database/migrate"
)

const moduleID module.ID = "migration"

// Module 是 Migration 模块的局部装配结果。
type Module struct {
	Service      *Service
	Contribution module.Contribution
}

// NewModule 构造不执行 I/O 的 Migration 模块。
func NewModule(databaseConfig database.Config, config configbinding.Config, set dbmigrate.Set, factory Factory, completion Completion) (Module, error) {
	service, err := New(databaseConfig, config, set, factory, completion)
	if err != nil {
		return Module{}, fmt.Errorf("compose migration service: %w", err)
	}
	contribution := module.Contribution{ID: moduleID}
	if err := module.ValidateContributions(contribution); err != nil {
		return Module{}, fmt.Errorf("validate migration contribution: %w", err)
	}
	return Module{Service: service, Contribution: contribution}, nil
}
