package migrate

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sync/atomic"

	"github.com/golang-migrate/migrate/v4/database"
)

var sqliteIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

// sqliteDriver 让 golang-migrate 复用项目既有的纯 Go SQLite database/sql driver，
// 避免在同一进程注册第二个同名 SQLite driver。
type sqliteDriver struct {
	database *sql.DB
	table    string
	locked   atomic.Bool
}

func newSQLiteDriver(connection *sql.DB, table string) (database.Driver, error) {
	if connection == nil {
		return nil, fmt.Errorf("sqlite migration connection is nil")
	}
	if !sqliteIdentifier.MatchString(table) {
		return nil, fmt.Errorf("sqlite migration table is invalid")
	}
	driver := &sqliteDriver{database: connection, table: table}
	if err := driver.database.Ping(); err != nil {
		return nil, err
	}
	if err := driver.ensureVersionTable(); err != nil {
		return nil, err
	}
	return driver, nil
}

func (*sqliteDriver) Open(string) (database.Driver, error) {
	return nil, fmt.Errorf("sqlite migration driver requires an owned connection")
}

func (d *sqliteDriver) Close() error { return d.database.Close() }

func (d *sqliteDriver) Lock() error {
	if !d.locked.CompareAndSwap(false, true) {
		return database.ErrLocked
	}
	return nil
}

func (d *sqliteDriver) Unlock() error {
	if !d.locked.CompareAndSwap(true, false) {
		return database.ErrNotLocked
	}
	return nil
}

func (d *sqliteDriver) Run(migration io.Reader) error {
	payload, err := io.ReadAll(migration)
	if err != nil {
		return err
	}
	return d.transaction(string(payload), nil)
}

func (d *sqliteDriver) SetVersion(version int, dirty bool) error {
	transaction, err := d.database.Begin()
	if err != nil {
		return err
	}
	// #nosec G202 -- table 在 Adapter 构造时经过严格 identifier 校验，不包含外部 SQL 片段。
	if _, err := transaction.Exec("DELETE FROM " + d.table); err != nil {
		return errors.Join(err, transaction.Rollback())
	}
	if version >= 0 || version == database.NilVersion && dirty {
		// #nosec G202 -- table 在 Adapter 构造时经过严格 identifier 校验，值仍使用占位符。
		statement := "INSERT INTO " + d.table + " (version, dirty) VALUES (?, ?)"
		if _, err := transaction.Exec(statement, version, dirty); err != nil {
			return errors.Join(err, transaction.Rollback())
		}
	}
	return transaction.Commit()
}

func (d *sqliteDriver) Version() (int, bool, error) {
	var version int
	var dirty bool
	err := d.database.QueryRow("SELECT version, dirty FROM "+d.table+" LIMIT 1").Scan(&version, &dirty)
	if errors.Is(err, sql.ErrNoRows) {
		return database.NilVersion, false, nil
	}
	return version, dirty, err
}

func (*sqliteDriver) Drop() error {
	return fmt.Errorf("sqlite migration drop is not supported")
}

func (d *sqliteDriver) ensureVersionTable() (resultErr error) {
	if err := d.Lock(); err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, d.Unlock()) }()
	// #nosec G202 -- table 在 Adapter 构造时经过严格 identifier 校验，不接受任意 SQL。
	statement := "CREATE TABLE IF NOT EXISTS " + d.table + " (version INTEGER NOT NULL, dirty BOOLEAN NOT NULL)"
	if _, err := d.database.Exec(statement); err != nil {
		return err
	}
	return nil
}

func (d *sqliteDriver) transaction(statement string, arguments []any) error {
	transaction, err := d.database.Begin()
	if err != nil {
		return err
	}
	if _, err := transaction.Exec(statement, arguments...); err != nil {
		return errors.Join(err, transaction.Rollback())
	}
	return transaction.Commit()
}

var _ database.Driver = (*sqliteDriver)(nil)
