package migration

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold-template/pkg/database"
)

// Access 是 compatibility gate 使用方定义的最窄数据库租约。
type Access interface {
	Use(context.Context, func(database.Client) error) error
}

// Compatibility 在启动 Todo 前只读校验 module-owned migration 版本。
type Compatibility struct{ access Access }

// NewCompatibility 构造不执行 I/O 的 Todo schema compatibility gate。
func NewCompatibility(access Access) (*Compatibility, error) {
	if access == nil {
		return nil, fmt.Errorf("todo migration compatibility access is nil")
	}
	return &Compatibility{access: access}, nil
}

// Check 拒绝空、dirty、too-old 与 too-new schema，不执行 DDL 或 repair。
func (c *Compatibility) Check(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("todo migration compatibility context is nil")
	}
	var status database.MigrationStatus
	if err := c.access.Use(ctx, func(client database.Client) error {
		var err error
		status, err = client.MigrationStatus(ctx, TableName)
		return err
	}); err != nil {
		return fmt.Errorf("read todo migration status: %w", err)
	}
	switch {
	case status.Dirty:
		return fmt.Errorf("todo migration is dirty at version %d", status.Version)
	case status.Empty:
		return fmt.Errorf("todo migration is empty; target version is %d", CurrentVersion)
	case status.Version < CurrentVersion:
		return fmt.Errorf("todo migration is too old: current %d target %d", status.Version, CurrentVersion)
	case status.Version > CurrentVersion:
		return fmt.Errorf("todo migration is too new: current %d target %d", status.Version, CurrentVersion)
	default:
		return nil
	}
}
