package composition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel"
	kernelcomposition "github.com/rin721/go-scaffold-template/internal/kernel/composition"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold-template/internal/kernel/logging"
	configbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/config"
	modelbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/model"
	todoservice "github.com/rin721/go-scaffold-template/internal/module/todo/service"
	pkgdatabase "github.com/rin721/go-scaffold-template/pkg/database"
	"github.com/rin721/go-scaffold-template/pkg/logger"
)

func TestApplicationGenerationKeepsTerminalCleanupFailure(t *testing.T) {
	want := errors.New("cleanup failed")
	generation := &applicationGeneration{committed: true, settled: true, terminalErr: want}

	if err := generation.Stop(t.Context()); !errors.Is(err, want) {
		t.Fatalf("Stop() error = %v, want terminal cleanup failure", err)
	}
	if err := generation.Retire(t.Context()); !errors.Is(err, want) {
		t.Fatalf("Retire() error = %v, want terminal cleanup failure", err)
	}
}

func TestApplicationGenerationReloadsTodoAndHTTPWithoutRestart(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	payload := generationConfig(directory, 120, 1<<20, filepath.Join(directory, "todo.db"))
	writeGenerationConfig(t, configPath, payload)

	coordinator, _ := newGenerationTestCoordinator(t, configPath)
	if err := coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := coordinator.Stop(ctx); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})
	initial := coordinator.Diagnostics()
	if initial.CurrentGeneration != 1 || initial.ConfiguredAddress != "127.0.0.1:0" || initial.BoundAddress == "" ||
		initial.RestartPolicy != "" || fmt.Sprint(initial.ResourceBuilt) != fmt.Sprint([]string{"logger", "database", "cache", "i18n", "storage", "todo", "http"}) {
		t.Fatalf("initial diagnostics = %#v", initial)
	}
	if status := createTodo(t, initial.BoundAddress, strings.Repeat("x", 100)); status != http.StatusCreated {
		t.Fatalf("initial create status = %d, want 201", status)
	}
	largeHeader := strings.Repeat("h", 1280<<10)
	if status := listTodosWithHeader(t, initial.BoundAddress, largeHeader); status != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("initial large-header status = %d, want 431", status)
	}

	updated := generationConfig(directory, 80, 2<<20, filepath.Join(directory, "todo.db"))
	writeGenerationConfig(t, configPath, updated)
	result, err := coordinator.Reload(t.Context())
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if !result.Applied || result.PreviousGeneration != 1 || result.CurrentGeneration != 2 {
		t.Fatalf("Reload() result = %#v", result)
	}
	if fmt.Sprint(result.ChangedSections) != fmt.Sprint([]string{"http", "todo"}) {
		t.Fatalf("changed sections = %#v", result.ChangedSections)
	}
	after := coordinator.Diagnostics()
	if after.BoundAddress != initial.BoundAddress || after.CurrentGeneration != 2 || !after.Ready {
		t.Fatalf("reloaded diagnostics = %#v, initial = %#v", after, initial)
	}
	if fmt.Sprint(after.ResourceReused) != fmt.Sprint([]string{"logger", "database", "cache", "i18n", "storage"}) ||
		fmt.Sprint(after.ResourceBuilt) != fmt.Sprint([]string{"todo", "http"}) {
		t.Fatalf("reloaded resource diagnostics = %#v", after)
	}
	if status := createTodo(t, after.BoundAddress, strings.Repeat("x", 100)); status != http.StatusBadRequest {
		t.Fatalf("reloaded create status = %d, want 400", status)
	}
	if status := listTodosWithHeader(t, after.BoundAddress, largeHeader); status != http.StatusOK {
		t.Fatalf("reloaded large-header status = %d, want 200", status)
	}
}

