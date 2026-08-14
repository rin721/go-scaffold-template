package main

import (
	"bytes"
	"context"
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

	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
	"github.com/rin721/go-scaffold2/pkg/cli"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

func TestProcessRunsConfigInitBeforeConfigExists(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "generated", "config.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	process := newTestProcess(t, strings.NewReader(""), &stdout, &stderr)
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
	for _, expected := range []string{
		"logger:", "environment: development", "level: info",
		"database:", "driver: sqlite", "dsn: .data/app.db", "pingTimeout: 5s",
		"cache:", "driver: disabled", "i18n:", "defaultLanguage: zh-CN",
		"storage:", "basePath: .data/storage",
		"http:", "addr: :8080",
	} {
		if !bytes.Contains(content, []byte(expected)) {
			t.Fatalf("generated config missing %q:\n%s", expected, content)
		}
	}
}

func TestProcessServiceModePreservesMissingConfigError(t *testing.T) {
	t.Parallel()

	missingPath := filepath.Join(t.TempDir(), "missing.yaml")
	process := newTestProcess(t, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	process.configPath = missingPath

	err := process.run(t.Context(), nil)
	if err == nil {
		t.Fatal("service mode error = nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("service mode error = %v, want os.ErrNotExist", err)
	}
	if !strings.Contains(err.Error(), "prepare application configuration") {
		t.Fatalf("service mode error = %v, want preparation context", err)
	}
}

func TestProcessServiceModeStartsDefaultCapabilitiesWithoutExternalServices(t *testing.T) {
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "app.db")
	storagePath := filepath.Join(directory, "storage")
	configPath := filepath.Join(directory, "config.yaml")
	httpAddress := reserveLoopbackAddress(t)
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
http:
  addr: %q
`, filepath.ToSlash(databasePath), filepath.ToSlash(storagePath), httpAddress)
	if err := os.WriteFile(configPath, []byte(payload), 0o600); err != nil {
		t.Fatalf("write service config: %v", err)
	}

	process := newTestProcess(t, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	process.configPath = configPath
	process.environmentPrefix = "GO_SCAFFOLD2_TEST_011_"
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- process.run(ctx, nil) }()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	httpClient := &http.Client{Timeout: 250 * time.Millisecond}
	for {
		select {
		case err := <-done:
			t.Fatalf("service exited before readiness: %v", err)
		case <-ticker.C:
			if fileExists(databasePath) && directoryExists(storagePath) {
				response, requestErr := httpClient.Get("http://" + httpAddress + "/not-registered")
				if requestErr != nil {
					continue
				}
				response.Body.Close()
				if response.StatusCode != http.StatusNotFound {
					cancel()
					<-done
					t.Fatalf("default service HTTP status = %d, want %d", response.StatusCode, http.StatusNotFound)
				}
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Fatalf("service shutdown error = %v", err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("service did not stop after cancellation")
				}
				return
			}
		case <-timeout.C:
			cancel()
			select {
			case <-done:
				t.Fatal("service did not create default SQLite and local Storage resources")
			case <-time.After(5 * time.Second):
				t.Fatal("service neither became ready nor stopped after cancellation")
			}
		}
	}
}

func reserveLoopbackAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve loopback address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release loopback address: %v", err)
	}
	return address
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func TestProcessRejectsNilContext(t *testing.T) {
	t.Parallel()

	process := newTestProcess(t, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err := process.run(nil, []string{"config", "init"}); err == nil {
		t.Fatal("run nil context error = nil")
	}
}

func TestExecuteUsesCLIExitCodeAndReportsError(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	process := newTestProcess(t, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	exitCode := execute(context.Background(), process, []string{"unknown"})
	if exitCode != cli.ExitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, cli.ExitUsage)
	}
	if !strings.Contains(stderr.String(), "go-scaffold2: run application CLI") {
		t.Fatalf("stderr = %q, want application context", stderr.String())
	}
}

func TestExecuteCoversSuccessConfigAndCancellationExitCodes(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		args []string
		want int
	}{
		{name: "success", ctx: t.Context(), args: []string{"--help"}, want: cli.ExitSuccess},
		{name: "config", ctx: t.Context(), args: []string{"config", "init", "--output", filepath.Join(t.TempDir(), "config.txt")}, want: cli.ExitConfig},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := newTestProcess(t, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
			if code := execute(test.ctx, current, test.args); code != test.want {
				t.Fatalf("execute() = %d, want %d", code, test.want)
			}
		})
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	current := newTestProcess(t, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if code := execute(cancelled, current, []string{"config", "init", "--output", filepath.Join(t.TempDir(), "cancelled.yaml")}); code != cli.ExitInterrupted {
		t.Fatalf("execute(cancelled) = %d, want %d", code, cli.ExitInterrupted)
	}
}

func TestExecuteReturnsErrorWhenReportingFails(t *testing.T) {
	t.Parallel()

	process := newTestProcess(t, strings.NewReader(""), &bytes.Buffer{}, failingWriter{})
	if exitCode := execute(context.Background(), process, []string{"unknown"}); exitCode != cli.ExitError {
		t.Fatalf("exit code = %d, want %d", exitCode, cli.ExitError)
	}
}

func TestApplicationLifecycleUsesInjectedLogger(t *testing.T) {
	log := pkglogger.NewTestLogger()
	lifecycle := applicationLifecycle{logging: log}
	if err := lifecycle.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := lifecycle.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	entries := log.Entries()
	if len(entries) != 2 || entries[0].Message != "application started" || entries[1].Message != "application stopping" {
		t.Fatalf("lifecycle entries = %#v", entries)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func newTestProcess(t *testing.T, stdin io.Reader, stdout, stderr io.Writer) process {
	t.Helper()
	manager, err := kernellogging.New(pkglogger.Noop())
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	return newProcess(stdin, stdout, stderr, manager)
}
