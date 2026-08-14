// Package modelbinding 绑定 Todo 持久化 Schema 与启动 migration。
package modelbinding

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold2/internal/module/todo/repo"
	"github.com/rin721/go-scaffold2/pkg/database"
)

const participantName = "module.todo.schema"

// Schema 返回 Todo Record 的唯一数据库 Schema。
func Schema() database.Schema {
	return database.Schema{
		Table: "todos",
		Fields: []database.Field{
			{Name: "ID", Column: "id", Type: database.FieldString, Length: 36, PrimaryKey: true},
			{Name: "Title", Column: "title", Type: database.FieldString, Length: 200},
			{Name: "Status", Column: "status", Type: database.FieldString, Length: 16},
			{Name: "CreatedAt", Column: "created_at", Type: database.FieldTime},
			{Name: "UpdatedAt", Column: "updated_at", Type: database.FieldTime},
			{Name: "CompletedAt", Column: "completed_at", Type: database.FieldTime, Nullable: true},
			{Name: "Version", Column: "version", Type: database.FieldUint64},
		},
		Indexes: []database.Index{{
			Name: "idx_todos_status_created_at", Fields: []string{"Status", "CreatedAt"},
		}},
		VersionField: "Version",
	}
}

// Migrator 在业务入口就绪前执行 Todo additive migration。
type Migrator struct {
	access repo.Access
	schema database.Schema
}

// NewMigrator 创建不执行 I/O 的 migration participant。
func NewMigrator(access repo.Access) (*Migrator, error) {
	if access == nil {
		return nil, fmt.Errorf("todo migration database access is nil")
	}
	return &Migrator{access: access, schema: Schema()}, nil
}

// Name 返回 Supervisor 使用的稳定 owner ID。
func (*Migrator) Name() string { return participantName }

// Start 在当前 Database generation 租约内执行 additive migration。
func (m *Migrator) Start(ctx context.Context) error {
	return m.access.Use(ctx, func(client database.Client) error {
		if err := client.Migrate(ctx, m.schema); err != nil {
			return fmt.Errorf("migrate todo schema: %w", err)
		}
		return nil
	})
}

// Stop 不释放共享 Database；连接池所有权仍属于 Kernel。
func (*Migrator) Stop(context.Context) error { return nil }
