// Package migrate 封装 golang-migrate，并只暴露项目自有迁移契约。
package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	golangmigrate "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
	pkgdatabase "github.com/rin721/go-scaffold-template/pkg/database"
)

const defaultMigrationTable = "schema_migrations"

var (
	// ErrNoVersion 表示数据库尚未应用任何 migration。
	ErrNoVersion = errors.New("database has no migration version")
	// ErrCompletionRequired 表示版本迁移完成后仍需要模块显式完成数据迁移。
	ErrCompletionRequired = errors.New("migration completion requires explicit operator input")
)

// Set 描述一个模块拥有的、按 driver 分离的嵌入 SQL 集合。
type Set struct {
	Name            string
	FS              fs.FS
	DriverPaths     map[pkgdatabase.Driver]string
	CurrentVersion  uint
	SHA256ByFile    map[string]string
	MigrationsTable string
}

// Config 是 Adapter 构造所需的数据库与有界执行参数。
type Config struct {
	Database    pkgdatabase.Config
	LockTimeout time.Duration
}

// Status 是不含 DSN 和 SQL 内容的迁移状态。
type Status struct {
	Version uint
	Dirty   bool
	Empty   bool
}

// Runner 拥有本次命令专用的连接、source 和 database driver。
type Runner struct {
	migration *golangmigrate.Migrate
	closeOnce sync.Once
	closeErr  error
	mu        sync.Mutex
}

// New 校验所有嵌入文件 checksum 后创建独立迁移资源。
func New(ctx context.Context, config Config, set Set) (*Runner, error) {
	if ctx == nil {
		return nil, fmt.Errorf("migration context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := pkgdatabase.ValidateConfig(&config.Database); err != nil {
		return nil, fmt.Errorf("validate migration database config: %w", err)
	}
	if config.LockTimeout <= 0 {
		return nil, fmt.Errorf("migration lock timeout must be positive")
	}
	if err := validateSet(set, config.Database.Driver); err != nil {
		return nil, err
	}
	driverPath := set.DriverPaths[config.Database.Driver]
	sourceDriver, err := iofs.New(set.FS, driverPath)
	if err != nil {
		return nil, fmt.Errorf("open migration source %s/%s: %w", set.Name, config.Database.Driver, err)
	}
	connection, driverName, err := openDatabase(config.Database)
	if err != nil {
		_ = sourceDriver.Close()
		return nil, err
	}
	table := set.MigrationsTable
	if table == "" {
		table = defaultMigrationTable
	}
	databaseDriver, err := migrationDriver(config.Database.Driver, connection, table)
	if err != nil {
		_ = sourceDriver.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("create migration database driver: %w", sanitize(err))
	}
	migration, err := golangmigrate.NewWithInstance("iofs", sourceDriver, driverName, databaseDriver)
	if err != nil {
		_ = sourceDriver.Close()
		_ = databaseDriver.Close()
		return nil, fmt.Errorf("create migration runner: %w", sanitize(err))
	}
	migration.LockTimeout = config.LockTimeout
	return &Runner{migration: migration}, nil
}

// Status 读取当前版本与 dirty 标志，不修改业务 schema。
func (r *Runner) Status(ctx context.Context) (Status, error) {
	if err := validateRunnerContext(r, ctx); err != nil {
		return Status{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	version, dirty, err := r.migration.Version()
	if errors.Is(err, golangmigrate.ErrNilVersion) {
		return Status{Empty: true}, nil
	}
	if err != nil {
		return Status{}, fmt.Errorf("read migration version: %w", sanitize(err))
	}
	return Status{Version: version, Dirty: dirty}, nil
}

// Up 应用所有尚未执行的 up migration；无变化视为幂等成功。
func (r *Runner) Up(ctx context.Context) error {
	if err := validateRunnerContext(r, ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.run(ctx, r.migration.Up)
}

func (r *Runner) run(ctx context.Context, operation func() error) error {
	result := make(chan error, 1)
	go func() { result <- operation() }()
	select {
	case err := <-result:
		if errors.Is(err, golangmigrate.ErrNoChange) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("execute migration: %w", classify(err))
		}
		return nil
	case <-ctx.Done():
		select {
		case r.migration.GracefulStop <- true:
		default:
		}
		err := <-result
		return errors.Join(ctx.Err(), classify(err))
	}
}

// Close 关闭 source 和本次命令专用数据库连接。
func (r *Runner) Close() error {
	if r == nil || r.migration == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		sourceErr, databaseErr := r.migration.Close()
		r.closeErr = errors.Join(sanitize(sourceErr), sanitize(databaseErr))
	})
	return r.closeErr
}

func validateRunnerContext(r *Runner, ctx context.Context) error {
	if r == nil || r.migration == nil {
		return fmt.Errorf("migration runner is nil")
	}
	if ctx == nil {
		return fmt.Errorf("migration context is nil")
	}
	return ctx.Err()
}

func validateSet(set Set, driver pkgdatabase.Driver) error {
	if strings.TrimSpace(set.Name) == "" || set.FS == nil || set.CurrentVersion == 0 {
		return fmt.Errorf("migration set is incomplete")
	}
	if strings.TrimSpace(set.DriverPaths[driver]) == "" {
		return fmt.Errorf("migration set %s has no %s path", set.Name, driver)
	}
	files := make([]string, 0, len(set.SHA256ByFile))
	seenPaths := make(map[string]struct{}, len(set.DriverPaths))
	for _, driverPath := range set.DriverPaths {
		driverPath = strings.TrimSpace(driverPath)
		if driverPath == "" {
			return fmt.Errorf("migration set %s contains an empty driver path", set.Name)
		}
		if _, exists := seenPaths[driverPath]; exists {
			continue
		}
		seenPaths[driverPath] = struct{}{}
		entries, err := fs.ReadDir(set.FS, driverPath)
		if err != nil {
			return fmt.Errorf("read migration set %s/%s: %w", set.Name, driverPath, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
				continue
			}
			files = append(files, path.Join(driverPath, entry.Name()))
		}
	}
	sort.Strings(files)
	if len(files) == 0 || len(set.SHA256ByFile) != len(files) {
		return fmt.Errorf("migration set %s checksum manifest is incomplete", set.Name)
	}
	for _, filename := range files {
		payload, err := fs.ReadFile(set.FS, filename)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", filename, err)
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(payload))
		if expected := set.SHA256ByFile[filename]; expected == "" || checksum != expected {
			return fmt.Errorf("migration file %s checksum mismatch", filename)
		}
	}
	return nil
}

