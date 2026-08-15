package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	pkgdatabase "github.com/rin721/go-scaffold-template/pkg/database"
)

// ReadStatus 只读查询 migration version table，不创建或修改 schema。
func ReadStatus(ctx context.Context, config pkgdatabase.Config, set Set) (result Status, resultErr error) {
	if ctx == nil {
		return Status{}, fmt.Errorf("migration status context is nil")
	}
	if err := pkgdatabase.ValidateConfig(&config); err != nil {
		return Status{}, fmt.Errorf("validate migration status database config: %w", err)
	}
	if err := validateSet(set, config.Driver); err != nil {
		return Status{}, err
	}
	table := set.MigrationsTable
	if table == "" {
		table = defaultMigrationTable
	}
	if !sqliteIdentifier.MatchString(table) {
		return Status{}, fmt.Errorf("migration table is invalid")
	}
	connection, _, err := openDatabase(config)
	if err != nil {
		return Status{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, sanitize(connection.Close())) }()
	exists, err := migrationTableExists(ctx, connection, config.Driver, table)
	if err != nil {
		return Status{}, fmt.Errorf("inspect migration version table: %w", sanitize(err))
	}
	if !exists {
		return Status{Empty: true}, nil
	}
	var version int64
	var dirty bool
	statement := "SELECT version, dirty FROM " + quotedTable(config.Driver, table) + " LIMIT 1"
	if err := connection.QueryRowContext(ctx, statement).Scan(&version, &dirty); errors.Is(err, sql.ErrNoRows) {
		return Status{Empty: true}, nil
	} else if err != nil {
		return Status{}, fmt.Errorf("read migration version: %w", sanitize(err))
	}
	if version < 0 {
		return Status{}, fmt.Errorf("migration version is invalid")
	}
	return Status{Version: uint(version), Dirty: dirty}, nil
}

func migrationTableExists(ctx context.Context, connection *sql.DB, driver pkgdatabase.Driver, table string) (bool, error) {
	var exists bool
	switch driver {
	case pkgdatabase.DriverSQLite:
		var count int
		err := connection.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count)
		return count == 1, err
	case pkgdatabase.DriverPostgres:
		err := connection.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists)
		return exists, err
	case pkgdatabase.DriverMySQL:
		var count int
		err := connection.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", table).Scan(&count)
		return count == 1, err
	default:
		return false, fmt.Errorf("unsupported migration database driver")
	}
}

func quotedTable(driver pkgdatabase.Driver, table string) string {
	if driver == pkgdatabase.DriverMySQL {
		return "`" + table + "`"
	}
	return `"` + table + `"`
}
