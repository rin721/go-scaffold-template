// Package migration 编排显式 version/status/up 用例，不拥有任何业务 SQL。
package migration

import (
	"context"
	"errors"
	"fmt"

	configbinding "github.com/rin721/go-scaffold-template/internal/module/migration/binding/config"
	"github.com/rin721/go-scaffold-template/pkg/database"
	dbmigrate "github.com/rin721/go-scaffold-template/pkg/database/migrate"
)

// ErrCompletionRequired 表示 version 已到 target，但模块数据仍需显式完成。
var ErrCompletionRequired = dbmigrate.ErrCompletionRequired

// Status 是 CLI 与 readiness 共用的项目状态。
type Status struct {
	Current    uint
	Target     uint
	Dirty      bool
	Empty      bool
	Compatible bool
}

// Runner 是 Service 使用方定义的通用迁移执行端口。
type Runner interface {
	Status(context.Context) (dbmigrate.Status, error)
	Up(context.Context) error
	Close() error
}

// Factory 为每次 one-shot operation 创建独占资源。
type Factory func(context.Context, dbmigrate.Config, dbmigrate.Set) (Runner, error)

// Completion 是业务模块拥有的 migration 数据完成门禁。
type Completion interface {
	Resolve(context.Context, string) error
	Verify(context.Context) error
}

// Service 执行一个 module-owned migration set。
type Service struct {
	database   database.Config
	config     configbinding.Config
	set        dbmigrate.Set
	factory    Factory
	completion Completion
}

// New 构造无 I/O 的 Migration Service。
func New(databaseConfig database.Config, config configbinding.Config, set dbmigrate.Set, factory Factory, completion Completion) (*Service, error) {
	if err := database.ValidateConfig(&databaseConfig); err != nil {
		return nil, fmt.Errorf("validate migration database config: %w", err)
	}
	if config.LockTimeout <= 0 || config.OperationTimeout <= 0 || config.LockTimeout >= config.OperationTimeout {
		return nil, fmt.Errorf("migration service budgets are invalid")
	}
	if set.Name == "" || set.CurrentVersion == 0 || factory == nil || completion == nil {
		return nil, fmt.Errorf("migration service dependencies are incomplete")
	}
	return &Service{database: databaseConfig, config: config, set: set, factory: factory, completion: completion}, nil
}

// Status 读取当前状态并计算是否与 module target 精确兼容。
func (s *Service) Status(ctx context.Context) (Status, error) {
	if ctx == nil {
		return Status{}, fmt.Errorf("migration status context is nil")
	}
	operationCtx, cancel := context.WithTimeout(ctx, s.config.OperationTimeout)
	defer cancel()
	current, err := dbmigrate.ReadStatus(operationCtx, s.database, s.set)
	if err != nil {
		return Status{}, err
	}
	result := Status{
		Current: current.Version, Target: s.set.CurrentVersion,
		Dirty: current.Dirty, Empty: current.Empty,
	}
	result.Compatible = !result.Empty && !result.Dirty && result.Current == result.Target
	if result.Compatible {
		if err := s.completion.Verify(operationCtx); err != nil {
			if errors.Is(err, ErrCompletionRequired) {
				result.Compatible = false
				return result, nil
			}
			return Status{}, err
		}
	}
	return result, nil
}

// Up 应用全部待执行 migration，并要求最终状态精确命中 target。
func (s *Service) Up(ctx context.Context, legacyOwnerSubject string) (Status, error) {
	var result Status
	err := s.withRunner(ctx, func(operationCtx context.Context, runner Runner) error {
		if err := runner.Up(operationCtx); err != nil {
			return err
		}
		if err := s.completion.Resolve(operationCtx, legacyOwnerSubject); err != nil {
			return err
		}
		current, err := runner.Status(operationCtx)
		if err != nil {
			return err
		}
		result = Status{
			Current: current.Version, Target: s.set.CurrentVersion,
			Dirty: current.Dirty, Empty: current.Empty,
		}
		result.Compatible = !result.Empty && !result.Dirty && result.Current == result.Target
		if !result.Compatible {
			return fmt.Errorf("migration finished at incompatible version")
		}
		return nil
	})
	return result, err
}

func (s *Service) withRunner(ctx context.Context, use func(context.Context, Runner) error) error {
	if ctx == nil {
		return fmt.Errorf("migration operation context is nil")
	}
	operationCtx, cancel := context.WithTimeout(ctx, s.config.OperationTimeout)
	defer cancel()
	runner, err := s.factory(operationCtx, dbmigrate.Config{
		Database: s.database, LockTimeout: s.config.LockTimeout,
	}, s.set)
	if err != nil {
		return err
	}
	operationErr := use(operationCtx, runner)
	closeErr := runner.Close()
	return errors.Join(operationErr, closeErr)
}

// Compatible 使用 fresh runner 执行 service startup 的只读版本门禁。
func (s *Service) Compatible(ctx context.Context) error {
	status, err := s.Status(ctx)
	if err != nil {
		return err
	}
	if status.Dirty {
		return fmt.Errorf("migration database is dirty at version %d", status.Current)
	}
	if status.Empty || status.Current < status.Target {
		return fmt.Errorf("migration version is too old: current %d target %d", status.Current, status.Target)
	}
	if status.Current > status.Target {
		return fmt.Errorf("migration version is too new: current %d target %d", status.Current, status.Target)
	}
	if !status.Compatible {
		return ErrCompletionRequired
	}
	return nil
}

// NewDefaultFactory 返回生产 composition 使用的 golang-migrate Adapter 构造器。
func NewDefaultFactory(ctx context.Context, config dbmigrate.Config, set dbmigrate.Set) (Runner, error) {
	return dbmigrate.New(ctx, config, set)
}