func openDatabase(config pkgdatabase.Config) (*sql.DB, string, error) {
	var sqlDriver string
	switch config.Driver {
	case pkgdatabase.DriverSQLite:
		sqlDriver = "sqlite"
	case pkgdatabase.DriverPostgres:
		sqlDriver = "pgx"
	case pkgdatabase.DriverMySQL:
		sqlDriver = "mysql"
	default:
		return nil, "", fmt.Errorf("unsupported migration database driver")
	}
	connection, err := sql.Open(sqlDriver, config.DSN)
	if err != nil {
		return nil, "", fmt.Errorf("open migration database: %w", sanitize(err))
	}
	connection.SetMaxOpenConns(1)
	connection.SetMaxIdleConns(1)
	return connection, string(config.Driver), nil
}

func migrationDriver(driver pkgdatabase.Driver, connection *sql.DB, table string) (database.Driver, error) {
	switch driver {
	case pkgdatabase.DriverSQLite:
		return newSQLiteDriver(connection, table)
	case pkgdatabase.DriverPostgres:
		return postgres.WithInstance(connection, &postgres.Config{MigrationsTable: table})
	case pkgdatabase.DriverMySQL:
		return mysql.WithInstance(connection, &mysql.Config{MigrationsTable: table})
	default:
		return nil, fmt.Errorf("unsupported migration database driver")
	}
}

func classify(err error) error {
	if err == nil {
		return nil
	}
	var dirty golangmigrate.ErrDirty
	switch {
	case errors.As(err, &dirty):
		return fmt.Errorf("migration database is dirty at version %d", dirty.Version)
	case errors.Is(err, golangmigrate.ErrLockTimeout), errors.Is(err, golangmigrate.ErrLocked):
		return fmt.Errorf("migration lock unavailable")
	default:
		return sanitize(err)
	}
}

func sanitize(err error) error {
	if err == nil {
		return nil
	}
	return errors.New("migration operation failed")
}
