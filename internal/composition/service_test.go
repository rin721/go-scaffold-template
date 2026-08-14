package composition

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/app"
	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
	"github.com/rin721/go-scaffold2/pkg/logger"
)

func TestExampleConfigSatisfiesApplicationBindings(t *testing.T) {
	manager, err := kernellogging.New(logger.Noop())
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	application, err := New(Config{
		Name: "go-scaffold2", Description: "test application",
		ConfigPath:        filepath.Join("..", "..", "config.example.yaml"),
		EnvironmentPrefix: "GO_SCAFFOLD2_TEST_014_EXAMPLE_", Logging: manager,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	prepared, err := application.prepareTodo(t.Context())
	if err != nil {
		t.Fatalf("prepareTodo(config.example.yaml) error = %v", err)
	}
	if prepared.module.Service == nil || prepared.coordinator == nil {
		t.Fatalf("prepared application = %#v", prepared)
	}
}

func TestTodoConfigurationChangeRequiresRestart(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	payload := fmt.Sprintf(`logger:
  environment: development
  level: info
database:
  driver: sqlite
  dsn: %q
cache:
  driver: disabled
i18n:
  defaultLanguage: zh-CN
  messageFiles: []
  missingBehavior: error
storage:
  driver: local
  local:
    basePath: %q
todo:
  titleMaxRunes: 120
  defaultListLimit: 20
  maxListLimit: 100
http:
  addr: 127.0.0.1:0
`, filepath.ToSlash(filepath.Join(directory, "todo.db")), filepath.ToSlash(filepath.Join(directory, "storage")))
	if err := os.WriteFile(configPath, []byte(payload), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	manager, err := kernellogging.New(logger.Noop())
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	application, err := New(Config{
		Name: "go-scaffold2", Description: "test application", ConfigPath: configPath,
		EnvironmentPrefix: "GO_SCAFFOLD2_TEST_014_RELOAD_", Logging: manager,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	prepared, err := application.prepareTodo(t.Context())
	if err != nil {
		t.Fatalf("prepareTodo() error = %v", err)
	}
	if err := prepared.coordinator.Start(t.Context()); err != nil {
		t.Fatalf("coordinator.Start() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.coordinator.Stop(context.Background()) })

	updated := strings.Replace(payload, "titleMaxRunes: 120", "titleMaxRunes: 80", 1)
	if err := os.WriteFile(configPath, []byte(updated), 0o600); err != nil {
		t.Fatalf("update config: %v", err)
	}
	result, err := prepared.coordinator.Reload(t.Context())
	if !errors.Is(err, app.ErrRestartRequired) || result.Applied ||
		len(result.RestartRequired) != 1 || result.RestartRequired[0] != "module.todo" {
		t.Fatalf("Reload() = %#v, %v; want Todo restart requirement", result, err)
	}
}

func TestApplicationLifecycleUsesInjectedLogger(t *testing.T) {
	log := logger.NewTestLogger()
	lifecycle := applicationLifecycle{applicationName: "test-app", logging: log}
	if err := lifecycle.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := lifecycle.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	entries := log.Entries()
	if len(entries) != 2 || entries[0].Message != "application started" || entries[1].Message != "application stopping" {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestReloadErrorReporterClassifiesAndRedacts(t *testing.T) {
	tests := []struct {
		err     error
		level   string
		message string
	}{
		{errors.New("candidate failed"), "error", "kernel reload rejected; previous configuration remains active"},
		{&app.RestartRequiredError{Components: []app.ID{"todo"}}, "warn", "kernel reload requires process restart; previous configuration remains active"},
		{&kernel.CommittedCleanupError{Err: errors.New("close failed")}, "error", "kernel reload applied but previous resources failed to close"},
	}
	for _, test := range tests {
		log := logger.NewTestLogger()
		reloadErrorReporter(log)(test.err)
		entries := log.Entries()
		if len(entries) != 1 || entries[0].Level != test.level || entries[0].Message != test.message {
			t.Fatalf("entries = %#v", entries)
		}
	}

	path := filepath.Join(t.TempDir(), "reload.log")
	addCaller := false
	resource, err := logger.New(&logger.Config{
		Environment: logger.EnvironmentProduction, OutputPaths: []string{path}, ErrorOutputPaths: []string{path}, AddCaller: &addCaller,
	})
	if err != nil {
		t.Fatalf("logger.New() error = %v", err)
	}
	secret := "postgres://user:top-secret@example.invalid/app"
	reloadErrorReporter(resource)(errors.New("connect " + secret))
	if err := resource.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if bytes.Contains(payload, []byte(secret)) || bytes.Contains(payload, []byte("top-secret")) {
		t.Fatalf("reload log leaked secret: %s", payload)
	}
}

func TestReloadErrorReporterIgnoresNilInputs(t *testing.T) {
	reloadErrorReporter(nil)(errors.New("ignored"))
	log := logger.NewTestLogger()
	reloadErrorReporter(log)(nil)
	if len(log.Entries()) != 0 {
		t.Fatalf("entries = %#v", log.Entries())
	}
}
