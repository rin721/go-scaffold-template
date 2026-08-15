package repo_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	migrationbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/migration"
	"github.com/rin721/go-scaffold-template/internal/module/todo/model"
	"github.com/rin721/go-scaffold-template/internal/module/todo/repo"
	"github.com/rin721/go-scaffold-template/internal/module/todo/service"
	"github.com/rin721/go-scaffold-template/pkg/database"
	dbmigrate "github.com/rin721/go-scaffold-template/pkg/database/migrate"
	"github.com/rin721/go-scaffold-template/pkg/fault"
)

func TestRepositorySQLiteContract(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "todo.db")
	migrationConfig := database.DefaultConfig()
	migrationConfig.Driver = database.DriverSQLite
	migrationConfig.DSN = databasePath
	runner, err := dbmigrate.New(t.Context(), dbmigrate.Config{
		Database: migrationConfig, LockTimeout: 5 * time.Second,
	}, migrationbinding.Set())
	if err != nil {
		t.Fatalf("New migration runner() error = %v", err)
	}
	if err := runner.Up(t.Context()); err != nil {
		_ = runner.Close()
		t.Fatalf("Migration up() error = %v", err)
	}
	if err := runner.Close(); err != nil {
		t.Fatalf("Close migration runner() error = %v", err)
	}
	resource, err := database.NewGORM(t.Context(), &database.Config{
		Driver: database.DriverSQLite, DSN: databasePath,
	})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	defer resource.Close()
	access := resourceAccess{resource: resource}
	repository, err := repo.New(access, repo.Schema())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	now := time.Date(2026, 8, 15, 1, 2, 3, 0, time.UTC)
	todo, err := model.New("11111111-1111-4111-8111-111111111111", "学习 Go", "actor-a", now)
	if err != nil {
		t.Fatalf("model.New() error = %v", err)
	}
	created, err := repository.Create(t.Context(), todo)
	if err != nil || created.Version != 1 {
		t.Fatalf("Create() = %#v, %v", created, err)
	}
	items, total, err := repository.List(t.Context(), service.ListFilter{OwnerSubject: "actor-a", Offset: 0, Limit: 10})
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("List() = %#v, %d, %v", items, total, err)
	}
	if _, err := created.Complete(now.Add(time.Hour)); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	stale := created
	saved, err := repository.Save(t.Context(), created)
	if err != nil || saved.Version != 2 || saved.Status != model.StatusCompleted {
		t.Fatalf("Save() = %#v, %v", saved, err)
	}
	if _, err := repository.Save(t.Context(), stale); fault.CodeOf(err) != fault.CodeConflict {
		t.Fatalf("Save(stale) error = %v, code = %s", err, fault.CodeOf(err))
	}
	if _, err := repository.Get(t.Context(), "22222222-2222-4222-8222-222222222222"); fault.CodeOf(err) != fault.CodeNotFound {
		t.Fatalf("Get(missing) error = %v", err)
	}
}

type resourceAccess struct{ resource database.Resource }

func (a resourceAccess) Use(ctx context.Context, use func(database.Client) error) error {
	return database.Borrow(ctx, a.resource.Client(), use)
}

func (a resourceAccess) WithinTx(ctx context.Context, use func(context.Context, database.Client, database.Tx) error) error {
	return a.Use(ctx, func(client database.Client) error {
		return client.WithinTx(ctx, func(txCtx context.Context, tx database.Tx) error {
			return use(txCtx, client, tx)
		})
	})
}

func TestRepositoryPreservesCancellation(t *testing.T) {
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	repository, err := repo.New(failingAccess{err: context.Canceled}, repo.Schema())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := repository.Get(cancelled, "id"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get(cancelled) error = %v", err)
	}
}

type failingAccess struct{ err error }

func (a failingAccess) Use(context.Context, func(database.Client) error) error { return a.err }
func (a failingAccess) WithinTx(context.Context, func(context.Context, database.Client, database.Tx) error) error {
	return a.err
}
