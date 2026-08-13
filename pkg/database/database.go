package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// NewGORM 创建由 GORM 实现的数据库资源。
//
// 技术实现由组合根调用本函数明确选择；配置只选择数据库协议和连接位置。
func NewGORM(ctx context.Context, cfg *Config) (Resource, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	resolved, err := resolveConfig(cfg)
	if err != nil {
		return nil, err
	}
	dsn := resolved.DSN
	if resolved.Driver == DriverSQLite {
		dsn, err = prepareSQLite(resolved.DSN)
		if err != nil {
			return nil, err
		}
		if sqliteMemoryDSN(resolved.DSN) {
			// 私有内存库按连接隔离；固定为单连接，避免连接池切换后表或数据凭空消失。
			resolved.Pool.MaxOpenConns = 1
			resolved.Pool.MaxIdleConns = 1
		}
	}

	db, err := gorm.Open(gormDialector(resolved.Driver, dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		DisableAutomaticPing:   true,
		TranslateError:         true,
		Logger:                 gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open %s database: %w", resolved.Driver, redactDriverError(err))
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("load database connection pool: %w", redactDriverError(err))
	}
	applyPoolConfig(sqlDB, resolved.Pool)
	client := &gormClient{db: db}
	resource := &gormResource{client: client, sqlDB: sqlDB, cfg: resolved}
	if err := resource.Ping(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("ping database: %w", err), resource.Close())
	}
	return resource, nil
}

type gormResource struct {
	client *gormClient
	sqlDB  *sql.DB
	cfg    resolvedConfig
}

// Client 返回不具备 Ping、Stats 或 Close 所有权的数据库视图。
func (r *gormResource) Client() Client { return r.client }

type gormClient struct {
	db     *gorm.DB
	closed atomic.Bool
}

func (c *gormClient) WithinTx(ctx context.Context, fn func(context.Context, Tx) error) (resultErr error) {
	if c.unavailable() {
		return ErrClientUnavailable
	}
	if err := validateContext(ctx); err != nil {
		return err
	}
	if fn == nil {
		return ErrNilTransactionFunc
	}
	tx := c.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("begin database transaction: %w", translateError(tx.Error))
	}
	txLifetime, cancelTx := context.WithCancel(ctx)
	defer cancelTx()
	txToken := &gormTx{db: tx, ctx: txLifetime}
	txToken.active.Store(true)
	defer txToken.active.Store(false)
	completed := false
	defer func() {
		if completed {
			return
		}
		rollbackErr := translateError(tx.Rollback().Error)
		if recovered := recover(); recovered != nil {
			// 回调 panic 时必须先释放事务资源；回滚成功后保持原 panic 值和栈语义。
			if rollbackErr != nil {
				panic(transactionPanic{value: recovered, rollback: rollbackErr})
			}
			panic(recovered)
		}
		if rollbackErr != nil {
			if resultErr != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("rollback database transaction: %w", rollbackErr))
			} else {
				resultErr = fmt.Errorf("rollback database transaction: %w", rollbackErr)
			}
		}
	}()
	if err := fn(txLifetime, txToken); err != nil {
		return err
	}
	if err := tx.Commit().Error; err != nil {
		completed = true
		return fmt.Errorf("commit database transaction: %w", translateError(err))
	}
	completed = true
	return nil
}

type transactionPanic struct {
	value    any
	rollback error
}

func (p transactionPanic) Error() string { return "database transaction panic and rollback failure" }

func (p transactionPanic) Unwrap() error { return p.rollback }

func (r *gormResource) Ping(ctx context.Context) error {
	if r.client.unavailable() {
		return ErrClientUnavailable
	}
	if err := validateContext(ctx); err != nil {
		return err
	}
	if r.cfg.PingTimeout <= 0 {
		if err := r.sqlDB.PingContext(ctx); err != nil {
			return redactDriverError(err)
		}
		return nil
	}
	pingCtx, cancel := context.WithTimeout(ctx, r.cfg.PingTimeout)
	defer cancel()
	if err := r.sqlDB.PingContext(pingCtx); err != nil {
		return redactDriverError(err)
	}
	return nil
}

func (r *gormResource) Stats() Stats {
	stats := r.sqlDB.Stats()
	return Stats{
		MaxOpenConnections: stats.MaxOpenConnections, OpenConnections: stats.OpenConnections,
		InUse: stats.InUse, Idle: stats.Idle, WaitCount: stats.WaitCount,
		WaitDuration: stats.WaitDuration, MaxIdleClosed: stats.MaxIdleClosed,
		MaxIdleTimeClosed: stats.MaxIdleTimeClosed, MaxLifetimeClosed: stats.MaxLifetimeClosed,
	}
}

func (r *gormResource) Close() error {
	if !r.client.closed.CompareAndSwap(false, true) {
		return nil
	}
	if err := r.sqlDB.Close(); err != nil {
		return translateError(err)
	}
	return nil
}

