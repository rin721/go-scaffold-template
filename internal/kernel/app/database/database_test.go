package database

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	pkgdatabase "github.com/rin721/go-scaffold2/pkg/database"
)

func TestDefinitionContributesConfigWithoutExposingClose(t *testing.T) {
	definition, err := Definition()
	if err != nil {
		t.Fatalf("Definition() error = %v", err)
	}
	plan := app.NewPlan()
	added, err := app.Add(plan, definition)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if added.Output == nil {
		t.Fatal("Database Access is nil")
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	bindings := frozen.Configurations()
	if len(bindings) != 1 || bindings[0].CapabilityID != string(ID) || bindings[0].ConfigPath != ConfigPath {
		t.Fatalf("Configurations() = %#v", bindings)
	}
	clientType := reflect.TypeOf((*Client)(nil)).Elem()
	if _, exists := clientType.MethodByName("Close"); exists {
		t.Fatal("Database Client exposes shared Close ownership")
	}
	if _, exists := clientType.MethodByName("Ping"); exists {
		t.Fatal("Database Client exposes owner-only Ping")
	}
	if _, exists := clientType.MethodByName("Stats"); exists {
		t.Fatal("Database Client exposes owner-only Stats")
	}
	resource := &fakeResource{client: &fakeClient{}}
	access, err := newAccess(fakeLease{resource: resource})
	if err != nil {
		t.Fatalf("newAccess() error = %v", err)
	}
	if err := access.Use(context.Background(), func(client Client) error {
		if _, exposed := client.(pkgdatabase.Resource); exposed {
			t.Fatal("Database Client dynamic value exposes Resource ownership")
		}
		return nil
	}); err != nil {
		t.Fatalf("Access.Use() error = %v", err)
	}
	if err := access.Ping(t.Context()); err != nil {
		t.Fatalf("Access.Ping() error = %v", err)
	}
	if err := access.Ping(nil); !errors.Is(err, pkgdatabase.ErrNilContext) {
		t.Fatalf("Access.Ping(nil) error = %v, want ErrNilContext", err)
	}
	accessType := reflect.TypeOf((*Access)(nil)).Elem()
	if _, exists := accessType.MethodByName("Close"); exists {
		t.Fatal("Database Access exposes shared Close ownership")
	}
	if _, exists := accessType.MethodByName("Stats"); exists {
		t.Fatal("Database Access exposes owner-only Stats")
	}
}

func TestAccessWithinTxUsesOneLease(t *testing.T) {
	resource, err := pkgdatabase.NewGORM(t.Context(), &pkgdatabase.Config{
		Driver: pkgdatabase.DriverSQLite,
		DSN:    ":memory:",
		Pool:   pkgdatabase.PoolConfig{MaxOpenConns: 1, MaxIdleConns: 1},
	})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	defer resource.Close()
	access, err := newAccess(fakeLease{resource: resource})
	if err != nil {
		t.Fatalf("newAccess() error = %v", err)
	}
	called := false
	if err := access.WithinTx(context.Background(), func(_ context.Context, client Client, tx pkgdatabase.Tx) error {
		called = true
		if client == nil || tx == nil {
			t.Fatal("WithinTx() exposed nil client or transaction")
		}
		return nil
	}); err != nil {
		t.Fatalf("WithinTx() error = %v", err)
	}
	if !called {
		t.Fatal("WithinTx() did not invoke callback")
	}
}

func TestAccessInvalidatesEscapedClientAfterLease(t *testing.T) {
	resource := &fakeResource{client: &fakeClient{}}
	access, err := newAccess(fakeLease{resource: resource})
	if err != nil {
		t.Fatalf("newAccess() error = %v", err)
	}
	var escaped Client
	if err := access.Use(context.Background(), func(client Client) error {
		escaped = client
		return nil
	}); err != nil {
		t.Fatalf("Access.Use() error = %v", err)
	}
	if err := escaped.Migrate(context.Background()); !errors.Is(err, pkgdatabase.ErrClientUnavailable) {
		t.Fatalf("escaped Client error = %v, want ErrClientUnavailable", err)
	}
}

type fakeLease struct{ resource pkgdatabase.Resource }

func (f fakeLease) Use(ctx context.Context, use func(pkgdatabase.Resource) error) error {
	return use(f.resource)
}

type fakeResource struct{ client pkgdatabase.Client }

func (f *fakeResource) Client() pkgdatabase.Client { return f.client }
func (*fakeResource) Ping(context.Context) error   { return nil }
func (*fakeResource) Stats() pkgdatabase.Stats     { return pkgdatabase.Stats{} }
func (*fakeResource) Close() error                 { return nil }

type fakeClient struct{ transactions int }

func (f *fakeClient) WithinTx(ctx context.Context, use func(context.Context, pkgdatabase.Tx) error) error {
	f.transactions++
	return nil
}
func (*fakeClient) Migrate(context.Context, ...pkgdatabase.Schema) error { return nil }
