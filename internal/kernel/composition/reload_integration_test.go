package composition

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rin721/go-scaffold2/internal/kernel"
	databaseapp "github.com/rin721/go-scaffold2/internal/kernel/app/database"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
	pkgdatabase "github.com/rin721/go-scaffold2/pkg/database"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

func TestHostReloadsRealSQLiteAndKeepsCrossComponentTransactionAtomic(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	databaseV1 := filepath.Join(directory, "database-v1.db")
	databaseV2 := filepath.Join(directory, "database-v2.db")
	logV1 := filepath.Join(directory, "logger-v1.log")
	logRejected := filepath.Join(directory, "logger-rejected.log")
	logV2 := filepath.Join(directory, "logger-v2.log")
	writeReloadConfig(t, configPath, "info", logV1, "sqlite", databaseV1)

	logging, err := kernellogging.New(pkglogger.Noop())
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	runtimeKernel, err := kernel.New(config.New(config.FileSource(configPath)), kernel.Options{
		Logging:       logging,
		Debounce:      20 * time.Millisecond,
		ReloadTimeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	capabilities, err := Compose(runtimeKernel, Options{Logger: ConfiguredLoggerReplacement})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	initialClock := capabilities.Clock
	initialIDGenerator := capabilities.IDGenerator
	initialValidator := capabilities.Validator
	reloadErrors := make(chan error, 4)
	participant := &countingParticipant{name: "application"}
	host, err := kernel.NewHost(runtimeKernel, kernel.HostOptions{
		Watch: &kernel.WatchOptions{OnReloadError: func(err error) { reloadErrors <- err }},
	}, participant)
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- host.Run(ctx) }()
	if err := waitForDatabaseUse(t, capabilities.Database, done, "generation_v1"); err != nil {
		cancel()
		if errors.Is(err, pkgdatabase.ErrInvalidDriver) {
			t.Skipf("current committed Database baseline has no SQLite support: %v", err)
		}
		t.Fatalf("start real SQLite host: %v", err)
	}

	// Logger 候选可构造，但 Database 候选非法；整轮必须保留旧 Logger 与 Database。
	writeReloadConfig(t, configPath, "debug", logRejected, "oracle", databaseV2)
	select {
	case reloadErr := <-reloadErrors:
		if reloadErr == nil {
			t.Fatal("reload error callback received nil")
		}
	case hostErr := <-done:
		cancel()
		t.Fatalf("Host stopped after candidate rejection: %v", hostErr)
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("timed out waiting for rejected candidate")
	}
	capabilities.Logger.Info("after rejected reload")
	waitForFileText(t, logV1, "after rejected reload")
	assertFileOmitsText(t, logRejected, "after rejected reload")
	if err := useDatabaseMarker(t.Context(), capabilities.Database, "still_generation_v1"); err != nil {
		t.Fatalf("use Database after rejected reload: %v", err)
	}

	// 有效候选完成整轮提交后，同一稳定 Logger 与 Database Access 都转向 v2。
	writeReloadConfig(t, configPath, "debug", logV2, "sqlite", databaseV2)
	waitForFileText(t, logV2, "kernel reload completed")
	if err := useDatabaseMarker(t.Context(), capabilities.Database, "generation_v2"); err != nil {
		t.Fatalf("use Database after successful reload: %v", err)
	}
	capabilities.Logger.Info("after applied reload")
	waitForFileText(t, logV2, "after applied reload")
	if runtime.GOOS == "windows" {
		released := databaseV1 + ".released"
		if err := os.Rename(databaseV1, released); err != nil {
			t.Fatalf("previous Database resource remains locked after reload: %v", err)
		}
		databaseV1 = released
	}

	if capabilities.Clock.Now().IsZero() {
		t.Fatal("Clock became unusable after reload")
	}
	if capabilities.Clock != initialClock || capabilities.IDGenerator != initialIDGenerator || capabilities.Validator != initialValidator {
		t.Fatal("direct capability identity changed during reload")
	}
	if value, err := capabilities.IDGenerator.New(); err != nil || value == "" {
		t.Fatalf("IDGenerator.New() = %q, %v", value, err)
	}
	if err := capabilities.Validator.Struct(struct {
		Name string `validate:"required"`
	}{Name: "available"}); err != nil {
		t.Fatalf("Validator.Struct() after reload error = %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Host.Run() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Host did not stop after cancellation")
	}
	if participant.starts.Load() != 1 || participant.stops.Load() != 1 {
		t.Fatalf("application lifecycle counts = start:%d stop:%d", participant.starts.Load(), participant.stops.Load())
	}
	// SQLite 使用 WAL；只有当前代和旧代都关闭后，表结构才保证 checkpoint
	// 回主文件。此时文件归属可以证明同一个稳定 Access 的实际换代结果。
	waitForFileText(t, databaseV1, "generation_v1")
	waitForFileText(t, databaseV1, "still_generation_v1")
	assertFileOmitsText(t, databaseV1, "generation_v2")
	waitForFileText(t, databaseV2, "generation_v2")
	assertFileOmitsText(t, databaseV2, "generation_v1")
	if runtime.GOOS == "windows" {
		assertDatabaseFileReleased(t, databaseV2)
	}
}

type countingParticipant struct {
	name   string
	starts atomic.Int32
	stops  atomic.Int32
}

func (p *countingParticipant) Name() string { return p.name }
func (p *countingParticipant) Start(context.Context) error {
	p.starts.Add(1)
	return nil
}
func (p *countingParticipant) Stop(context.Context) error {
	p.stops.Add(1)
	return nil
}

func writeReloadConfig(t *testing.T, path, level, logPath, driver, dsn string) {
	t.Helper()
	payload := fmt.Sprintf(`logger:
  environment: production
  level: %s
  encoding: json
  outputPaths: [%s]
  errorOutputPaths: [%s]
  addCaller: false
  addStacktrace: false
database:
  driver: %s
  dsn: %s
  pool:
    maxOpenConns: 2
    maxIdleConns: 1
  pingTimeout: 1s
cache:
  driver: disabled
i18n:
  defaultLanguage: zh-CN
  messageFiles: []
  missingBehavior: error
storage:
  driver: local
  local:
    basePath: %s
`, level, yamlString(logPath), yamlString(logPath), driver, yamlString(dsn), yamlString(filepath.Join(filepath.Dir(path), "storage")))
	temporary := path + ".next"
	if err := os.WriteFile(temporary, []byte(payload), 0o600); err != nil {
		t.Fatalf("write config candidate: %v", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			t.Fatalf("remove previous config: %v", removeErr)
		}
		if retryErr := os.Rename(temporary, path); retryErr != nil {
			t.Fatalf("publish config candidate: %v", retryErr)
		}
	}
}

func yamlString(value string) string { return strconv.Quote(filepath.ToSlash(value)) }

func waitForDatabaseUse(t *testing.T, access databaseapp.Access, done <-chan error, marker string) error {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		err := useDatabaseMarker(ctx, access, marker)
		cancel()
		if err == nil {
			return nil
		}
		select {
		case hostErr := <-done:
			if hostErr != nil {
				return hostErr
			}
			return fmt.Errorf("host stopped before Database became available")
		default:
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("database access timeout: %w", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// useDatabaseMarker 使用反射只调用当前 Database Client 已公开的 Migrate 契约。
// 这样 009 不导入 008 正在演进的 Schema 类型，任务提交仍与其文件归属隔离。
func useDatabaseMarker(ctx context.Context, access databaseapp.Access, table string) error {
	return access.Use(ctx, func(client databaseapp.Client) error {
		method := reflect.ValueOf(client).MethodByName("Migrate")
		if !method.IsValid() {
			return fmt.Errorf("database client does not expose Migrate")
		}
		methodType := method.Type()
		if !methodType.IsVariadic() || methodType.NumIn() != 2 {
			return fmt.Errorf("database Migrate signature is incompatible")
		}
		schemaType := methodType.In(1).Elem()
		schema := reflect.New(schemaType).Elem()
		schema.FieldByName("Table").SetString(table)
		fields := reflect.MakeSlice(schema.FieldByName("Fields").Type(), 1, 1)
		field := fields.Index(0)
		field.FieldByName("Name").SetString("ID")
		field.FieldByName("Column").SetString("id")
		field.FieldByName("Type").SetString("int64")
		field.FieldByName("PrimaryKey").SetBool(true)
		schema.FieldByName("Fields").Set(fields)
		schemas := reflect.MakeSlice(methodType.In(1), 1, 1)
		schemas.Index(0).Set(schema)
		results := method.CallSlice([]reflect.Value{reflect.ValueOf(ctx), schemas})
		if len(results) != 1 || results[0].IsNil() {
			return nil
		}
		return results[0].Interface().(error)
	})
}

func waitForFileText(t *testing.T, path, expected string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		payload, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(payload), expected) {
			return
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read %s: %v", filepath.Base(path), err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s does not contain %q", filepath.Base(path), expected)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func assertFileOmitsText(t *testing.T, path, forbidden string) {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read %s: %v", filepath.Base(path), err)
	}
	if strings.Contains(string(payload), forbidden) {
		t.Fatalf("%s unexpectedly contains %q", filepath.Base(path), forbidden)
	}
}

func assertDatabaseFileReleased(t *testing.T, path string) {
	t.Helper()
	released := path + ".released"
	if err := os.Rename(path, released); err != nil {
		t.Fatalf("database file %s remains locked after Host stop: %v", filepath.Base(path), err)
	}
}
