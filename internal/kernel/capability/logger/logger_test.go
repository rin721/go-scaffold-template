package logger

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

func TestDefaultsPreserveEnvironmentDerivedOptions(t *testing.T) {
	manager, err := config.NewDefaultManager(config.Binding{
		CapabilityID: string(ID),
		ConfigPath:   ConfigPath,
		Contract:     capability{},
	})
	if err != nil {
		t.Fatalf("NewDefaultManager() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "logger.yaml")
	if _, err := manager.Generate(t.Context(), config.GenerateRequest{Path: path}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, expected := range [][]byte{
		[]byte("logger:"),
		[]byte("environment: development"),
		[]byte("level: info"),
		[]byte("outputPaths:"),
		[]byte("- stdout"),
		[]byte("errorOutputPaths:"),
		[]byte("- stderr"),
	} {
		if !bytes.Contains(payload, expected) {
			t.Fatalf("generated defaults missing %q:\n%s", expected, payload)
		}
	}
	for _, derived := range [][]byte{[]byte("encoding:"), []byte("addCaller:"), []byte("addStacktrace:")} {
		if bytes.Contains(payload, derived) {
			t.Fatalf("generated defaults unexpectedly fix derived field %q:\n%s", derived, payload)
		}
	}
}

func TestDefinitionPublishesNarrowAccessAndRestoresBaseline(t *testing.T) {
	baseline := pkglogger.NewTestLogger()
	manager, err := kernellogging.New(baseline)
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "configured.log")
	runtime, err := kernel.New(config.New(config.MapSource("logger", map[string]any{
		"logger": map[string]any{
			"environment":      "production",
			"level":            "info",
			"encoding":         "json",
			"outputPaths":      []any{path},
			"errorOutputPaths": []any{path},
			"addCaller":        false,
			"addStacktrace":    false,
		},
	})), kernel.Options{Logging: manager})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	registration, err := kernel.Register(runtime, Definition(manager))
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	access, err := NewAccess(registration.Access)
	if err != nil {
		t.Fatalf("NewAccess() error = %v", err)
	}
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := access.Use(t.Context(), func(log pkglogger.Logger) error {
		log.Info("business log")
		return nil
	}); err != nil {
		t.Fatalf("Access.Use() error = %v", err)
	}
	manager.Info("kernel log")
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	manager.Info("baseline restored")
	if err := access.Use(t.Context(), func(pkglogger.Logger) error { return nil }); !errors.Is(err, kernel.ErrStopped) {
		t.Fatalf("Access.Use() after stop error = %v, want ErrStopped", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, message := range [][]byte{[]byte("business log"), []byte("kernel log"), []byte("kernel started")} {
		if !bytes.Contains(payload, message) {
			t.Fatalf("configured log missing %q:\n%s", message, payload)
		}
	}
	if !containsMessage(baseline.Entries(), "baseline restored") || !containsMessage(baseline.Entries(), "kernel stopped") {
		t.Fatalf("baseline entries = %#v", baseline.Entries())
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove() after Kernel Stop error = %v", err)
	}
}

func TestInvalidConfigNeverReplacesBaseline(t *testing.T) {
	baseline := pkglogger.NewTestLogger()
	manager, err := kernellogging.New(baseline)
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	runtime, err := kernel.New(config.New(config.MapSource("logger", map[string]any{
		"logger": map[string]any{"environment": "invalid"},
	})), kernel.Options{Logging: manager})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	if _, err := kernel.Register(runtime, Definition(manager)); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := runtime.Start(t.Context()); err == nil {
		t.Fatal("Start() error = nil")
	}
	manager.Info("still baseline")
	if !containsMessage(baseline.Entries(), "still baseline") {
		t.Fatalf("baseline entries = %#v", baseline.Entries())
	}
	if err := runtime.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestDefinitionRejectsMissingManagerBeforeActivation(t *testing.T) {
	manager, err := kernellogging.New(pkglogger.Noop())
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	runtime, err := kernel.New(config.New(config.MapSource("logger", map[string]any{})), kernel.Options{Logging: manager})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	if _, err := kernel.Register(runtime, Definition(nil)); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := runtime.Start(t.Context()); err == nil {
		t.Fatal("Start() error = nil, want missing manager error")
	}
	if err := runtime.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func containsMessage(entries []pkglogger.Entry, message string) bool {
	for _, entry := range entries {
		if entry.Message == message {
			return true
		}
	}
	return false
}
