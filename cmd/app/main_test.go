package main

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

	applicationcomposition "github.com/rin721/go-scaffold-template/internal/composition"
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
		"auth:", "mode: development-anonymous", "migration:", "lockTimeout: 15s",
		"todo:", "titleMaxRunes: 120", "defaultListLimit: 20", "maxListLimit: 100",
		"http:", "addr: 127.0.0.1:8080",
		"management:", "addr: 127.0.0.1:9090", "metricsAccess: public",
		"observability:", "serviceName: go-scaffold-template", "sampleRatio: 0.1",
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
todo:
  titleMaxRunes: 120
  defaultListLimit: 20
  maxListLimit: 100
http:
  addr: %q
`, filepath.ToSlash(databasePath), filepath.ToSlash(storagePath), httpAddress)
	if err := os.WriteFile(configPath, []byte(payload), 0o600); err != nil {
		t.Fatalf("write service config: %v", err)
	}
	runTestMigration(t, configPath, "GO_SCAFFOLD2_TEST_011_")

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
				missingTypeRequest, requestErr := http.NewRequest(
					http.MethodPost,
					"http://"+httpAddress+"/api/v1/todos",
					strings.NewReader(`{"title":"缺少 Content-Type"}`),
				)
				if requestErr != nil {
					t.Fatalf("NewRequest(missing Content-Type) error = %v", requestErr)
				}
				missingTypeResponse, requestErr := httpClient.Do(missingTypeRequest)
				if requestErr != nil {
					continue
				}
				var missingTypePayload struct {
					Type   string `json:"type"`
					Status int    `json:"status"`
					Code   string `json:"code"`
					Detail string `json:"detail"`
				}
				decodeErr := json.NewDecoder(missingTypeResponse.Body).Decode(&missingTypePayload)
				missingTypeResponse.Body.Close()
				if missingTypeResponse.StatusCode != http.StatusUnsupportedMediaType || decodeErr != nil ||
					missingTypePayload.Status != missingTypeResponse.StatusCode ||
					missingTypePayload.Code != "unsupported_media_type" ||
					missingTypePayload.Type != "urn:go-scaffold-template:problem:unsupported_media_type" {
					cancel()
					<-done
					t.Fatalf("missing Content-Type response = %d %#v, decodeErr=%v", missingTypeResponse.StatusCode, missingTypePayload, decodeErr)
				}
				if missingTypeResponse.Header.Get("X-Request-ID") == "" ||
					missingTypeResponse.Header.Get("X-Content-Type-Options") != "nosniff" {
					cancel()
					<-done
					t.Fatalf("global middleware headers = %#v", missingTypeResponse.Header)
				}

				request, requestErr := http.NewRequest(http.MethodPost, "http://"+httpAddress+"/api/v1/todos", strings.NewReader(`{"title":"学习 Go"}`))
				if requestErr != nil {
					t.Fatalf("NewRequest() error = %v", requestErr)
				}
				request.Header.Set("Content-Type", "application/json")
				response, requestErr := httpClient.Do(request)
				if requestErr != nil {
					continue
				}
				response.Body.Close()
				if response.StatusCode != http.StatusCreated {
					cancel()
					<-done
					t.Fatalf("Todo create status = %d, want %d", response.StatusCode, http.StatusCreated)
				}
				notFound, requestErr := httpClient.Get("http://" + httpAddress + "/not-registered")
				if requestErr != nil {
					cancel()
					<-done
					t.Fatalf("GET(not registered) error = %v", requestErr)
				}
				notFound.Body.Close()
				if notFound.StatusCode != http.StatusNotFound {
					cancel()
					<-done
					t.Fatalf("not registered status = %d, want %d", notFound.StatusCode, http.StatusNotFound)
				}
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Fatalf("service shutdown error = %v", err)
					}
					assertAddressCanBeRebound(t, httpAddress)
					assertFileCanBeRenamed(t, databasePath)
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

func TestProcessTodoCLIUsesSQLiteAcrossInvocations(t *testing.T) {
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "todo.db")
	storagePath := filepath.Join(directory, "storage")
	configPath := filepath.Join(directory, "config.yaml")
	address := reserveLoopbackAddress(t)
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
  addr: %q
`, filepath.ToSlash(databasePath), filepath.ToSlash(storagePath), address)
	if err := os.WriteFile(configPath, []byte(payload), 0o600); err != nil {
		t.Fatalf("write CLI config: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	process := newTestProcess(t, strings.NewReader(""), &stdout, &stderr)
	process.configPath = configPath
	process.environmentPrefix = "GO_SCAFFOLD2_TEST_014_"
	if err := process.run(t.Context(), []string{"db", "migrate", "up"}); err != nil {
		t.Fatalf("db migrate up error = %v, stderr=%s", err, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	if err := process.run(t.Context(), []string{"todo", "create", "--subject", "development-loopback", "--scopes", "todos:read,todos:write", "--title", "学习 Go"}); err != nil {
		t.Fatalf("todo create error = %v, stderr=%s", err, stderr.String())
	}
	var created struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil || created.ID == "" || created.Status != "pending" {
		t.Fatalf("create output = %q, parsed=%#v, err=%v", stdout.String(), created, err)
	}

	stdout.Reset()
	if err := process.run(t.Context(), []string{"todo", "list", "--subject", "development-loopback", "--scopes", "todos:read,todos:write"}); err != nil {
		t.Fatalf("todo list error = %v", err)
	}
	var listed struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &listed); err != nil || listed.Total != 1 {
		t.Fatalf("list output = %q, parsed=%#v, err=%v", stdout.String(), listed, err)
	}

	stdout.Reset()
	if err := process.run(t.Context(), []string{"todo", "complete", "--subject", "development-loopback", "--scopes", "todos:read,todos:write", "--id", created.ID}); err != nil {
		t.Fatalf("todo complete error = %v", err)
	}
	var completed struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &completed); err != nil || completed.Status != "completed" {
		t.Fatalf("complete output = %q, parsed=%#v, err=%v", stdout.String(), completed, err)
	}
	if !fileExists(databasePath) {
		t.Fatal("Todo CLI did not create SQLite database")
	}
	assertFileCanBeRenamed(t, databasePath)

	serviceContext, cancelService := context.WithCancel(t.Context())
	serviceDone := make(chan error, 1)
	go func() {
		serviceDone <- process.run(serviceContext, nil)
	}()

	baseURL := "http://" + address
	deadline := time.Now().Add(5 * time.Second)
	for {
		response, err := http.Get(baseURL + "/api/v1/todos/" + created.ID)
		if err == nil {
			var fetched struct {
				Status string `json:"status"`
			}
			decodeErr := json.NewDecoder(response.Body).Decode(&fetched)
			response.Body.Close()
			if response.StatusCode == http.StatusOK && decodeErr == nil {
				if fetched.Status != "completed" {
					cancelService()
					<-serviceDone
					t.Fatalf("HTTP fetched status = %q, want completed", fetched.Status)
				}
				break
			}
		}
		if time.Now().After(deadline) {
			cancelService()
			<-serviceDone
			t.Fatal("service did not expose Todo created by CLI")
		}
		time.Sleep(20 * time.Millisecond)
	}

	response, err := http.Post(
		baseURL+"/api/v1/todos",
		"application/json",
		strings.NewReader(`{"title":"通过 HTTP 创建"}`),
	)
	if err != nil {
		cancelService()
		<-serviceDone
		t.Fatalf("HTTP create error = %v", err)
	}
	var createdByHTTP struct {
		ID string `json:"id"`
	}
	decodeErr := json.NewDecoder(response.Body).Decode(&createdByHTTP)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated || decodeErr != nil || createdByHTTP.ID == "" {
		cancelService()
		<-serviceDone
		t.Fatalf("HTTP create status = %d, body=%#v, decodeErr=%v", response.StatusCode, createdByHTTP, decodeErr)
	}
	cancelService()
	select {
	case err := <-serviceDone:
		if err != nil {
			t.Fatalf("service shutdown error = %v", err)
		}
		assertAddressCanBeRebound(t, address)
		assertFileCanBeRenamed(t, databasePath)
	case <-time.After(5 * time.Second):
		t.Fatal("service did not stop after cancellation")
	}

	stdout.Reset()
	if err := process.run(t.Context(), []string{"todo", "complete", "--subject", "development-loopback", "--scopes", "todos:read,todos:write", "--id", createdByHTTP.ID}); err != nil {
		t.Fatalf("CLI complete HTTP Todo error = %v", err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &completed); err != nil || completed.Status != "completed" {
		t.Fatalf("cross-mode complete output = %q, parsed=%#v, err=%v", stdout.String(), completed, err)
	}
}

func runTestMigration(t *testing.T, configPath, environmentPrefix string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	process := newTestProcess(t, strings.NewReader(""), &stdout, &stderr)
	process.configPath = configPath
	process.environmentPrefix = environmentPrefix
	if err := process.run(t.Context(), []string{"db", "migrate", "up"}); err != nil {
		t.Fatalf("db migrate up error = %v, stderr=%s", err, stderr.String())
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

func assertAddressCanBeRebound(t *testing.T, address string) {
	t.Helper()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("rebind released address %s: %v", address, err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("close rebound address %s: %v", address, err)
	}
}

func assertFileCanBeRenamed(t *testing.T, path string) {
	t.Helper()
	releasedPath := path + ".released"
	if err := os.Rename(path, releasedPath); err != nil {
		t.Fatalf("rename released file %s: %v", path, err)
	}
	if err := os.Rename(releasedPath, path); err != nil {
		t.Fatalf("restore released file %s: %v", path, err)
	}
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
	if exitCode != applicationcomposition.ExitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, applicationcomposition.ExitUsage)
	}
	if !strings.Contains(stderr.String(), "go-scaffold-template: run application CLI") {
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
		{name: "success", ctx: t.Context(), args: []string{"--help"}, want: applicationcomposition.ExitSuccess},
		{name: "config", ctx: t.Context(), args: []string{"config", "init", "--output", filepath.Join(t.TempDir(), "config.txt")}, want: applicationcomposition.ExitConfig},
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
	if code := execute(cancelled, current, []string{"config", "init", "--output", filepath.Join(t.TempDir(), "cancelled.yaml")}); code != applicationcomposition.ExitInterrupted {
		t.Fatalf("execute(cancelled) = %d, want %d", code, applicationcomposition.ExitInterrupted)
	}
}

func TestExecuteReturnsErrorWhenReportingFails(t *testing.T) {
	t.Parallel()

	process := newTestProcess(t, strings.NewReader(""), &bytes.Buffer{}, failingWriter{})
	if exitCode := execute(context.Background(), process, []string{"unknown"}); exitCode != applicationcomposition.ExitError {
		t.Fatalf("exit code = %d, want %d", exitCode, applicationcomposition.ExitError)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func newTestProcess(t *testing.T, stdin io.Reader, stdout, stderr io.Writer) process {
	t.Helper()
	return newProcess(stdin, stdout, stderr)
}
