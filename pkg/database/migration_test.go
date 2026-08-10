package database

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestMigratorAppliesInOrder(t *testing.T) {
	var versions []string
	migrator, err := NewMigrator(
		Migration{Version: "001", Name: "init", Up: func(context.Context, Executor) error { versions = append(versions, "001"); return nil }},
		Migration{Version: "002", Name: "add", Up: func(context.Context, Executor) error { versions = append(versions, "002"); return nil }},
	)
	if err != nil {
		t.Fatalf("NewMigrator() error = %v", err)
	}
	if err := migrator.Apply(context.Background(), migrationFakeClient{}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := versions[0] + "," + versions[1]; got != "001,002" {
		t.Fatalf("versions = %s", got)
	}
}

func TestReadinessUsesPing(t *testing.T) {
	if err := Readiness(context.Background(), migrationFakeClient{}, time.Second); err != nil {
		t.Fatalf("Readiness() error = %v", err)
	}
}

var _ Client = migrationFakeClient{}

type migrationFakeClient struct{}

func (migrationFakeClient) Exec(context.Context, string, ...any) (Result, error) { return nil, nil }
func (migrationFakeClient) Query(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}
func (migrationFakeClient) QueryRow(context.Context, string, ...any) *sql.Row { return nil }
func (migrationFakeClient) Get(context.Context, any, string, ...any) error    { return nil }
func (migrationFakeClient) Select(context.Context, any, string, ...any) error { return nil }
func (migrationFakeClient) WithinTx(context.Context, *sql.TxOptions, func(context.Context, Tx) error) error {
	return nil
}
func (migrationFakeClient) Ping(context.Context) error { return nil }
func (migrationFakeClient) Stats() sql.DBStats         { return sql.DBStats{} }
func (migrationFakeClient) Close() error               { return nil }
