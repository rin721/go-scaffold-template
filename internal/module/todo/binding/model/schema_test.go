package modelbinding

import (
	"context"
	"testing"

	"github.com/rin721/go-scaffold-template/pkg/database"
)

func TestSchemaAndMigratorContract(t *testing.T) {
	schema := Schema()
	if schema.Table != "todos" || schema.VersionField != "Version" || len(schema.Fields) != 7 || len(schema.Indexes) != 1 {
		t.Fatalf("Schema() = %#v", schema)
	}
	access := &recordingAccess{}
	migrator, err := NewMigrator(access)
	if err != nil {
		t.Fatalf("NewMigrator() error = %v", err)
	}
	if migrator.Name() != "module.todo.schema" {
		t.Fatalf("Name() = %q", migrator.Name())
	}
	if err := migrator.Start(t.Context()); err != nil || !access.migrated {
		t.Fatalf("Start() = %v, migrated=%v", err, access.migrated)
	}
}

type recordingAccess struct{ migrated bool }

func (a *recordingAccess) Use(ctx context.Context, use func(database.Client) error) error {
	return use(recordingClient{migrated: &a.migrated})
}
func (*recordingAccess) WithinTx(context.Context, func(context.Context, database.Client, database.Tx) error) error {
	return nil
}

type recordingClient struct{ migrated *bool }

func (recordingClient) WithinTx(context.Context, func(context.Context, database.Tx) error) error {
	return nil
}
func (c recordingClient) Migrate(context.Context, ...database.Schema) error {
	*c.migrated = true
	return nil
}

func (recordingClient) CheckSchemas(context.Context, ...database.Schema) error { return nil }
