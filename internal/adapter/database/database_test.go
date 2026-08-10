package database

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/rin721/go-scaffold2/internal/adapter"
	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgdatabase "github.com/rin721/go-scaffold2/pkg/database"
)

func TestDecodeUsesDatabaseDefaultsAndDurationStrings(t *testing.T) {
	snapshot, err := config.New(config.MapSource("test", map[string]any{
		"database": map[string]any{
			"engine":      "sql",
			"driver":      "postgres",
			"dsn":         "postgres://user:secret@example.invalid/app",
			"pingTimeout": "9s",
		},
	})).Load(t.Context())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg, err := decode(snapshot)
	if err != nil {
		t.Fatalf("decode() error = %v", err)
	}
	if cfg.PingTimeout != 9*time.Second {
		t.Fatalf("PingTimeout = %v, want 9s", cfg.PingTimeout)
	}
	if cfg.Pool.MaxOpenConns != pkgdatabase.DefaultMaxOpenConns {
		t.Fatalf("MaxOpenConns = %d, want %d", cfg.Pool.MaxOpenConns, pkgdatabase.DefaultMaxOpenConns)
	}
}

func TestDatabaseAdapterReloadsThroughStableAccess(t *testing.T) {
	source := &databaseSource{dsn: "v1"}
	runtime, err := kernel.New(config.New(source), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	var mu sync.Mutex
	created := make(map[string]*fakeClient)
	access, err := register(runtime, adapter.BuilderFunc[Config, pkgdatabase.Client](func(_ context.Context, cfg Config) (pkgdatabase.Client, error) {
		client := &fakeClient{version: cfg.DSN}
		mu.Lock()
		created[cfg.DSN] = client
		mu.Unlock()
		return client, nil
	}))
	if err != nil {
		t.Fatalf("register() error = %v", err)
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

	mu.Lock()
	oldClient := created["v1"]
	newClient := created["v2"]
	mu.Unlock()
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

func assertDatabaseVersion(t *testing.T, access Access, want string) {
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