func TestAllConfigurationSectionsCommitOneGeneration(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	payload := generationConfig(directory, 120, 1<<20, filepath.Join(directory, "todo.db"))
	writeGenerationConfig(t, configPath, payload)
	coordinator, _ := newGenerationTestCoordinator(t, configPath)
	if err := coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = coordinator.Stop(ctx)
	}()
	nextDatabase := filepath.Join(directory, "next.db")
	prepareTodoSchema(t, nextDatabase)
	updated := strings.Replace(payload, "level: info", "level: debug", 1)
	updated = strings.Replace(updated, filepath.ToSlash(filepath.Join(directory, "todo.db")), filepath.ToSlash(nextDatabase), 1)
	updated = strings.Replace(updated, "tagPrefix: generation-a", "tagPrefix: generation-b", 1)
	updated = strings.Replace(updated, "defaultLanguage: zh-CN", "defaultLanguage: en-US", 1)
	updated = strings.Replace(updated, filepath.ToSlash(filepath.Join(directory, "storage")), filepath.ToSlash(filepath.Join(directory, "storage-next")), 1)
	updated = strings.Replace(updated, "maxHeaderBytes: 1048576", "maxHeaderBytes: 2097152", 1)
	updated = strings.Replace(updated, "titleMaxRunes: 120", "titleMaxRunes: 80", 1)
	writeGenerationConfig(t, configPath, updated)
	result, err := coordinator.Reload(t.Context())
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	wantSections := []string{"logger", "database", "cache", "i18n", "storage", "http", "todo"}
	if !result.Applied || result.PreviousGeneration != 1 || result.CurrentGeneration != 2 || fmt.Sprint(result.ChangedSections) != fmt.Sprint(wantSections) {
		t.Fatalf("Reload() result = %#v", result)
	}
	if diagnostics := coordinator.Diagnostics(); diagnostics.CurrentGeneration != 2 || diagnostics.CandidateGeneration != 0 || len(diagnostics.ResourceReused) != 0 {
		t.Fatalf("Diagnostics() = %#v", diagnostics)
	}
}

func TestApplicationGenerationRejectsDatabaseWithoutSchemaAndKeepsCurrent(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	writeGenerationConfig(t, configPath, generationConfig(directory, 120, 1<<20, filepath.Join(directory, "current.db")))
	coordinator, _ := newGenerationTestCoordinator(t, configPath)
	if err := coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = coordinator.Stop(ctx)
	}()
	before := coordinator.Diagnostics()
	writeGenerationConfig(t, configPath, generationConfig(directory, 120, 1<<20, filepath.Join(directory, "empty.db")))
	result, err := coordinator.Reload(t.Context())
	if err == nil || !strings.Contains(err.Error(), "schema table") || result.Applied {
		t.Fatalf("Reload(empty schema) = %#v, %v", result, err)
	}
	after := coordinator.Diagnostics()
	if after.CurrentGeneration != before.CurrentGeneration || after.BoundAddress != before.BoundAddress || !after.Ready {
		t.Fatalf("diagnostics changed after rejection: before=%#v after=%#v", before, after)
	}
	if status := listTodos(t, before.BoundAddress); status != http.StatusOK {
		t.Fatalf("old generation list status = %d", status)
	}
}

