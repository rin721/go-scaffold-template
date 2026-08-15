package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/rin721/go-scaffold-template/pkg/database"
	dbmigrate "github.com/rin721/go-scaffold-template/pkg/database/migrate"
)

const unassignedOwnerSubject = "migration:unassigned"

// Completion 负责 Todo owner migration 的显式数据回填与完成门禁。
type Completion struct{ database database.Config }

// NewCompletion 构造不打开连接的 Todo migration completion。
func NewCompletion(config database.Config) (*Completion, error) {
	if err := database.ValidateConfig(&config); err != nil {
		return nil, fmt.Errorf("validate Todo migration completion database config: %w", err)
	}
	return &Completion{database: config}, nil
}

// Resolve 把隔离占位 owner 显式映射到 operator 提供的真实 subject。
// 没有 legacy 行时允许省略 subject；存在 legacy 行时拒绝猜测。
func (c *Completion) Resolve(ctx context.Context, legacyOwnerSubject string) error {
	return c.use(ctx, func(transaction *sql.Tx, firstPlaceholder, secondPlaceholder string) error {
		count, err := unresolvedCount(ctx, transaction, firstPlaceholder)
		if err != nil || count == 0 {
			return err
		}
		legacyOwnerSubject = strings.TrimSpace(legacyOwnerSubject)
		if legacyOwnerSubject == "" || strings.HasPrefix(legacyOwnerSubject, "migration:") {
			return dbmigrate.ErrCompletionRequired
		}
		statement := "UPDATE todos SET owner_subject = " + firstPlaceholder + " WHERE owner_subject = " + secondPlaceholder
		if _, err := transaction.ExecContext(ctx, statement, legacyOwnerSubject, unassignedOwnerSubject); err != nil {
			return fmt.Errorf("backfill Todo legacy owner: migration operation failed")
		}
		return nil
	})
}

// Verify 拒绝仍含隔离占位 owner 的 schema。
func (c *Completion) Verify(ctx context.Context) error {
	return c.use(ctx, func(transaction *sql.Tx, firstPlaceholder, _ string) error {
		count, err := unresolvedCount(ctx, transaction, firstPlaceholder)
		if err != nil {
			return err
		}
		if count != 0 {
			return dbmigrate.ErrCompletionRequired
		}
		return nil
	})
}

func unresolvedCount(ctx context.Context, transaction *sql.Tx, placeholder string) (int64, error) {
	var count int64
	statement := "SELECT COUNT(*) FROM todos WHERE owner_subject = " + placeholder
	if err := transaction.QueryRowContext(ctx, statement, unassignedOwnerSubject).Scan(&count); err != nil {
		return 0, fmt.Errorf("inspect Todo legacy owners: migration operation failed")
	}
	return count, nil
}

func (c *Completion) use(ctx context.Context, operation func(*sql.Tx, string, string) error) (resultErr error) {
	if ctx == nil {
		return fmt.Errorf("Todo migration completion context is nil")
	}
	driver, firstPlaceholder, secondPlaceholder, err := completionDriver(c.database.Driver)
	if err != nil {
		return err
	}
	connection, err := sql.Open(driver, c.database.DSN)
	if err != nil {
		return fmt.Errorf("open Todo migration completion database: migration operation failed")
	}
	connection.SetMaxOpenConns(1)
	defer func() {
		if err := connection.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close Todo migration completion database: migration operation failed"))
		}
	}()
	transaction, err := connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Todo migration completion: migration operation failed")
	}
	if err := operation(transaction, firstPlaceholder, secondPlaceholder); err != nil {
		if rollbackErr := transaction.Rollback(); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback Todo migration completion: migration operation failed"))
		}
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit Todo migration completion: migration operation failed")
	}
	return nil
}

func completionDriver(driver database.Driver) (string, string, string, error) {
	switch driver {
	case database.DriverSQLite:
		return "sqlite", "?", "?", nil
	case database.DriverPostgres:
		return "pgx", "$1", "$2", nil
	case database.DriverMySQL:
		return "mysql", "?", "?", nil
	default:
		return "", "", "", fmt.Errorf("unsupported Todo migration completion driver")
	}
}
