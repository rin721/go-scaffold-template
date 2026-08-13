package composition

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

func TestComposeMakesCLIExplicitlyOptional(t *testing.T) {
	disabledAssembly := newTestAssembly(t, config.MapSource("empty", map[string]any{}), nil)
	disabled, err := Compose(disabledAssembly)
	if err != nil {
		t.Fatalf("Compose(disabled) error = %v", err)
	}
	if disabled.Logger == nil || disabled.Clock == nil || disabled.IDGenerator == nil || disabled.Validator == nil || disabled.Database == nil || disabled.Configuration == nil || disabled.CLI != nil {
		t.Fatalf("Compose(disabled) = %#v", disabled)
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
	if err := disabled.Runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop(disabled) error = %v", err)
	}

	cliConfig := pkgcli.Config{
		Name:                   "test",
		DisableInteractiveHome: true,
	}
	enabledAssembly := newTestAssembly(t, config.MapSource("empty", map[string]any{}), &cliConfig)
	enabled, err := Compose(enabledAssembly)
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
	if err := enabled.Runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop(enabled) error = %v", err)
	}
	if err := enabled.Logger.Use(t.Context(), func(pkglogger.Logger) error { return nil }); !errors.Is(err, app.ErrStopped) {
		t.Fatalf("Logger.Use() after CLI stop = %v, want ErrStopped", err)
	}
	loggerIndex := bytes.Index(cliPayload, []byte("logger:"))
	databaseIndex := bytes.Index(cliPayload, []byte("database:"))
	if loggerIndex < 0 || databaseIndex < 0 || loggerIndex >= databaseIndex {
		t.Fatalf("generated capability order is not Logger then Database:\n%s", cliPayload)
	}
}

func TestComposeCLIErrorReturnsZeroCapabilities(t *testing.T) {
	invalidCLI := pkgcli.Config{}
	assembly := newTestAssembly(t, config.MapSource("empty", map[string]any{}), &invalidCLI)
	capabilities, err := Compose(assembly)
	if err == nil {
		t.Fatal("Compose(invalid CLI) error = nil")
	}
	if capabilities.Logger != nil || capabilities.Clock != nil || capabilities.IDGenerator != nil || capabilities.Validator != nil || capabilities.Database != nil || capabilities.Configuration != nil || capabilities.CLI != nil {
		t.Fatalf("Compose(invalid CLI) = %#v, want zero capabilities", capabilities)
	}
}

func TestComposeRejectsDuplicateCapabilitySet(t *testing.T) {
	assembly := newTestAssembly(t, config.MapSource("empty", map[string]any{}), nil)
	first, err := Compose(assembly)
	if err != nil {
		t.Fatalf("first Compose() error = %v", err)
	}
	capabilities, err := Compose(assembly)
	if err == nil {
		t.Fatal("second Compose() error = nil")
	}
	if capabilities.Logger != nil || capabilities.Clock != nil || capabilities.IDGenerator != nil || capabilities.Validator != nil || capabilities.Database != nil {
		t.Fatal("second Compose() returned partial capabilities")
	}
	if capabilities.Configuration != nil || capabilities.CLI != nil {
		t.Fatal("second Compose() returned partial configuration or CLI capabilities")
	}
	if err := first.Runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}
