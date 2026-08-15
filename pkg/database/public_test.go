package database_test

import (
	"context"
	stdlibsql "database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/rin721/go-scaffold-template/pkg/database"
)

type publicAccount struct {
	ID      uint64
	Name    string
	Version uint64
}

func TestPublicAPIContract(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "public.db")
	seed, err := stdlibsql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if _, err := seed.ExecContext(t.Context(), `
CREATE TABLE public_accounts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  version INTEGER NOT NULL
);
CREATE UNIQUE INDEX uidx_public_accounts_name ON public_accounts (name);`); err != nil {
		_ = seed.Close()
		t.Fatalf("seed public schema error = %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close public schema seed error = %v", err)
	}
	resource, err := database.NewGORM(t.Context(), &database.Config{
		Driver: database.DriverSQLite,
		DSN:    databasePath,
	})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	t.Cleanup(func() {
		if err := resource.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	client := resource.Client()
	if _, exposesOwnership := client.(database.Resource); exposesOwnership {
		t.Fatal("Client exposes Resource ownership through its dynamic type")
	}
	schema := database.Schema{
		Table: "public_accounts",
		Fields: []database.Field{
			{Name: "ID", Column: "id", Type: database.FieldUint64, PrimaryKey: true, AutoIncrement: true},
			{Name: "Name", Column: "name", Type: database.FieldString, Length: 100},
			{Name: "Version", Column: "version", Type: database.FieldUint64},
		},
		Indexes:      []database.Index{{Name: "uidx_public_accounts_name", Fields: []string{"Name"}, Unique: true}},
		VersionField: "Version",
	}
	repository, err := database.NewRepository[publicAccount](client, schema)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	created := publicAccount{Name: "Rin"}
	if err := repository.Create(t.Context(), &created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == 0 || created.Version != 1 {
		t.Fatalf("Create() result = %+v", created)
	}

	err = client.WithinTx(t.Context(), func(ctx context.Context, tx database.Tx) error {
		txRepository, err := repository.WithTx(tx)
		if err != nil {
			return err
		}
		return txRepository.Create(ctx, &publicAccount{Name: "Lin"})
	})
	if err != nil {
		t.Fatalf("WithinTx() error = %v", err)
	}
	if err := repository.Create(t.Context(), &publicAccount{Name: "Rin"}); !errors.Is(err, database.ErrDuplicateKey) {
		t.Fatalf("duplicate Create() error = %v", err)
	}
	count, err := repository.Count(context.Background(), database.Query{})
	if err != nil || count != 2 {
		t.Fatalf("Count() = %d, %v", count, err)
	}
}
