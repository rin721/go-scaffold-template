package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewWithNilConfigUsesDefaults(t *testing.T) {
	log, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil) returned error: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

func TestEnvironmentDefaultEncoding(t *testing.T) {
	tests := []struct {
		name        string
		environment Environment
		wantJSON    bool
	}{
		{name: "development uses console", environment: EnvironmentDevelopment, wantJSON: false},
		{name: "production uses json", environment: EnvironmentProduction, wantJSON: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addCaller := false
			output := captureStdout(t, func() {
				log, err := New(&Config{
					Environment: tt.environment,
					AddCaller:   &addCaller,
				})
				if err != nil {
					t.Fatalf("New returned error: %v", err)
				}

				log.Info("service started", String("component", "test"))
				if err := log.Close(); err != nil {
					t.Fatalf("Close returned error: %v", err)
				}
			})

			line := firstLogLine(t, output)
			isJSON := json.Valid([]byte(line))
			if isJSON != tt.wantJSON {
				t.Fatalf("json format = %v, want %v, line: %s", isJSON, tt.wantJSON, line)
			}
		})
	}
}

func TestEnvironmentDefaultLevel(t *testing.T) {
	tests := []struct {
		name        string
		environment Environment
		wantLevels  []string
	}{
		{name: "development emits debug and above", environment: EnvironmentDevelopment, wantLevels: []string{"debug", "info", "warn", "error"}},
		{name: "production emits info and above", environment: EnvironmentProduction, wantLevels: []string{"info", "warn", "error"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addCaller := false
			output := captureStdout(t, func() {
				log, err := New(&Config{Environment: tt.environment, AddCaller: &addCaller})
				if err != nil {
					t.Fatalf("New() error = %v", err)
				}
				log.Debug("debug")
				log.Info("info")
				log.Warn("warn")
				log.Error("error")
				if err := log.Close(); err != nil {
					t.Fatalf("Close() error = %v", err)
				}
			})
			lines := logLines(output)
			if len(lines) != len(tt.wantLevels) {
				t.Fatalf("log lines = %v, want levels %v", lines, tt.wantLevels)
			}
			for index, level := range tt.wantLevels {
				if !strings.Contains(lines[index], level) {
					t.Fatalf("line %d = %q, want level %q", index, lines[index], level)
				}
			}
		})
	}
}

func TestDefaultConfigEnvironmentSwitchUsesProductionEncoding(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Environment = EnvironmentProduction

	output := captureStdout(t, func() {
		log, err := New(&cfg)
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}

		log.Info("service started")
		if err := log.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})

	line := firstLogLine(t, output)
	if !json.Valid([]byte(line)) {
		t.Fatalf("production DefaultConfig output is not json: %s", line)
	}
}

