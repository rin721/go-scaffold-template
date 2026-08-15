package repo

import "github.com/rin721/go-scaffold-template/pkg/database"

// Schema 返回 Todo Repository 字段映射；它只描述查询映射，不执行或声明 DDL。
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
			{Name: "OwnerSubject", Column: "owner_subject", Type: database.FieldString, Length: 255},
		},
		Indexes: []database.Index{
			{Name: "idx_todos_status_created_at", Fields: []string{"Status", "CreatedAt"}},
			{Name: "idx_todos_owner_created_at", Fields: []string{"OwnerSubject", "CreatedAt"}},
		},
		VersionField: "Version",
	}
}
