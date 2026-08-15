package database

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// MigrationStatus 只读查询独立 migration command 拥有的版本表。
func (c *gormClient) MigrationStatus(ctx context.Context, table string) (MigrationStatus, error) {
	if c.unavailable() {
		return MigrationStatus{}, ErrClientUnavailable
	}
	if err := validateContext(ctx); err != nil {
		return MigrationStatus{}, err
	}
	if !validIdentifier(table) {
		return MigrationStatus{}, fmt.Errorf("%w: invalid migration table %q", ErrInvalidQuery, table)
	}
	db := c.db.WithContext(ctx)
	if !db.Migrator().HasTable(table) {
		return MigrationStatus{Empty: true}, nil
	}
	var row struct {
		Version int64
		Dirty   bool
	}
	err := db.Table(table).Select("version", "dirty").Limit(1).Take(&row).Error
	if err == gorm.ErrRecordNotFound {
		return MigrationStatus{Empty: true}, nil
	}
	if err != nil {
		return MigrationStatus{}, fmt.Errorf("read migration status: %w", translateError(err))
	}
	if row.Version < 0 {
		return MigrationStatus{}, fmt.Errorf("%w: migration version is negative", ErrOperationFailed)
	}
	return MigrationStatus{Version: uint(row.Version), Dirty: row.Dirty}, nil
}