func (c *gormClient) databaseSession(ctx context.Context) (any, error) {
	if c.unavailable() {
		return nil, ErrClientUnavailable
	}
	return c.db.WithContext(ctx), nil
}

func (c *gormClient) unavailable() bool { return c.closed.Load() }

type gormTx struct {
	db     *gorm.DB
	ctx    context.Context
	active atomic.Bool
}

func (*gormTx) databaseTransaction() {}
func (t *gormTx) databaseSession(ctx context.Context) (any, error) {
	if !t.active.Load() {
		return nil, ErrClientUnavailable
	}
	operationCtx, _, err := contextBoundTo(ctx, t.ctx)
	if err != nil {
		return nil, err
	}
	return t.db.WithContext(operationCtx), nil
}

func contextBoundTo(ctx context.Context, lifetime context.Context) (context.Context, context.CancelFunc, error) {
	if err := validateContext(ctx); err != nil {
		return nil, nil, err
	}
	if err := validateContext(lifetime); err != nil {
		return nil, nil, err
	}
	operationCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(lifetime, cancel)
	return operationCtx, func() {
		stop()
		cancel()
	}, nil
}

func gormDialector(driver Driver, dsn string) gorm.Dialector {
	switch driver {
	case DriverSQLite:
		return sqlite.Open(dsn)
	case DriverMySQL:
		return mysql.Open(dsn)
	default:
		return postgres.Open(dsn)
	}
}

func prepareSQLite(dsn string) (string, error) {
	if sqliteMemoryDSN(dsn) {
		return sqliteDSN(dsn), nil
	}
	path, err := sqlitePath(dsn)
	if err != nil {
		return "", err
	}
	directory := filepath.Dir(path)
	if err := prepareSQLiteDirectory(directory); err != nil {
		return "", err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", fmt.Errorf("sqlite database path must be a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect sqlite database: %w", redactDriverError(err))
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return "", fmt.Errorf("create sqlite database: %w", redactDriverError(err))
	}
	closeErr := file.Close()
	chmodErr := os.Chmod(path, 0o600)
	if closeErr != nil || chmodErr != nil {
		return "", errors.Join(redactOptionalError(closeErr), redactOptionalError(chmodErr))
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			return "", fmt.Errorf("verify sqlite database: %w", redactDriverError(err))
		}
		if info.Mode().Perm() != 0o600 {
			return "", fmt.Errorf("sqlite database permissions must be 0600")
		}
	}
	return sqliteDSN(path), nil
}

func sqliteMemoryDSN(dsn string) bool {
	return dsn == ":memory:" || strings.HasPrefix(dsn, "file::memory:")
}

func prepareSQLiteDirectory(directory string) error {
	created := false
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("create sqlite directory: %w", redactDriverError(err))
		}
		created = true
		info, err = os.Lstat(directory)
	}
	if err != nil {
		return fmt.Errorf("inspect sqlite directory: %w", redactDriverError(err))
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("sqlite parent path must be a regular directory")
	}
	if created {
		if err := os.Chmod(directory, 0o700); err != nil {
			return fmt.Errorf("secure sqlite directory: %w", redactDriverError(err))
		}
		info, err = os.Stat(directory)
		if err != nil {
			return fmt.Errorf("verify sqlite directory: %w", redactDriverError(err))
		}
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("sqlite directory permissions must not grant group or other access")
	}
	return nil
}

func sqlitePath(dsn string) (string, error) {
	path := dsn
	if strings.HasPrefix(dsn, "file:") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			return "", fmt.Errorf("%w: invalid sqlite dsn", ErrEmptyDSN)
		}
		path = parsed.Path
		if path == "" {
			path = parsed.Opaque
		}
	}
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return "", fmt.Errorf("%w: sqlite path is empty", ErrEmptyDSN)
	}
	return path, nil
}

func sqliteDSN(path string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	if !strings.HasPrefix(path, "file:") && path != ":memory:" {
		path = "file:" + filepath.ToSlash(path)
	}
	return path + separator + "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
}

func applyPoolConfig(db *sql.DB, cfg PoolConfig) {
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	return ctx.Err()
}

type safeDatabaseError struct {
	classification error
	cause          error
}

func (e safeDatabaseError) Error() string { return e.classification.Error() }

func (e safeDatabaseError) Is(target error) bool {
	return target == e.classification || errors.Is(e.cause, target)
}

func redactDriverError(err error) error {
	return safeDatabaseError{classification: ErrOperationFailed, cause: err}
}

func redactOptionalError(err error) error {
	if err == nil {
		return nil
	}
	return redactDriverError(err)
}

func translateError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return safeDatabaseError{classification: ErrNotFound, cause: err}
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return safeDatabaseError{classification: ErrDuplicateKey, cause: err}
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return safeDatabaseError{classification: ErrForeignKeyViolation, cause: err}
	default:
		return safeDatabaseError{classification: ErrOperationFailed, cause: err}
	}
}

var _ Resource = (*gormResource)(nil)
var _ Client = (*gormClient)(nil)
var _ Tx = (*gormTx)(nil)