func TestApplicationGenerationPinsOldDatabaseUntilRetire(t *testing.T) {
	directory := t.TempDir()
	firstDatabase := filepath.Join(directory, "first.db")
	secondDatabase := filepath.Join(directory, "second.db")
	configPath := filepath.Join(directory, "config.yaml")
	writeGenerationConfig(t, configPath, generationConfig(directory, 120, 1<<20, firstDatabase))
	manager, err := kernellogging.New(logger.Noop())
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	factory, err := newApplicationGenerationFactory(manager)
	if err != nil {
		t.Fatalf("newApplicationGenerationFactory() error = %v", err)
	}
	loader := config.New(config.FileSource(configPath))
	firstSnapshot, err := loader.Load(t.Context())
	if err != nil {
		t.Fatalf("Load(first) error = %v", err)
	}
	firstPrepared, err := factory.Prepare(t.Context(), firstSnapshot, nil)
	if err != nil {
		t.Fatalf("Prepare(first) error = %v", err)
	}
	first := firstPrepared.(*applicationGeneration)
	firstActive, err := first.Commit(nil)
	if err != nil {
		t.Fatalf("Commit(first) error = %v", err)
	}
	if _, err := first.module.Service.Create(t.Context(), todoservice.CreateCommand{Title: "old database"}); err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}

	prepareTodoSchema(t, secondDatabase)
	writeGenerationConfig(t, configPath, generationConfig(directory, 5, 1<<20, secondDatabase))
	secondSnapshot, err := loader.Load(t.Context())
	if err != nil {
		t.Fatalf("Load(second) error = %v", err)
	}
	secondPrepared, err := factory.Prepare(t.Context(), secondSnapshot, firstActive)
	if err != nil {
		t.Fatalf("Prepare(second) error = %v", err)
	}
	second := secondPrepared.(*applicationGeneration)
	secondActive, err := second.Commit(firstActive)
	if err != nil {
		t.Fatalf("Commit(second) error = %v", err)
	}
	if _, err := first.module.Service.Create(t.Context(), todoservice.CreateCommand{Title: "1234567890"}); err != nil {
		t.Fatalf("old policy rejected old-generation request: %v", err)
	}
	if _, err := second.module.Service.Create(t.Context(), todoservice.CreateCommand{Title: "1234567890"}); err == nil {
		t.Fatal("new policy accepted over-limit new-generation request")
	}
	oldItems, err := first.module.Service.List(t.Context(), todoservice.ListQuery{})
	if err != nil {
		t.Fatalf("List(first) error = %v", err)
	}
	newItems, err := second.module.Service.List(t.Context(), todoservice.ListQuery{})
	if err != nil {
		t.Fatalf("List(second) error = %v", err)
	}
	if oldItems.Total != 2 || newItems.Total != 0 {
		t.Fatalf("database generations mixed: old=%d new=%d", oldItems.Total, newItems.Total)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := first.Retire(ctx); err != nil {
		t.Fatalf("Retire(first) error = %v", err)
	}
	if err := secondActive.Stop(ctx); err != nil {
		t.Fatalf("Stop(second) error = %v", err)
	}
	if err := factory.Stop(ctx); err != nil {
		t.Fatalf("factory.Stop() error = %v", err)
	}
}

