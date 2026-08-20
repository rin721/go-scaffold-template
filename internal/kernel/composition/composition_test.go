package composition

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rin721/go-scaffold-template/internal/kernel"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold-template/pkg/cli"
)

func TestKernelNewDoesNotComposeCapabilities(t *testing.T) {
	runtime := newTestRuntime(t, config.MapSource("empty", map[string]any{}))
	coordinator, err := kernel.NewCoordinator(runtime)
	if err != nil {
		t.Fatalf("NewCoordinator() error = %v", err)
	}
	if err := coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() without Compose error = %v", err)
	}
	if err := coordinator.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestComposeBuildsServiceCapabilitiesWithoutCLI(t *testing.T) {
	runtime := newTestRuntime(t, config.MapSource("empty", map[string]any{}))
	capabilities, err := Compose(runtime, Options{})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	if capabilities.Logger == nil || capabilities.Clock == nil || capabilities.IDGenerator == nil || capabilities.Validator == nil || capabilities.Database == nil || capabilities.Cache == nil || capabilities.I18n == nil || capabilities.Storage == nil {
		t.Fatalf("Compose() = %#v", capabilities)
	}
	if capabilities.Logger != runtime.Logger() {
		t.Fatal("Compose() did not return the Kernel builtin Logger facade")
	}
	if _, err := capabilities.IDGenerator.New(); err != nil {
		t.Fatalf("IDGenerator.New() error = %v", err)
	}
	if err := capabilities.Validator.Struct(struct {
		Name string `validate:"required"`
	}{Name: "ready"}); err != nil {
		t.Fatalf("Validator.Struct() error = %v", err)
	}
	if capabilities.Clock.Now().IsZero() {
		t.Fatal("Clock.Now() returned zero time")
	}
}

func TestComposeBootstrapGeneratesAllServiceSectionsWithoutKernel(t *testing.T) {
	bootstrap, err := ComposeBootstrap(pkgcli.Config{Name: "test", DisableInteractiveHome: true}, BootstrapOptions{})
	if err != nil {
		t.Fatalf("ComposeBootstrap() error = %v", err)
	}
	directory := t.TempDir()
	cliTarget := filepath.Join(directory, "cli.yaml")
	var stdout bytes.Buffer
	if err := bootstrap.CLI.RunWithIO(t.Context(), []string{"config", "init", "-o", cliTarget}, strings.NewReader(""), &stdout, io.Discard); err != nil {
		t.Fatalf("config init error = %v", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("config init stdout is empty")
	}
	directTarget := filepath.Join(directory, "direct.yaml")
	if _, err := bootstrap.Configuration.Generate(t.Context(), config.GenerateRequest{Path: directTarget}); err != nil {
		t.Fatalf("Configuration.Generate() error = %v", err)
	}
	cliPayload, err := os.ReadFile(cliTarget)
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
	previous := -1
	for _, section := range []string{"logger:", "database:", "cache:", "i18n:", "storage:", "execution:", "scheduler:", "http:"} {
		index := bytes.Index(cliPayload, []byte(section))
		if index <= previous {
			t.Fatalf("generated section %s is missing or out of order:\n%s", section, cliPayload)
		}
		previous = index
	}
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

func TestComposeBootstrapFailureDoesNotTouchServiceRuntime(t *testing.T) {
	if bootstrap, err := ComposeBootstrap(pkgcli.Config{}, BootstrapOptions{}); err == nil || bootstrap.CLI != nil {
		t.Fatalf("ComposeBootstrap(invalid app) = %#v, %v", bootstrap, err)
	}
	runtime := newTestRuntime(t, config.MapSource("empty", map[string]any{}))
	if _, err := Compose(runtime, Options{}); err != nil {
		t.Fatalf("Compose() after bootstrap failure error = %v", err)
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
	if capabilities != (Capabilities{}) {
		t.Fatalf("second Compose() = %#v, want zero capabilities", capabilities)
	}
	coordinator, err := kernel.NewCoordinator(runtime)
	if err != nil {
		t.Fatalf("NewCoordinator() error = %v", err)
	}
	if err := coordinator.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}
