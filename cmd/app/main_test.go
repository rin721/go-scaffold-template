package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rin721/go-scaffold2/pkg/cli"
)

func TestProcessRunsConfigInitBeforeConfigExists(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "generated", "config.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	process := newProcess(strings.NewReader(""), &stdout, &stderr)
	process.configPath = filepath.Join(t.TempDir(), "missing.yaml")

	if err := process.run(t.Context(), []string{"config", "init", "--output", outputPath}); err != nil {
		t.Fatalf("run config init: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), outputPath) {
		t.Fatalf("stdout = %q, want generated path", stdout.String())
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	for _, expected := range []string{"database:", "engine: \"\"", "driver: \"\"", "dsn: \"\"", "pingTimeout: 5s"} {
		if !bytes.Contains(content, []byte(expected)) {
			t.Fatalf("generated config missing %q:\n%s", expected, content)
		}
	}
}

func TestProcessServiceModePreservesMissingConfigError(t *testing.T) {
	t.Parallel()

	missingPath := filepath.Join(t.TempDir(), "missing.yaml")
	process := newProcess(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	process.configPath = missingPath

	err := process.run(t.Context(), nil)
	if err == nil {
		t.Fatal("service mode error = nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("service mode error = %v, want os.ErrNotExist", err)
	}
	if !strings.Contains(err.Error(), "run application host") {
		t.Fatalf("service mode error = %v, want host context", err)
	}
}

func TestProcessRejectsNilContext(t *testing.T) {
	t.Parallel()

	process := newProcess(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err := process.run(nil, []string{"config", "init"}); err == nil {
		t.Fatal("run nil context error = nil")
	}
}

func TestExecuteUsesCLIExitCodeAndReportsError(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	process := newProcess(strings.NewReader(""), &bytes.Buffer{}, &stderr)
	exitCode := execute(context.Background(), process, []string{"unknown"})
	if exitCode != cli.ExitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, cli.ExitUsage)
	}
	if !strings.Contains(stderr.String(), "go-scaffold2: run application CLI") {
		t.Fatalf("stderr = %q, want application context", stderr.String())
	}
}

func TestExecuteReturnsErrorWhenReportingFails(t *testing.T) {
	t.Parallel()

	process := newProcess(strings.NewReader(""), &bytes.Buffer{}, failingWriter{})
	if exitCode := execute(context.Background(), process, []string{"unknown"}); exitCode != cli.ExitError {
		t.Fatalf("exit code = %d, want %d", exitCode, cli.ExitError)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