func TestApplicationGenerationReusesUnchangedTypedResources(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	writeGenerationConfig(t, configPath, generationConfig(directory, 120, 1<<20, filepath.Join(directory, "todo.db")))
	manager, err := kernellogging.New(logger.Noop())
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	factory, err := newApplicationGenerationFactory(manager)
	if err != nil {
		t.Fatalf("newApplicationGenerationFactory() error = %v", err)
	}
	loader := config.New(config.FileSource(configPath))
	firstSnapshot, err := loader.Load(t.Context())
	if err != nil {
		t.Fatalf("Load(first) error = %v", err)
	}
	firstPrepared, err := factory.Prepare(t.Context(), firstSnapshot, nil)
	if err != nil {
		t.Fatalf("Prepare(first) error = %v", err)
	}
	first := firstPrepared.(*applicationGeneration)
	firstActive, err := first.Commit(nil)
	if err != nil {
		t.Fatalf("Commit(first) error = %v", err)
	}

	writeGenerationConfig(t, configPath, generationConfig(directory, 80, 1<<20, filepath.Join(directory, "todo.db")))
	secondSnapshot, err := loader.Load(t.Context())
	if err != nil {
		t.Fatalf("Load(second) error = %v", err)
	}
	secondPrepared, err := factory.Prepare(t.Context(), secondSnapshot, firstActive)
	if err != nil {
		t.Fatalf("Prepare(second) error = %v", err)
	}
	second := secondPrepared.(*applicationGeneration)
	if first.logger.entry != second.logger.entry || first.database.entry != second.database.entry ||
		first.cache.entry != second.cache.entry || first.i18n.entry != second.i18n.entry || first.storage.entry != second.storage.entry {
		t.Fatal("Todo-only candidate rebuilt an unchanged typed resource")
	}
	secondActive, err := second.Commit(firstActive)
	if err != nil {
		t.Fatalf("Commit(second) error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := first.Retire(ctx); err != nil {
		t.Fatalf("Retire(first) error = %v", err)
	}
	if err := secondActive.Stop(ctx); err != nil {
		t.Fatalf("Stop(second) error = %v", err)
	}
	if err := factory.Stop(ctx); err != nil {
		t.Fatalf("factory.Stop() error = %v", err)
	}
}

func TestEachConfigurationSectionCreatesOneCompleteGeneration(t *testing.T) {
	tests := []struct {
		section string
		update  func(*testing.T, string, string) string
	}{
		{section: "logger", update: replaceConfig("level: info", "level: debug")},
		{section: "database", update: func(t *testing.T, directory, payload string) string {
			target := filepath.Join(directory, "next.db")
			prepareTodoSchema(t, target)
			return strings.Replace(payload, filepath.ToSlash(filepath.Join(directory, "todo.db")), filepath.ToSlash(target), 1)
		}},
		{section: "cache", update: replaceConfig("tagPrefix: generation-a", "tagPrefix: generation-b")},
		{section: "i18n", update: replaceConfig("defaultLanguage: zh-CN", "defaultLanguage: en-US")},
		{section: "storage", update: func(_ *testing.T, directory, payload string) string {
			return strings.Replace(payload, filepath.ToSlash(filepath.Join(directory, "storage")), filepath.ToSlash(filepath.Join(directory, "storage-next")), 1)
		}},
		{section: "http", update: replaceConfig("maxHeaderBytes: 1048576", "maxHeaderBytes: 2097152")},
		{section: "todo", update: replaceConfig("titleMaxRunes: 120", "titleMaxRunes: 80")},
	}
	for _, test := range tests {
		t.Run(test.section, func(t *testing.T) {
			directory := t.TempDir()
			configPath := filepath.Join(directory, "config.yaml")
			payload := generationConfig(directory, 120, 1<<20, filepath.Join(directory, "todo.db"))
			writeGenerationConfig(t, configPath, payload)
			coordinator, _ := newGenerationTestCoordinator(t, configPath)
			if err := coordinator.Start(t.Context()); err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			writeGenerationConfig(t, configPath, test.update(t, directory, payload))
			result, err := coordinator.Reload(t.Context())
			if err != nil {
				t.Fatalf("Reload(%s) error = %v", test.section, err)
			}
			if !result.Applied || fmt.Sprint(result.ChangedSections) != fmt.Sprint([]string{test.section}) {
				t.Fatalf("Reload(%s) result = %#v", test.section, result)
			}
			if status := listTodos(t, coordinator.Diagnostics().BoundAddress); status != http.StatusOK {
				t.Fatalf("list after %s reload status = %d", test.section, status)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := coordinator.Stop(ctx); err != nil {
				t.Fatalf("Stop() error = %v", err)
			}
		})
	}
}

func TestApplicationGenerationAddressChangeAndBindFailure(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	payload := generationConfig(directory, 120, 1<<20, filepath.Join(directory, "todo.db"))
	writeGenerationConfig(t, configPath, payload)
	coordinator, _ := newGenerationTestCoordinator(t, configPath)
	if err := coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = coordinator.Stop(ctx)
	}()
	oldAddress := coordinator.Diagnostics().BoundAddress

	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve address: %v", err)
	}
	blockedAddress := reserved.Addr().String()
	blocked := strings.Replace(payload, "127.0.0.1:0", blockedAddress, 1)
	writeGenerationConfig(t, configPath, blocked)
	result, err := coordinator.Reload(t.Context())
	if err == nil || result.Applied {
		t.Fatalf("Reload(blocked address) = %#v, %v", result, err)
	}
	if status := listTodos(t, oldAddress); status != http.StatusOK {
		t.Fatalf("old address after bind rejection status = %d", status)
	}
	_ = reserved.Close()

	writeGenerationConfig(t, configPath, blocked)
	result, err = coordinator.Reload(t.Context())
	if err != nil || !result.Applied {
		t.Fatalf("Reload(new address) = %#v, %v", result, err)
	}
	newAddress := coordinator.Diagnostics().BoundAddress
	if newAddress != blockedAddress || newAddress == oldAddress {
		t.Fatalf("new address = %s, old = %s, configured = %s", newAddress, oldAddress, blockedAddress)
	}
	if status := listTodos(t, newAddress); status != http.StatusOK {
		t.Fatalf("new address status = %d", status)
	}
}

