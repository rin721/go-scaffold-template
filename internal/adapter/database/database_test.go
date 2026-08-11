package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

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
	cfg, err := New().Decode(snapshot)
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

func TestAdapterBuildPreservesContextError(t *testing.T) {
	client, err := New().Build(nil, Config{
		Engine: pkgdatabase.EngineSQL,
		Driver: pkgdatabase.DriverPostgres,
		DSN:    "postgres://example.invalid/app",
	})
	if client != nil {
		t.Fatal("Build(nil) client is not nil")
	}
	if !errors.Is(err, pkgdatabase.ErrNilContext) {
		t.Fatalf("Build(nil) error = %v, want ErrNilContext", err)
	}
}

func TestAdapterLifecyclePreservesReadinessAndCloseErrors(t *testing.T) {
	pingErr := errors.New("ping failed")
	closeErr := errors.New("close failed")
	client := &fakeClient{pingErr: pingErr, closeErr: closeErr}
	capability := New()

	if err := capability.Start(t.Context(), client); !errors.Is(err, pingErr) {
		t.Fatalf("Start() error = %v, want ping error", err)
	}
	if err := capability.Stop(t.Context(), client); !errors.Is(err, closeErr) {
		t.Fatalf("Stop() error = %v, want close error", err)
	}
	if client.pings != 1 || client.closes != 1 {
		t.Fatalf("lifecycle calls = ping:%d close:%d, want 1 each", client.pings, client.closes)
	}
}

type fakeClient struct {
	pingErr  error
	closeErr error
	pings    int
	closes   int
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
	c.pings++
	return c.pingErr
}

func (c *fakeClient) Stats() sql.DBStats { return sql.DBStats{} }

func (c *fakeClient) Close() error {
	c.closes++
	return c.closeErr
}
