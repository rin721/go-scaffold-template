package gormdb

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/rin721/go-scaffold2/pkg/database/internal/core"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type accountRow struct {
	ID   int    `gorm:"column:id"`
	Name string `gorm:"column:name"`
}

func TestClientExecGetSelectAndStats(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.Close()

	mock.ExpectExec("UPDATE accounts SET name = \\$1 WHERE id = \\$2").
		WithArgs("rei", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	result, err := client.Exec(context.Background(), "UPDATE accounts SET name = $1 WHERE id = $2", "rei", 1)
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

	mock.ExpectQuery("SELECT id, name FROM accounts WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "rei"))
	var one accountRow
	if err := client.Get(context.Background(), &one, "SELECT id, name FROM accounts WHERE id = $1", 1); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if one.Name != "rei" {
		t.Fatalf("Get name = %q, want rei", one.Name)
	}

	mock.ExpectQuery("SELECT id, name FROM accounts").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "rei").AddRow(2, "rin"))
	var many []accountRow
	if err := client.Select(context.Background(), &many, "SELECT id, name FROM accounts"); err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if len(many) != 2 {
		t.Fatalf("Select len = %d, want 2", len(many))
	}
	if client.SQLDB() == nil {
		t.Fatal("SQLDB returned nil")
	}
	if client.DB(context.Background()) == nil {
		t.Fatal("DB returned nil")
	}
	if client.Stats().MaxOpenConnections != 9 {
		t.Fatalf("max open conns = %d, want 9", client.Stats().MaxOpenConnections)
	}

	assertExpectations(t, mock)
}

func TestWithinTxCommitAndRollback(t *testing.T) {
	t.Run("commit", func(t *testing.T) {
		client, mock := newMockClient(t)
		defer client.Close()

		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO accounts").WithArgs("rei").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		err := client.WithinTx(context.Background(), nil, func(ctx context.Context, tx core.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO accounts(name) VALUES ($1)", "rei")
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

func TestGetReturnsNotFoundWhenNoRowsScanned(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.Close()

	mock.ExpectQuery("SELECT id, name FROM accounts WHERE id = \\$1").
		WithArgs(404).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	var one accountRow
	err := client.Get(context.Background(), &one, "SELECT id, name FROM accounts WHERE id = $1", 404)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Get error = %v, want %v", err, core.ErrNotFound)
	}
	assertExpectations(t, mock)
}

func TestNewErrorDoesNotLeakDSN(t *testing.T) {
	secretDSN := "postgres://user:top-secret@example.invalid/app"
	_, err := New(context.Background(), &Config{
		Engine: core.EngineGORM,
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

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{SkipDefaultTransaction: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open gorm mock: %v", err)
	}

	cfg := core.ResolvedConfig{
		Engine: core.EngineGORM,
		Driver: core.DriverPostgres,
		DSN:    "postgres://user:secret@example.invalid/app",
		Pool: core.PoolConfig{
			MaxOpenConns:    9,
			MaxIdleConns:    4,
			ConnMaxLifetime: time.Minute,
			ConnMaxIdleTime: time.Second,
		},
		PingTimeout: time.Second,
	}
	client, err := newClient(context.Background(), cfg, db, sqlDB)
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
