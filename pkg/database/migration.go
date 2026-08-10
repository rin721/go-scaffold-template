package database

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Migration 表示一个可顺序执行的数据库迁移。
type Migration struct {
	Version string
	Name    string
	Up      func(context.Context, Executor) error
}

// Migrator 执行迁移集合。
type Migrator struct {
	migrations []Migration
}

// NewMigrator 创建迁移器。
func NewMigrator(migrations ...Migration) (*Migrator, error) {
	seen := map[string]struct{}{}
	for _, migration := range migrations {
		version := strings.TrimSpace(migration.Version)
		if version == "" {
			return nil, fmt.Errorf("migration version is required")
		}
		if migration.Up == nil {
			return nil, fmt.Errorf("migration %s up function is nil", version)
		}
		if _, exists := seen[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %s", version)
		}
		seen[version] = struct{}{}
	}
	return &Migrator{migrations: append([]Migration(nil), migrations...)}, nil
}

// Apply 按注册顺序执行迁移。
func (m *Migrator) Apply(ctx context.Context, executor Executor) error {
	if executor == nil {
		return fmt.Errorf("database executor is nil")
	}
	for _, migration := range m.migrations {
		if err := migration.Up(ctx, executor); err != nil {
			return fmt.Errorf("apply migration %s %s: %w", migration.Version, migration.Name, err)
		}
	}
	return nil
}

// Readiness 执行数据库 readiness 检查。
func Readiness(ctx context.Context, client HealthChecker, timeout time.Duration) error {
	if client == nil {
		return fmt.Errorf("database health checker is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return client.Ping(ctx)
}

// QueryObserver 记录慢查询等数据库事件。
type QueryObserver interface {
	ObserveQuery(ctx context.Context, query string, duration time.Duration, err error)
}

// RedactSQL 对 SQL 文本做保守脱敏，避免日志记录参数值。
func RedactSQL(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	return strings.Join(strings.Fields(query), " ")
}
