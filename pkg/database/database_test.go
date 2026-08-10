package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/rin721/go-scaffold2/pkg/database/internal/core"
)

func TestNewRoutesByEngine(t *testing.T) {
	tests := []struct {
		name   string
		engine Engine
		want   string
	}{
		{name: "gorm", engine: EngineGORM, want: "gorm"},
		{name: "sql", engine: EngineSQL, want: "sql"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalGORM := openGORM
			originalSQL := openSQL
			t.Cleanup(func() {
				openGORM = originalGORM
				openSQL = originalSQL
			})

			var selected string
			openGORM = func(context.Context, core.ResolvedConfig) (core.Client, error) {
				selected = "gorm"
				return fakeClient{}, nil
			}
			openSQL = func(context.Context, core.ResolvedConfig) (core.Client, error) {
				selected = "sql"
				return fakeClient{}, nil
			}

			client, err := New(context.Background(), &Config{
				Engine: tt.engine,
				Driver: DriverPostgres,
				DSN:    "postgres://user:secret@example.invalid/app",
			})
			if err != nil {
				t.Fatalf("New returned error: %v", err)
			}
			if client == nil {
				t.Fatal("New returned nil client")
			}
			if selected != tt.want {
				t.Fatalf("selected engine = %q, want %q", selected, tt.want)
			}
		})
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want error
	}{
		{name: "nil config", cfg: nil, want: ErrInvalidEngine},
		{name: "empty engine", cfg: &Config{Driver: DriverPostgres, DSN: "x"}, want: ErrInvalidEngine},
		{name: "invalid driver", cfg: &Config{Engine: EngineSQL, Driver: Driver("sqlite"), DSN: "x"}, want: ErrInvalidDriver},
		{name: "empty dsn", cfg: &Config{Engine: EngineSQL, Driver: DriverPostgres}, want: ErrEmptyDSN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(context.Background(), tt.cfg)
			if err == nil {
				t.Fatal("New returned nil error")
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestNewRejectsNilContext(t *testing.T) {
	_, err := New(nil, &Config{Engine: EngineSQL, Driver: DriverPostgres, DSN: "x"})
	if !errors.Is(err, ErrNilContext) {
		t.Fatalf("error = %v, want %v", err, ErrNilContext)
	}
}

func TestValidateConfigDoesNotOpenDatabase(t *testing.T) {
	originalSQL := openSQL
	t.Cleanup(func() { openSQL = originalSQL })
	openSQL = func(context.Context, core.ResolvedConfig) (core.Client, error) {
		t.Fatal("ValidateConfig() opened database")
		return nil, nil
	}
	cfg := &Config{
		Engine: EngineSQL,
		Driver: DriverPostgres,
		DSN:    "postgres://user:secret@example.invalid/app",
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
}

func TestValidationErrorDoesNotLeakDSN(t *testing.T) {
	secretDSN := "postgres://user:top-secret@example.invalid/app"
	_, err := New(context.Background(), &Config{
		Engine: Engine("unknown"),
		Driver: DriverPostgres,
		DSN:    secretDSN,
	})
	if err == nil {
		t.Fatal("New returned nil error")
	}
	if strings.Contains(err.Error(), secretDSN) || strings.Contains(err.Error(), "top-secret") {
		t.Fatalf("error leaks dsn: %v", err)
	}
}

type fakeClient struct{}

func (fakeClient) Exec(context.Context, string, ...any) (Result, error) { return nil, nil }
func (fakeClient) Query(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}
func (fakeClient) QueryRow(context.Context, string, ...any) *sql.Row { return nil }
func (fakeClient) Get(context.Context, any, string, ...any) error    { return nil }
func (fakeClient) Select(context.Context, any, string, ...any) error { return nil }
func (fakeClient) WithinTx(context.Context, *sql.TxOptions, func(context.Context, Tx) error) error {
	return nil
}
func (fakeClient) Ping(context.Context) error { return nil }
func (fakeClient) Stats() sql.DBStats         { return sql.DBStats{} }
func (fakeClient) Close() error               { return nil }
