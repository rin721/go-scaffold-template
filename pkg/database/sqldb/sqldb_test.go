package sqldb

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/rin721/go-scaffold2/pkg/database/internal/core"
)

type userRow struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

func TestClientExecGetSelectAndStats(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.Close()

	mock.ExpectExec("UPDATE users SET name = \\? WHERE id = \\?").
		WithArgs("rei", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	result, err := client.Exec(context.Background(), "UPDATE users SET name = ? WHERE id = ?", "rei", 1)
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected returned error: %v", err)
	}
	if rowsAffected != 1 {
		t.Fatalf("RowsAffected = %d, want 1", rowsAffected)
	}

	mock.ExpectQuery("SELECT id, name FROM users WHERE id = \\?").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "rei"))
	var one userRow
	if err := client.Get(context.Background(), &one, "SELECT id, name FROM users WHERE id = ?", 1); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if one.Name != "rei" {
		t.Fatalf("Get name = %q, want rei", one.Name)
	}

	mock.ExpectQuery("SELECT id, name FROM users").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "rei").AddRow(2, "rin"))
	var many []userRow
	if err := client.Select(context.Background(), &many, "SELECT id, name FROM users"); err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if len(many) != 2 {
		t.Fatalf("Select len = %d, want 2", len(many))
	}
	if client.Stats().MaxOpenConnections != 7 {
		t.Fatalf("max open conns = %d, want 7", client.Stats().MaxOpenConnections)
	}

	assertExpectations(t, mock)
}

func TestWithinTxCommitAndRollback(t *testing.T) {
	t.Run("commit", func(t *testing.T) {
		client, mock := newMockClient(t)
		defer client.Close()

		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO users").WithArgs("rei").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		err := client.WithinTx(context.Background(), nil, func(ctx context.Context, tx core.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO users(name) VALUES (?)", "rei")
			return err
		})
		if err != nil {
			t.Fatalf("WithinTx returned error: %v", err)
		}
		assertExpectations(t, mock)
	})

	t.Run("rollback", func(t *testing.T) {
		client, mock := newMockClient(t)
		defer client.Close()

		wantErr := errors.New("domain failed")
		mock.ExpectBegin()
		mock.ExpectRollback()

		err := client.WithinTx(context.Background(), nil, func(context.Context, core.Tx) error {
			return wantErr
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("WithinTx error = %v, want %v", err, wantErr)
		}
		assertExpectations(t, mock)
	})
}

func TestGetTranslatesNoRows(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.Close()

	mock.ExpectQuery("SELECT id, name FROM users WHERE id = \\?").
		WithArgs(404).
		WillReturnError(sql.ErrNoRows)
	var one userRow
	err := client.Get(context.Background(), &one, "SELECT id, name FROM users WHERE id = ?", 404)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Get error = %v, want %v", err, core.ErrNotFound)
	}
	assertExpectations(t, mock)
}

func TestNewErrorDoesNotLeakDSN(t *testing.T) {
	secretDSN := "postgres://user:top-secret@example.invalid/app"
	_, err := New(context.Background(), &Config{
		Engine: core.EngineSQL,
		Driver: core.DriverPostgres,
		DSN:    secretDSN,
	})
	if err == nil {
		t.Fatal("New returned nil error")
	}
	if strings.Contains(err.Error(), secretDSN) || strings.Contains(err.Error(), "top-secret") {
		t.Fatalf("error leaks dsn: %v", err)
	}
}

func newMockClient(t *testing.T) (*Client, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	mock.ExpectPing()

	cfg := core.ResolvedConfig{
		Engine: core.EngineSQL,
		Driver: core.DriverPostgres,
		DSN:    "postgres://user:secret@example.invalid/app",
		Pool: core.PoolConfig{
			MaxOpenConns:    7,
			MaxIdleConns:    3,
			ConnMaxLifetime: time.Minute,
			ConnMaxIdleTime: time.Second,
		},
		PingTimeout: time.Second,
	}
	client, err := newClient(context.Background(), cfg, sqlx.NewDb(sqlDB, "sqlmock"))
	if err != nil {
		t.Fatalf("newClient returned error: %v", err)
	}
	return client, mock
}

func assertExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
