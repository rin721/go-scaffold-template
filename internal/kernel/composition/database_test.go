package composition

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel"
	databasecapability "github.com/rin721/go-scaffold2/internal/kernel/capability/database"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgdatabase "github.com/rin721/go-scaffold2/pkg/database"
)

func TestComposeDatabaseMakesDefinitionParticipateInKernelStart(t *testing.T) {
	runtime, err := kernel.New(config.New(config.MapSource("empty", map[string]any{})), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	binding, err := composeDatabase(runtime)
	if err != nil {
		t.Fatalf("composeDatabase() error = %v", err)
	}
	if binding.access == nil {
		t.Fatal("composeDatabase() access is nil")
	}
	if err := runtime.Start(t.Context()); err == nil {
		t.Fatal("Start() after composeDatabase error = nil, want missing database config error")
	}
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestComposedDatabaseReloadsThroughStableAccess(t *testing.T) {
	source := &databaseSource{dsn: "v1"}
	runtime, err := kernel.New(config.New(source), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	hooks := newFakeDatabaseHooks()
	definition := databasecapability.Definition()
	definition.Builder = hooks
	definition.Hooks = hooks
	binding, err := registerDatabase(runtime, definition)
	if err != nil {
		t.Fatalf("registerDatabase() error = %v", err)
	}
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Stop(context.Background()) })
	assertDatabaseVersion(t, binding.access, "v1")

	source.setDSN("v2")
	result, err := runtime.Reload(t.Context())
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if !result.Applied {
		t.Fatalf("Reload() result = %#v, want applied", result)
	}
	assertDatabaseVersion(t, binding.access, "v2")

	oldClient := hooks.client("v1")
	newClient := hooks.client("v2")
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

type fakeDatabaseHooks struct {
	mu      sync.Mutex
	created map[string]*fakeClient
}

func newFakeDatabaseHooks() *fakeDatabaseHooks {
	return &fakeDatabaseHooks{created: make(map[string]*fakeClient)}
}

func (h *fakeDatabaseHooks) Build(_ context.Context, cfg databasecapability.Config) (pkgdatabase.Client, error) {
	client := &fakeClient{version: cfg.DSN}
	h.mu.Lock()
	h.created[cfg.DSN] = client
	h.mu.Unlock()
	return client, nil
}

func (h *fakeDatabaseHooks) Start(ctx context.Context, client pkgdatabase.Client) error {
	return client.Ping(ctx)
}

func (h *fakeDatabaseHooks) Stop(_ context.Context, client pkgdatabase.Client) error {
	return client.Close()
}

func (h *fakeDatabaseHooks) client(version string) *fakeClient {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.created[version]
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

func assertDatabaseVersion(t *testing.T, access databasecapability.Access, want string) {
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
