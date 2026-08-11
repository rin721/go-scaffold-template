package assembly

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	databaseadapter "github.com/rin721/go-scaffold2/internal/adapter/database"
	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgdatabase "github.com/rin721/go-scaffold2/pkg/database"
)

func TestKernelNewDoesNotInjectDatabase(t *testing.T) {
	runtime, err := kernel.New(config.New(config.MapSource("empty", map[string]any{})), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() without Inject error = %v", err)
	}
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestInjectMakesDatabaseParticipateInKernelStart(t *testing.T) {
	runtime, err := kernel.New(config.New(config.MapSource("empty", map[string]any{})), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	capabilities, err := Inject(runtime)
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if capabilities.Database == nil {
		t.Fatal("Inject() Database access is nil")
	}
	if err := runtime.Start(t.Context()); err == nil {
		t.Fatal("Start() after Inject error = nil, want missing database config error")
	}
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestInjectRejectsDuplicateDatabase(t *testing.T) {
	runtime, err := kernel.New(config.New(config.MapSource("empty", map[string]any{})), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	if _, err := Inject(runtime); err != nil {
		t.Fatalf("first Inject() error = %v", err)
	}
	capabilities, err := Inject(runtime)
	if err == nil {
		t.Fatal("second Inject() error = nil")
	}
	if capabilities.Database != nil {
		t.Fatal("second Inject() returned partial capabilities")
	}
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestInjectedDatabaseReloadsThroughStableAccess(t *testing.T) {
	source := &databaseSource{dsn: "v1"}
	runtime, err := kernel.New(config.New(source), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	capability := newFakeDatabaseCapability()
	access, err := injectDatabase(runtime, capability)
	if err != nil {
		t.Fatalf("injectDatabase() error = %v", err)
	}
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Stop(context.Background()) })
	assertDatabaseVersion(t, access, "v1")

	source.setDSN("v2")
	result, err := runtime.Reload(t.Context())
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if !result.Applied {
		t.Fatalf("Reload() result = %#v, want applied", result)
	}
	assertDatabaseVersion(t, access, "v2")

	oldClient := capability.client("v1")
	newClient := capability.client("v2")
	if oldClient.closeCount() != 1 {
		t.Fatalf("old Close count = %d, want 1", oldClient.closeCount())
	}
	if newClient.pingCount() != 1 {
		t.Fatalf("new Ping count = %d, want 1", newClient.pingCount())
	}
}

type databaseSource struct {
	mu  sync.Mutex
	dsn string
}

func (s *databaseSource) Name() string { return "database-test" }

func (s *databaseSource) Load(context.Context) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]any{
		"database": map[string]any{
			"engine": "sql",
			"driver": "postgres",
			"dsn":    s.dsn,
		},
	}, nil
}

func (s *databaseSource) setDSN(dsn string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dsn = dsn
}

type fakeDatabaseCapability struct {
	adapter *databaseadapter.Adapter
	mu      sync.Mutex
	created map[string]*fakeClient
}

func newFakeDatabaseCapability() *fakeDatabaseCapability {
	return &fakeDatabaseCapability{
		adapter: databaseadapter.New(),
		created: make(map[string]*fakeClient),
	}
}

func (c *fakeDatabaseCapability) Decode(snapshot config.Snapshot) (databaseadapter.Config, error) {
	return c.adapter.Decode(snapshot)
}

func (c *fakeDatabaseCapability) Build(_ context.Context, cfg databaseadapter.Config) (pkgdatabase.Client, error) {
	client := &fakeClient{version: cfg.DSN}
	c.mu.Lock()
	c.created[cfg.DSN] = client
	c.mu.Unlock()
	return client, nil
}

func (c *fakeDatabaseCapability) Start(ctx context.Context, client pkgdatabase.Client) error {
	return client.Ping(ctx)
}

func (c *fakeDatabaseCapability) Stop(_ context.Context, client pkgdatabase.Client) error {
	return client.Close()
}

func (c *fakeDatabaseCapability) client(version string) *fakeClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.created[version]
}

type fakeClient struct {
	mu      sync.Mutex
	version string
	pings   int
	closes  int
}

func (c *fakeClient) Exec(context.Context, string, ...any) (pkgdatabase.Result, error) {
	return nil, nil
}

func (c *fakeClient) Query(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}

func (c *fakeClient) QueryRow(context.Context, string, ...any) *sql.Row { return nil }
func (c *fakeClient) Get(context.Context, any, string, ...any) error    { return nil }
func (c *fakeClient) Select(context.Context, any, string, ...any) error { return nil }

func (c *fakeClient) WithinTx(context.Context, *sql.TxOptions, func(context.Context, pkgdatabase.Tx) error) error {
	return nil
}

func (c *fakeClient) Ping(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pings++
	return nil
}

func (c *fakeClient) Stats() sql.DBStats { return sql.DBStats{} }

func (c *fakeClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closes++
	return nil
}

func (c *fakeClient) pingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pings
}

func (c *fakeClient) closeCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closes
}

func assertDatabaseVersion(t *testing.T, access databaseadapter.Access, want string) {
	t.Helper()
	if err := access.Use(t.Context(), func(client pkgdatabase.Client) error {
		if client.(*fakeClient).version != want {
			t.Fatalf("database version = %s, want %s", client.(*fakeClient).version, want)
		}
		return nil
	}); err != nil {
		t.Fatalf("Use() error = %v", err)
	}
}
