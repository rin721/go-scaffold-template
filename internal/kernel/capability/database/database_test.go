package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgdatabase "github.com/rin721/go-scaffold2/pkg/database"
)

func TestDefinitionDecodesDatabaseDefaultsAndDurationStrings(t *testing.T) {
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
	definition := Definition()
	cfg, err := definition.Decode(snapshot)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if cfg.PingTimeout != 9*time.Second {
		t.Fatalf("PingTimeout = %v, want 9s", cfg.PingTimeout)
	}
	if cfg.Pool.MaxOpenConns != pkgdatabase.DefaultMaxOpenConns {
		t.Fatalf("MaxOpenConns = %d, want %d", cfg.Pool.MaxOpenConns, pkgdatabase.DefaultMaxOpenConns)
	}
}

func TestDefinitionBuilderPreservesContextError(t *testing.T) {
	client, err := Definition().Builder.Build(nil, Config{
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

func TestDefinitionInstanceHooksPreserveReadinessAndCloseErrors(t *testing.T) {
	pingErr := errors.New("ping failed")
	closeErr := errors.New("close failed")
	client := &fakeClient{pingErr: pingErr, closeErr: closeErr}
	hooks := Definition().Hooks

	if err := hooks.Start(t.Context(), client); !errors.Is(err, pingErr) {
		t.Fatalf("Start() error = %v, want ping error", err)
	}
	if err := hooks.Stop(t.Context(), client); !errors.Is(err, closeErr) {
		t.Fatalf("Stop() error = %v, want close error", err)
	}
	if client.pings != 1 || client.closes != 1 {
		t.Fatalf("lifecycle calls = ping:%d close:%d, want 1 each", client.pings, client.closes)
	}
}

func TestDefinitionDefaultsGenerateStableYAMLAndJSON(t *testing.T) {
	definition := Definition()
	manager, err := config.NewDefaultManager(config.Binding{
		CapabilityID: string(definition.ID),
		ConfigPath:   definition.ConfigPath,
		Contract:     definition.Defaults,
	})
	if err != nil {
		t.Fatalf("NewDefaultManager() error = %v", err)
	}
	tests := []struct {
		name string
		ext  string
		want string
	}{
		{
			name: "yaml",
			ext:  ".yaml",
			want: "database:\n" +
				"  engine: \"\"\n" +
				"  driver: \"\"\n" +
				"  dsn: \"\"\n" +
				"  pool:\n" +
				"    maxOpenConns: 25\n" +
				"    maxIdleConns: 5\n" +
				"    connMaxLifetime: 30m0s\n" +
				"    connMaxIdleTime: 5m0s\n" +
				"  pingTimeout: 5s\n",
		},
		{
			name: "json",
			ext:  ".json",
			want: "{\n" +
				"  \"database\": {\n" +
				"    \"engine\": \"\",\n" +
				"    \"driver\": \"\",\n" +
				"    \"dsn\": \"\",\n" +
				"    \"pool\": {\n" +
				"      \"maxOpenConns\": 25,\n" +
				"      \"maxIdleConns\": 5,\n" +
				"      \"connMaxLifetime\": \"30m0s\",\n" +
				"      \"connMaxIdleTime\": \"5m0s\"\n" +
				"    },\n" +
				"    \"pingTimeout\": \"5s\"\n" +
				"  }\n" +
				"}\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "config"+test.ext)
			if _, err := manager.Generate(t.Context(), config.GenerateRequest{Path: target}); err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			payload, err := os.ReadFile(target)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			if string(payload) != test.want {
				t.Fatalf("payload:\n%s\nwant:\n%s", payload, test.want)
			}
		})
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
