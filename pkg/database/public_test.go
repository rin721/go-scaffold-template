package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/rin721/go-scaffold2/pkg/database"
)

type publicAccount struct {
	ID      uint64
	Name    string
	Version uint64
}

func TestPublicAPIContract(t *testing.T) {
	resource, err := database.NewGORM(t.Context(), &database.Config{
		Driver: database.DriverSQLite,
		DSN:    ":memory:",
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
	if err := client.Migrate(t.Context(), schema); err != nil {
		t.Fatalf("Migrate() error = %v", err)
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
