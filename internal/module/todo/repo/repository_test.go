package repo_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	modelbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/model"
	"github.com/rin721/go-scaffold-template/internal/module/todo/model"
	"github.com/rin721/go-scaffold-template/internal/module/todo/repo"
	"github.com/rin721/go-scaffold-template/internal/module/todo/service"
	"github.com/rin721/go-scaffold-template/pkg/database"
	"github.com/rin721/go-scaffold-template/pkg/fault"
)

func TestRepositorySQLiteContract(t *testing.T) {
	resource, err := database.NewGORM(t.Context(), &database.Config{
		Driver: database.DriverSQLite, DSN: filepath.Join(t.TempDir(), "todo.db"),
	})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	defer resource.Close()
	access := resourceAccess{resource: resource}
	if err := access.Use(t.Context(), func(client database.Client) error {
		return client.Migrate(t.Context(), modelbinding.Schema())
	}); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	repository, err := repo.New(access, modelbinding.Schema())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	now := time.Date(2026, 8, 15, 1, 2, 3, 0, time.UTC)
	todo, err := model.New("11111111-1111-4111-8111-111111111111", "学习 Go", now)
	if err != nil {
		t.Fatalf("model.New() error = %v", err)
	}
	created, err := repository.Create(t.Context(), todo)
	if err != nil || created.Version != 1 {
		t.Fatalf("Create() = %#v, %v", created, err)
	}
	items, total, err := repository.List(t.Context(), service.ListFilter{Offset: 0, Limit: 10})
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
	repository, err := repo.New(failingAccess{err: context.Canceled}, modelbinding.Schema())
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
