package logger

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNewWithNilConfigUsesDefaults(t *testing.T) {
	log, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil) returned error: %v", err)
	}
	if err := log.Sync(); err != nil {
		t.Fatalf("Sync returned error: %v", err)
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
			})

			line := firstLogLine(t, output)
			isJSON := json.Valid([]byte(line))
			if isJSON != tt.wantJSON {
				t.Fatalf("json format = %v, want %v, line: %s", isJSON, tt.wantJSON, line)
			}
		})
	}
}

func TestDefaultConfigCanUseProductionDefaults(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Environment = EnvironmentProduction

	output := captureStdout(t, func() {
		log, err := New(&cfg)
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}

		log.Info("service started")
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