func TestApplicationGenerationWatcherRecoversAfterRejectedCandidate(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	payload := generationConfig(directory, 120, 1<<20, filepath.Join(directory, "todo.db"))
	writeGenerationConfig(t, configPath, payload)
	coordinator, _ := newGenerationTestCoordinator(t, configPath)
	if err := coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	watchCtx, cancelWatch := context.WithCancel(context.Background())
	ready := make(chan struct{})
	reloadErrors := make(chan error, 4)
	watchDone := make(chan error, 1)
	go func() {
		watchDone <- coordinator.Watch(watchCtx, func(err error) { reloadErrors <- err }, ready)
	}()
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not become ready")
	}
	writeGenerationConfig(t, configPath, "logger: [invalid\n")
	select {
	case err := <-reloadErrors:
		if err == nil {
			t.Fatal("rejected candidate error = nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not report invalid candidate")
	}
	writeGenerationConfig(t, configPath, strings.Replace(payload, "titleMaxRunes: 120", "titleMaxRunes: 80", 1))
	deadline := time.Now().Add(3 * time.Second)
	for coordinator.Diagnostics().CurrentGeneration != 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if diagnostics := coordinator.Diagnostics(); diagnostics.CurrentGeneration != 2 || !diagnostics.Ready {
		t.Fatalf("watcher did not recover: %#v", diagnostics)
	}
	cancelWatch()
	if err := <-watchDone; err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
	ctx, cancelStop := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelStop()
	if err := coordinator.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestServiceRuntimeReloadsInSameProcessAndStopsGracefully(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	payload := generationConfig(directory, 120, 1<<20, filepath.Join(directory, "todo.db"))
	writeGenerationConfig(t, configPath, payload)
	manager, err := kernellogging.New(logger.Noop())
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	application, err := New(Config{
		Name: "go-scaffold-template", Description: "runtime acceptance",
		ConfigPath: configPath, EnvironmentPrefix: "GO_SCAFFOLD_RELOAD_ACCEPTANCE_", Logging: manager,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	runtime, err := application.newServiceRuntime()
	if err != nil {
		t.Fatalf("newServiceRuntime() error = %v", err)
	}
	runCtx, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- runtime.supervisor.Run(runCtx) }()
	select {
	case <-runtime.supervisor.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("service runtime did not become ready")
	}
	initial := runtime.coordinator.Diagnostics()
	if status := createTodo(t, initial.BoundAddress, strings.Repeat("x", 100)); status != http.StatusCreated {
		t.Fatalf("initial create status = %d", status)
	}
	writeGenerationConfig(t, configPath, strings.Replace(payload, "titleMaxRunes: 120", "titleMaxRunes: 80", 1))
	deadline := time.Now().Add(5 * time.Second)
	for runtime.coordinator.Diagnostics().CurrentGeneration != 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	after := runtime.coordinator.Diagnostics()
	if after.CurrentGeneration != 2 || after.BoundAddress != initial.BoundAddress {
		t.Fatalf("runtime reload diagnostics: before=%#v after=%#v", initial, after)
	}
	if status := createTodo(t, after.BoundAddress, strings.Repeat("x", 100)); status != http.StatusBadRequest {
		t.Fatalf("reloaded create status = %d", status)
	}
	cancelRun()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("service runtime Run() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("service runtime did not stop gracefully")
	}
}

func newGenerationTestCoordinator(t *testing.T, configPath string) (*kernel.GenerationCoordinator, *applicationGenerationFactory) {
	t.Helper()
	manager, err := kernellogging.New(logger.Noop())
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	factory, err := newApplicationGenerationFactory(manager)
	if err != nil {
		t.Fatalf("newApplicationGenerationFactory() error = %v", err)
	}
	bindings, err := kernelcomposition.ConfigurationBindings(configbinding.Binding())
	if err != nil {
		t.Fatalf("ConfigurationBindings() error = %v", err)
	}
	coordinator, err := kernel.NewGenerationCoordinator(
		config.New(config.FileSource(configPath)), bindings, factory,
		kernel.Options{Logging: manager, ReloadTimeout: 5 * time.Second, Debounce: 10 * time.Millisecond},
	)
	if err != nil {
		t.Fatalf("NewGenerationCoordinator() error = %v", err)
	}
	return coordinator, factory
}

func generationConfig(directory string, titleMax, maxHeaderBytes int, databasePath string) string {
	return fmt.Sprintf(`logger:
  environment: development
  level: info
database:
  driver: sqlite
  dsn: %q
cache:
  driver: disabled
  redis:
    tagPrefix: generation-a
i18n:
  defaultLanguage: zh-CN
  messageFiles: []
  missingBehavior: error
storage:
  driver: local
  local:
    basePath: %q
todo:
  titleMaxRunes: %d
  defaultListLimit: 20
  maxListLimit: 100
http:
  addr: 127.0.0.1:0
  maxHeaderBytes: %d
`, filepath.ToSlash(databasePath), filepath.ToSlash(filepath.Join(directory, "storage")), titleMax, maxHeaderBytes)
}

func replaceConfig(oldValue, newValue string) func(*testing.T, string, string) string {
	return func(t *testing.T, _ string, payload string) string {
		t.Helper()
		updated := strings.Replace(payload, oldValue, newValue, 1)
		if updated == payload {
			t.Fatalf("config value %q was not found", oldValue)
		}
		return updated
	}
}

func prepareTodoSchema(t *testing.T, path string) {
	t.Helper()
	cfg := pkgdatabase.DefaultConfig()
	cfg.Driver = pkgdatabase.DriverSQLite
	cfg.DSN = filepath.ToSlash(path)
	resource, err := pkgdatabase.NewGORM(t.Context(), &cfg)
	if err != nil {
		t.Fatalf("NewGORM(%s) error = %v", path, err)
	}
	if err := resource.Client().Migrate(t.Context(), modelbinding.Schema()); err != nil {
		_ = resource.Close()
		t.Fatalf("Migrate(%s) error = %v", path, err)
	}
	if err := resource.Close(); err != nil {
		t.Fatalf("Close(%s) error = %v", path, err)
	}
}

func writeGenerationConfig(t *testing.T, path, payload string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func createTodo(t *testing.T, address, title string) int {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"title": title})
	request, err := http.NewRequest(http.MethodPost, "http://"+address+"/api/v1/todos", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	return doGenerationRequest(t, request)
}

func listTodos(t *testing.T, address string) int {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, "http://"+address+"/api/v1/todos", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	return doGenerationRequest(t, request)
}

func listTodosWithHeader(t *testing.T, address, value string) int {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, "http://"+address+"/api/v1/todos", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.Header.Set("X-Reload-Acceptance", value)
	return doGenerationRequest(t, request)
}

func doGenerationRequest(t *testing.T, request *http.Request) int {
	t.Helper()
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("HTTP request error = %v", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	return response.StatusCode
}
