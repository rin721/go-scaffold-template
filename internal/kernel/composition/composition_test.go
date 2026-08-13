package composition

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
)

func TestKernelNewDoesNotComposeCapabilities(t *testing.T) {
	runtime := newTestRuntime(t, config.MapSource("empty", map[string]any{}))
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() without Compose error = %v", err)
	}
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestComposeMakesCLIExplicitlyOptional(t *testing.T) {
	disabledRuntime := newTestRuntime(t, config.MapSource("empty", map[string]any{}))
	disabled, err := Compose(disabledRuntime, Options{})
	if err != nil {
		t.Fatalf("Compose(disabled) error = %v", err)
	}
	if disabled.Logger == nil || disabled.Clock == nil || disabled.IDGenerator == nil || disabled.Validator == nil || disabled.Database == nil || disabled.Configuration == nil || disabled.CLI != nil {
		t.Fatalf("Compose(disabled) = %#v", disabled)
	}
	if disabled.Logger != disabledRuntime.Logger() {
		t.Fatal("Compose(disabled) did not return the Kernel builtin Logger facade")
	}
	if _, err := disabled.IDGenerator.New(); err != nil {
		t.Fatalf("IDGenerator.New() error = %v", err)
	}
	if err := disabled.Validator.Struct(struct {
		Name string `validate:"required"`
	}{Name: "ready"}); err != nil {
		t.Fatalf("Validator.Struct() error = %v", err)
	}
	if disabled.Clock.Now().IsZero() {
		t.Fatal("Clock.Now() returned zero time")
	}

	enabledRuntime := newTestRuntime(t, config.MapSource("empty", map[string]any{}))
	enabled, err := Compose(enabledRuntime, Options{
		Logger: ConfiguredLoggerReplacement,
		CLI: &CLIOptions{App: pkgcli.Config{
			Name:                   "test",
			DisableInteractiveHome: true,
		}}})
	if err != nil {
		t.Fatalf("Compose(enabled) error = %v", err)
	}
	if enabled.CLI == nil {
		t.Fatal("Compose(enabled) CLI is nil")
	}
	if enabled.Logger != enabledRuntime.Logger() {
		t.Fatal("Compose(enabled) replaced the Logger facade identity")
	}
	directory := t.TempDir()
	target := filepath.Join(directory, "cli.yaml")
	var stdout bytes.Buffer
	if err := enabled.CLI.RunWithIO(t.Context(), []string{"config", "init", "-o", target}, nil, &stdout, io.Discard); err != nil {
		t.Fatalf("config init error = %v", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("config init stdout is empty")
	}
	directTarget := filepath.Join(directory, "direct.yaml")
	if _, err := enabled.Configuration.Generate(t.Context(), config.GenerateRequest{Path: directTarget}); err != nil {
		t.Fatalf("Configuration.Generate() error = %v", err)
	}
	cliPayload, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile(CLI) error = %v", err)
	}
	directPayload, err := os.ReadFile(directTarget)
	if err != nil {
		t.Fatalf("ReadFile(direct) error = %v", err)
	}
	if !bytes.Equal(cliPayload, directPayload) {
		t.Fatalf("CLI and direct generation differ:\nCLI:\n%s\ndirect:\n%s", cliPayload, directPayload)
	}
	loggerIndex := bytes.Index(cliPayload, []byte("logger:"))
	databaseIndex := bytes.Index(cliPayload, []byte("database:"))
	if loggerIndex < 0 || databaseIndex < 0 || loggerIndex >= databaseIndex {
		t.Fatalf("generated capability order is not Logger then Database:\n%s", cliPayload)
	}
}

func TestComposeBuiltinLoggerOmitsConfiguredDefaults(t *testing.T) {
	runtime := newTestRuntime(t, config.MapSource("empty", map[string]any{}))
	capabilities, err := Compose(runtime, Options{})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	if capabilities.Logger == nil {
		t.Fatal("Compose() Logger is nil")
	}
	directory := t.TempDir()
	target := filepath.Join(directory, "builtin.yaml")
	if _, err := capabilities.Configuration.Generate(t.Context(), config.GenerateRequest{Path: target}); err != nil {
		t.Fatalf("Configuration.Generate() error = %v", err)
	}
	payload, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if bytes.Contains(payload, []byte("logger:")) {
		t.Fatalf("builtin-only configuration unexpectedly contains logger defaults:\n%s", payload)
	}
	if !bytes.Contains(payload, []byte("database:")) {
		t.Fatalf("builtin-only configuration misses database defaults:\n%s", payload)
	}
	capabilities.Logger.Info("builtin logger active")
}

func TestComposeRejectsUnknownLoggerSelectionWithoutInstallingPlan(t *testing.T) {
	runtime := newTestRuntime(t, config.MapSource("empty", map[string]any{}))
	capabilities, err := Compose(runtime, Options{Logger: LoggerSelection(255)})
	if err == nil {
		t.Fatal("Compose(unknown logger) error = nil")
	}
	if capabilities != (Capabilities{}) {
		t.Fatalf("Compose(unknown logger) = %#v, want zero capabilities", capabilities)
	}
	if _, err := Compose(runtime, Options{}); err != nil {
		t.Fatalf("Compose(valid after unknown logger) error = %v", err)
	}
}

func TestComposeCLIErrorReturnsZeroCapabilities(t *testing.T) {
	runtime := newTestRuntime(t, config.MapSource("empty", map[string]any{}))
	capabilities, err := Compose(runtime, Options{CLI: &CLIOptions{}})
	if err == nil {
		t.Fatal("Compose(invalid CLI) error = nil")
	}
	if capabilities.Logger != nil || capabilities.Clock != nil || capabilities.IDGenerator != nil || capabilities.Validator != nil || capabilities.Database != nil || capabilities.Configuration != nil || capabilities.CLI != nil {
		t.Fatalf("Compose(invalid CLI) = %#v, want zero capabilities", capabilities)
	}
	if _, err := Compose(runtime, Options{}); err != nil {
		t.Fatalf("Compose(valid after CLI failure) error = %v", err)
	}
}

func TestComposeRejectsDuplicateCapabilitySet(t *testing.T) {
	runtime := newTestRuntime(t, config.MapSource("empty", map[string]any{}))
	if _, err := Compose(runtime, Options{}); err != nil {
		t.Fatalf("first Compose() error = %v", err)
	}
	capabilities, err := Compose(runtime, Options{})
	if err == nil {
		t.Fatal("second Compose() error = nil")
	}
	if capabilities.Logger != nil || capabilities.Clock != nil || capabilities.IDGenerator != nil || capabilities.Validator != nil || capabilities.Database != nil {
		t.Fatal("second Compose() returned partial capabilities")
	}
	if capabilities.Configuration != nil || capabilities.CLI != nil {
		t.Fatal("second Compose() returned partial configuration or CLI capabilities")
	}
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}
