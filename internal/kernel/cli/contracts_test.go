package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
)

func TestNewAppRejectsInvalidContractsWithoutReturningPartialApp(t *testing.T) {
	contractErr := errors.New("contract failed")
	tests := []struct {
		name      string
		contracts []Contract
		want      error
	}{
		{name: "nil", contracts: []Contract{nil}},
		{name: "empty", contracts: []Contract{ContractFunc(func() ([]pkgcli.CommandSpec, error) { return nil, nil })}},
		{name: "error", contracts: []Contract{ContractFunc(func() ([]pkgcli.CommandSpec, error) { return nil, contractErr })}, want: contractErr},
		{name: "empty command", contracts: []Contract{commands(pkgcli.CommandSpec{})}},
		{name: "duplicate", contracts: []Contract{commands(pkgcli.CommandSpec{Name: "same"}), commands(pkgcli.CommandSpec{Name: "same"})}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, err := NewApp(pkgcli.Config{Name: "test", DisableInteractiveHome: true}, test.contracts...)
			if err == nil || app != nil {
				t.Fatalf("NewApp() = %#v, %v; want nil, error", app, err)
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("NewApp() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestConfigInitUsesDefaultsAndFlags(t *testing.T) {
	manager := &recordingManager{result: config.GenerateResult{Path: filepath.Join("absolute", "config.json"), Format: config.FormatJSON, Replaced: true, SectionIDs: []string{"database"}}}
	app, err := NewApp(pkgcli.Config{Name: "test", DisableInteractiveHome: true}, ConfigCommands(manager))
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	var stdout bytes.Buffer
	if err := app.RunWithIO(t.Context(), []string{"config", "init", "-o", "custom.json", "-f"}, strings.NewReader(""), &stdout, io.Discard); err != nil {
		t.Fatalf("RunWithIO() error = %v", err)
	}
	wantRequest := config.GenerateRequest{Path: "custom.json", Force: true}
	if !reflect.DeepEqual(manager.request, wantRequest) {
		t.Fatalf("Generate request = %#v, want %#v", manager.request, wantRequest)
	}
	wantOutput := "created default configuration: " + manager.result.Path + " (format=json replaced=true sections=database)\n"
	if stdout.String() != wantOutput {
		t.Fatalf("stdout = %q, want %q", stdout.String(), wantOutput)
	}

	manager.request = config.GenerateRequest{}
	if err := app.RunWithIO(t.Context(), []string{"config", "init"}, strings.NewReader(""), io.Discard, io.Discard); err != nil {
		t.Fatalf("RunWithIO(defaults) error = %v", err)
	}
	if manager.request.Path != "config.yaml" || manager.request.Force {
		t.Fatalf("default Generate request = %#v", manager.request)
	}
}

func TestConfigInitMapsUsageCommandAndOutputErrors(t *testing.T) {
	cause := errors.New("generation failed")
	manager := &recordingManager{err: cause}
	app, err := NewApp(pkgcli.Config{Name: "test", DisableInteractiveHome: true}, ConfigCommands(manager))
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if err := app.RunWithIO(t.Context(), []string{"config", "init", "unexpected"}, strings.NewReader(""), io.Discard, io.Discard); pkgcli.GetExitCode(err) != pkgcli.ExitUsage {
		t.Fatalf("positional args error = %v, exit = %d; want usage", err, pkgcli.GetExitCode(err))
	}
	err = app.RunWithIO(t.Context(), []string{"config", "init"}, strings.NewReader(""), io.Discard, io.Discard)
	var configErr *pkgcli.ConfigError
	if !errors.As(err, &configErr) || !errors.Is(err, cause) || pkgcli.GetExitCode(err) != pkgcli.ExitConfig {
		t.Fatalf("generation error = %v, want ConfigError preserving cause", err)
	}

	manager.err = nil
	manager.result.Path = "config.yaml"
	outputErr := errors.New("stdout failed")
	err = app.RunWithIO(t.Context(), []string{"config", "init"}, strings.NewReader(""), failingWriter{err: outputErr}, io.Discard)
	var commandErr *pkgcli.CommandError
	if !errors.As(err, &commandErr) || !errors.Is(err, outputErr) {
		t.Fatalf("stdout error = %v, want CommandError preserving output error", err)
	}
}

func TestConfigCommandsRejectsNilManager(t *testing.T) {
	app, err := NewApp(pkgcli.Config{Name: "test"}, ConfigCommands(nil))
	if err == nil || app != nil {
		t.Fatalf("NewApp(nil manager) = %#v, %v; want nil, error", app, err)
	}
}

type recordingManager struct {
	request config.GenerateRequest
	result  config.GenerateResult
	err     error
}

func (m *recordingManager) Generate(_ context.Context, request config.GenerateRequest) (config.GenerateResult, error) {
	m.request = request
	return m.result, m.err
}

func commands(specs ...pkgcli.CommandSpec) Contract {
	return ContractFunc(func() ([]pkgcli.CommandSpec, error) { return specs, nil })
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("write: %w", w.err) }
