package composition

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	cacheapp "github.com/rin721/go-scaffold2/internal/kernel/app/cache"
	storageapp "github.com/rin721/go-scaffold2/internal/kernel/app/storage"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgcache "github.com/rin721/go-scaffold2/pkg/cache"
	pkgi18n "github.com/rin721/go-scaffold2/pkg/i18n"
	pkgstorage "github.com/rin721/go-scaffold2/pkg/storage"
)

func TestComposeSwapsI18nAndStorageAndPreflightsCacheRestart(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	databasePath := filepath.Join(directory, "app.db")
	storageV1 := filepath.Join(directory, "storage-v1")
	storageV2 := filepath.Join(directory, "storage-v2")
	storageInvalid := filepath.Join(directory, "storage-invalid")
	storageRestart := filepath.Join(directory, "storage-restart")
	writeCapabilityConfig(t, configPath, databasePath, "disabled", "error", "[]", storageV1)

	runtime := newTestRuntime(t, config.FileSource(configPath))
	capabilities, err := Compose(runtime, Options{})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	initialI18n := capabilities.I18n
	initialStorage := capabilities.Storage
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Stop(context.Background()) })

	if _, err := capabilities.I18n.Translate("zh-CN", pkgi18n.Text("missing")); err == nil {
		t.Fatal("initial I18n missing message error = nil")
	}
	if err := capabilities.Cache.Ping(t.Context()); !errors.Is(err, pkgcache.ErrDisabled) {
		t.Fatalf("Cache.Ping() error = %v, want ErrDisabled", err)
	}
	typedCache, err := cacheapp.NewClient[string](capabilities.Cache, &pkgcache.Config{DefaultTTL: time.Minute})
	if err != nil {
		t.Fatalf("cacheapp.NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = typedCache.Close() })
	if err := typedCache.Set(t.Context(), "key", "value"); !errors.Is(err, pkgcache.ErrDisabled) {
		t.Fatalf("typed Cache Set() error = %v, want ErrDisabled", err)
	}
	putStorageValue(t, capabilities.Storage, "v1.txt", "v1")

	writeCapabilityConfig(t, configPath, databasePath, "disabled", "use-id", "[]", storageV2)
	result, err := runtime.Reload(t.Context())
	if err != nil {
		t.Fatalf("Reload(v2) error = %v", err)
	}
	if !result.Applied {
		t.Fatalf("Reload(v2) result = %#v, want applied", result)
	}
	if capabilities.I18n != initialI18n || capabilities.Storage != initialStorage {
		t.Fatal("stable I18n or Storage facade identity changed")
	}
	text, err := capabilities.I18n.Translate("zh-CN", pkgi18n.Text("missing"))
	if err != nil || text != "missing" {
		t.Fatalf("I18n after reload text=%q error=%v", text, err)
	}
	putStorageValue(t, capabilities.Storage, "v2.txt", "v2")
	if _, err := os.Stat(filepath.Join(storageV2, "v2.txt")); err != nil {
		t.Fatalf("stat v2 storage object: %v", err)
	}

	missingMessages := "[" + yamlString(filepath.Join(directory, "missing.zh-CN.yaml")) + "]"
	writeCapabilityConfig(t, configPath, databasePath, "disabled", "error", missingMessages, storageInvalid)
	result, err = runtime.Reload(t.Context())
	if err == nil {
		t.Fatal("Reload(invalid i18n) error = nil")
	}
	if result.Applied {
		t.Fatalf("Reload(invalid i18n) result = %#v, want not applied", result)
	}
	if _, err := os.Stat(storageInvalid); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid candidate created storage path: %v", err)
	}
	text, err = capabilities.I18n.Translate("zh-CN", pkgi18n.Text("missing"))
	if err != nil || text != "missing" {
		t.Fatalf("I18n changed after invalid candidate text=%q error=%v", text, err)
	}
	putStorageValue(t, capabilities.Storage, "after-invalid.txt", "v2")

	writeCapabilityConfig(t, configPath, databasePath, "redis", "error", "[]", storageRestart)
	result, err = runtime.Reload(t.Context())
	if !errors.Is(err, app.ErrRestartRequired) {
		t.Fatalf("Reload(cache change) error = %v, want ErrRestartRequired", err)
	}
	if result.Applied || len(result.RestartRequired) != 1 || result.RestartRequired[0] != cacheapp.ID {
		t.Fatalf("Reload(cache change) result = %#v", result)
	}
	if _, err := os.Stat(storageRestart); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("restart-required reload created rejected storage path: %v", err)
	}
	text, err = capabilities.I18n.Translate("zh-CN", pkgi18n.Text("missing"))
	if err != nil || text != "missing" {
		t.Fatalf("I18n changed during rejected reload text=%q error=%v", text, err)
	}
	putStorageValue(t, capabilities.Storage, "still-v2.txt", "v2")
}

func putStorageValue(t *testing.T, access storageapp.Access, key string, value string) {
	t.Helper()
	if err := access.Use(t.Context(), storageapp.RoutePrimary, func(client storageapp.Client) error {
		return client.Put(t.Context(), key, []byte(value), pkgstorage.PutOptions{ContentType: "text/plain"})
	}); err != nil {
		t.Fatalf("Storage.Use(%s) error = %v", key, err)
	}
}

func writeCapabilityConfig(t *testing.T, path string, databasePath string, cacheDriver string, missingBehavior string, messageFiles string, storagePath string) {
	t.Helper()
	payload := fmt.Sprintf(`database:
  driver: sqlite
  dsn: %s
  pool:
    maxOpenConns: 1
    maxIdleConns: 1
  pingTimeout: 1s
cache:
  driver: %s
i18n:
  defaultLanguage: zh-CN
  messageFiles: %s
  missingBehavior: %s
storage:
  driver: local
  local:
    basePath: %s
`, yamlString(databasePath), cacheDriver, messageFiles, missingBehavior, yamlString(storagePath))
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write capability config: %v", err)
	}
}
