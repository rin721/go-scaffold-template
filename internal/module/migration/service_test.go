package migration_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	modulemigration "github.com/rin721/go-scaffold-template/internal/module/migration"
	migrationconfig "github.com/rin721/go-scaffold-template/internal/module/migration/binding/config"
	todomigration "github.com/rin721/go-scaffold-template/internal/module/todo/binding/migration"
	"github.com/rin721/go-scaffold-template/pkg/database"
)

func TestSQLiteMigrationFreshAndIdempotent(t *testing.T) {
	service, databasePath := newSQLiteService(t)
	status, err := service.Status(t.Context())
	if err != nil || !status.Empty || status.Compatible {
		t.Fatalf("Status(empty) = %#v, %v", status, err)
	}
	statusConnection := openSQLite(t, databasePath)
	var versionTables int
	if err := statusConnection.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'").Scan(&versionTables); err != nil {
		_ = statusConnection.Close()
		t.Fatalf("inspect read-only status effect error = %v", err)
	}
	if err := statusConnection.Close(); err != nil {
		t.Fatalf("close status inspection error = %v", err)
	}
	if versionTables != 0 {
		t.Fatal("Status() created schema_migrations")
	}
	status, err = service.Up(t.Context(), "")
	if err != nil || status.Current != todomigration.CurrentVersion || !status.Compatible || status.Dirty {
		t.Fatalf("Up(fresh) = %#v, %v", status, err)
	}
	status, err = service.Up(t.Context(), "")
	if err != nil || !status.Compatible {
		t.Fatalf("Up(idempotent) = %#v, %v", status, err)
	}
	connection := openSQLite(t, databasePath)
	defer connection.Close()
	var notNull int
	rows, err := connection.QueryContext(t.Context(), "PRAGMA table_info(todos)")
	if err != nil {
		t.Fatalf("PRAGMA table_info error = %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, kind string
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan table info error = %v", err)
		}
		if name == "owner_subject" && notNull == 1 {
			return
		}
	}
	t.Fatal("owner_subject NOT NULL contract is missing")
}

func TestSQLiteMigrationRequiresExplicitLegacyOwner(t *testing.T) {
	service, databasePath := newSQLiteService(t)
	connection := openSQLite(t, databasePath)
	if _, err := connection.ExecContext(t.Context(), `
CREATE TABLE todos (
  id TEXT NOT NULL PRIMARY KEY,
  title TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  completed_at DATETIME NULL,
  version INTEGER NOT NULL
);
CREATE INDEX idx_todos_status_created_at ON todos (status, created_at);
CREATE TABLE schema_migrations (version INTEGER NOT NULL, dirty BOOLEAN NOT NULL);
INSERT INTO schema_migrations (version, dirty) VALUES (1, FALSE);
INSERT INTO todos (id, title, status, created_at, updated_at, version)
VALUES ('11111111-1111-4111-8111-111111111111', 'legacy', 'pending', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1);`); err != nil {
		_ = connection.Close()
		t.Fatalf("seed legacy schema error = %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("close legacy seed error = %v", err)
	}
	if _, err := service.Up(t.Context(), ""); !errors.Is(err, modulemigration.ErrCompletionRequired) {
		t.Fatalf("Up(without explicit owner) error = %v", err)
	}
	status, err := service.Status(t.Context())
	if err != nil || status.Current != todomigration.CurrentVersion || status.Compatible {
		t.Fatalf("Status(unresolved legacy) = %#v, %v", status, err)
	}
	status, err = service.Up(t.Context(), "legacy-owner")
	if err != nil || !status.Compatible {
		t.Fatalf("Up(explicit owner) = %#v, %v", status, err)
	}
	connection = openSQLite(t, databasePath)
	defer connection.Close()
	var owner string
	if err := connection.QueryRowContext(t.Context(), "SELECT owner_subject FROM todos LIMIT 1").Scan(&owner); err != nil || owner != "legacy-owner" {
		t.Fatalf("legacy owner = %q, %v", owner, err)
	}
}

func newSQLiteService(t *testing.T) (*modulemigration.Service, string) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "migration.db")
	config := database.DefaultConfig()
	config.Driver = database.DriverSQLite
	config.DSN = databasePath
	completion, err := todomigration.NewCompletion(config)
	if err != nil {
		t.Fatalf("NewCompletion() error = %v", err)
	}
	module, err := modulemigration.NewModule(
		config, migrationconfig.Config{LockTimeout: 5 * time.Second, OperationTimeout: 30 * time.Second},
		todomigration.Set(), modulemigration.NewDefaultFactory, completion,
	)
	if err != nil {
		t.Fatalf("NewModule() error = %v", err)
	}
	return module.Service, databasePath
}

func openSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()
	connection, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	return connection
}
