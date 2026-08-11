package composition

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
)

func TestKernelNewDoesNotComposeCapabilities(t *testing.T) {
	runtime, err := kernel.New(config.New(config.MapSource("empty", map[string]any{})), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() without Compose error = %v", err)
	}
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestComposeMakesCLIExplicitlyOptional(t *testing.T) {
	disabledRuntime, err := kernel.New(config.New(config.MapSource("empty", map[string]any{})), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	disabled, err := Compose(disabledRuntime, Options{})
	if err != nil {
		t.Fatalf("Compose(disabled) error = %v", err)
	}
	if disabled.Database == nil || disabled.Configuration == nil || disabled.CLI != nil {
		t.Fatalf("Compose(disabled) = %#v", disabled)
	}

	enabledRuntime, err := kernel.New(config.New(config.MapSource("empty", map[string]any{})), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	enabled, err := Compose(enabledRuntime, Options{CLI: &CLIOptions{App: pkgcli.Config{
		Name:                   "test",
		DisableInteractiveHome: true,
	}}})
	if err != nil {
		t.Fatalf("Compose(enabled) error = %v", err)
	}
	if enabled.CLI == nil {
		t.Fatal("Compose(enabled) CLI is nil")
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
}

func TestComposeCLIErrorReturnsZeroCapabilities(t *testing.T) {
	runtime, err := kernel.New(config.New(config.MapSource("empty", map[string]any{})), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	capabilities, err := Compose(runtime, Options{CLI: &CLIOptions{}})
	if err == nil {
		t.Fatal("Compose(invalid CLI) error = nil")
	}
	if capabilities.Database != nil || capabilities.Configuration != nil || capabilities.CLI != nil {
		t.Fatalf("Compose(invalid CLI) = %#v, want zero capabilities", capabilities)
	}
}

func TestComposeRejectsDuplicateCapabilitySet(t *testing.T) {
	runtime, err := kernel.New(config.New(config.MapSource("empty", map[string]any{})), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	if _, err := Compose(runtime, Options{}); err != nil {
		t.Fatalf("first Compose() error = %v", err)
	}
	capabilities, err := Compose(runtime, Options{})
	if err == nil {
		t.Fatal("second Compose() error = nil")
	}
	if capabilities.Database != nil {
		t.Fatal("second Compose() returned partial capabilities")
	}
	if capabilities.Configuration != nil || capabilities.CLI != nil {
		t.Fatal("second Compose() returned partial configuration or CLI capabilities")
	}
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}