func TestNewRejectsInvalidConfigValues(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{name: "invalid environment", cfg: &Config{Environment: Environment("test")}, want: "environment"},
		{name: "invalid level", cfg: &Config{Level: Level("trace")}, want: "level"},
		{name: "invalid encoding", cfg: &Config{Encoding: Encoding("xml")}, want: "encoding"},
		{name: "empty output path", cfg: &Config{OutputPaths: []string{" "}}, want: "output path"},
		{name: "unsupported sink scheme", cfg: &Config{OutputPaths: []string{"https://logs.example.invalid"}}, want: "sink scheme"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if err == nil {
				t.Fatal("New returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestWithAddsFieldsWithoutChangingParentLogger(t *testing.T) {
	addCaller := false
	output := captureStdout(t, func() {
		log, err := New(&Config{
			Environment: EnvironmentProduction,
			AddCaller:   &addCaller,
		})
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}

		log.Info("parent")
		log.With(String("component", "api")).Info("child")
		if err := log.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})

	lines := logLines(output)
	if len(lines) != 2 {
		t.Fatalf("got %d log lines, want 2: %v", len(lines), lines)
	}

	parent := decodeLogLine(t, lines[0])
	if _, ok := parent["component"]; ok {
		t.Fatalf("parent log unexpectedly contains component field: %v", parent)
	}

	child := decodeLogLine(t, lines[1])
	if child["component"] != "api" {
		t.Fatalf("child component = %v, want api", child["component"])
	}
}

func TestSyncCanBeCalled(t *testing.T) {
	log, err := New(nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := log.Sync(); err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

func TestValidateConfigDoesNotOpenOutputFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "application.log")
	cfg := DefaultConfig()
	cfg.OutputPaths = []string{path}
	if err := ValidateConfig(&cfg); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat() error = %v, want os.ErrNotExist", err)
	}
}

func TestResourceCloseFlushesAndClosesOwnedFileOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.log")
	cfg := DefaultConfig()
	cfg.OutputPaths = []string{path}
	cfg.ErrorOutputPaths = []string{path}
	resource, err := New(&cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	resource.Info("persisted message")
	if err := resource.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := resource.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Contains(payload, []byte("persisted message")) {
		t.Fatalf("log payload = %q", payload)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove() after Close error = %v", err)
	}
}

func TestResourceCloseJoinsSyncAndAllCloserErrors(t *testing.T) {
	syncErr := errors.New("sync failed")
	secondSyncErr := errors.New("second sync failed")
	firstCloseErr := errors.New("first close failed")
	secondCloseErr := errors.New("second close failed")
	writer := &failingWriteSyncer{syncErr: syncErr}
	secondWriter := &failingWriteSyncer{syncErr: secondSyncErr}
	underlying := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		writer,
		zapcore.DebugLevel,
	))
	first := &countingCloser{err: firstCloseErr}
	second := &countingCloser{err: secondCloseErr}
	state := &resourceState{
		syncers: []zapcore.WriteSyncer{writer, secondWriter},
		closers: []io.Closer{first, second},
	}
	resource := &zapLogger{logger: underlying, state: state}

	err := resource.Close()
	for _, want := range []error{syncErr, secondSyncErr, firstCloseErr, secondCloseErr} {
		if !errors.Is(err, want) {
			t.Fatalf("Close() error = %v, want %v", err, want)
		}
	}
	if second.count != 1 || first.count != 1 || writer.syncCount() != 1 || secondWriter.syncCount() != 1 {
		t.Fatalf("counts = sync:%d secondSync:%d first:%d second:%d, want all 1", writer.syncCount(), secondWriter.syncCount(), first.count, second.count)
	}
	if secondErr := resource.Close(); !errors.Is(secondErr, syncErr) {
		t.Fatalf("second Close() error = %v, want original joined error", secondErr)
	}
	if second.count != 1 || first.count != 1 || writer.syncCount() != 1 || secondWriter.syncCount() != 1 {
		t.Fatalf("second Close counts = sync:%d secondSync:%d first:%d second:%d", writer.syncCount(), secondWriter.syncCount(), first.count, second.count)
	}
}

func TestStandardStreamsHaveNoOwnedCloser(t *testing.T) {
	for _, path := range []string{outputPathStdout, outputPathStderr} {
		writer, closer, err := openSink(path)
		if err != nil {
			t.Fatalf("openSink(%s) error = %v", path, err)
		}
		if closer != nil {
			t.Fatalf("openSink(%s) closer = %#v, want nil", path, closer)
		}
		if err := writer.Sync(); err != nil {
			t.Fatalf("openSink(%s).Sync() error = %v", path, err)
		}
	}
}

type failingWriteSyncer struct {
	mu      sync.Mutex
	syncErr error
	syncs   int
}

func (*failingWriteSyncer) Write(payload []byte) (int, error) { return len(payload), nil }

func (w *failingWriteSyncer) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.syncs++
	return w.syncErr
}

func (w *failingWriteSyncer) syncCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.syncs
}

type countingCloser struct {
	err   error
	count int
}

func (c *countingCloser) Close() error {
	c.count++
	return c.err
}

func decodeLogLine(t *testing.T, line string) map[string]any {
	t.Helper()
	var values map[string]any
	if err := json.Unmarshal([]byte(line), &values); err != nil {
		t.Fatalf("decode log line: %v; line: %s", err, line)
	}
	return values
}

func captureStdout(t *testing.T, writeLog func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}

	restored := false
	defer func() {
		if !restored {
			os.Stdout = originalStdout
			_ = writer.Close()
			_ = reader.Close()
		}
	}()

	os.Stdout = writer
	writeLog()
	os.Stdout = originalStdout
	restored = true
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}

	return string(output)
}

func firstLogLine(t *testing.T, output string) string {
	t.Helper()
	lines := logLines(output)
	if len(lines) == 0 {
		t.Fatal("log output is empty")
	}
	return lines[0]
}

func logLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
