package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/app"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

func TestServiceHostOptionsEnablesWatch(t *testing.T) {
	logging := pkglogger.NewTestLogger()
	options := serviceHostOptions(logging)
	if options.Watch == nil || options.Watch.OnReloadError == nil {
		t.Fatal("service HostOptions does not enable config watch")
	}

	options.Watch.OnReloadError(errors.New("candidate failed"))
	entries := logging.Entries()
	if len(entries) != 1 || entries[0].Level != "error" || entries[0].Message != "kernel reload rejected; previous configuration remains active" {
		t.Fatalf("reload entries = %#v", entries)
	}
}

func TestReloadErrorReporterClassifiesLifecycleOutcome(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		level   string
		message string
	}{
		{
			name: "rejected",
			err:  errors.New("candidate failed"), level: "error",
			message: "kernel reload rejected; previous configuration remains active",
		},
		{
			name: "restart required",
			err:  &app.RestartRequiredError{Components: []app.ID{"server"}}, level: "warn",
			message: "kernel reload requires process restart; previous configuration remains active",
		},
		{
			name: "committed cleanup",
			err:  &kernel.CommittedCleanupError{Err: errors.New("close previous failed")}, level: "error",
			message: "kernel reload applied but previous resources failed to close",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logging := pkglogger.NewTestLogger()
			reloadErrorReporter(logging)(tt.err)
			entries := logging.Entries()
			if len(entries) != 1 || entries[0].Level != tt.level || entries[0].Message != tt.message {
				t.Fatalf("entries = %#v", entries)
			}
		})
	}
}

func TestReloadErrorReporterIgnoresNilInputs(t *testing.T) {
	reloadErrorReporter(nil)(errors.New("ignored"))
	logging := pkglogger.NewTestLogger()
	reloadErrorReporter(logging)(nil)
	if entries := logging.Entries(); len(entries) != 0 {
		t.Fatalf("entries = %#v, want empty", entries)
	}
}

func TestReloadErrorReporterDoesNotWriteSensitiveErrorDetails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reload.log")
	addCaller := false
	resource, err := pkglogger.New(&pkglogger.Config{
		Environment:      pkglogger.EnvironmentProduction,
		OutputPaths:      []string{path},
		ErrorOutputPaths: []string{path},
		AddCaller:        &addCaller,
	})
	if err != nil {
		t.Fatalf("logger.New() error = %v", err)
	}
	secret := "postgres://user:top-secret@example.invalid/app"
	reloadErrorReporter(resource)(errors.New("connect " + secret))
	if err := resource.Close(); err != nil {
		t.Fatalf("logger.Close() error = %v", err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if bytes.Contains(payload, []byte(secret)) || bytes.Contains(payload, []byte("top-secret")) {
		t.Fatalf("reload log leaked sensitive error details: %s", payload)
	}
	if !bytes.Contains(payload, []byte("error_type")) {
		t.Fatalf("reload log misses safe diagnostic type: %s", payload)
	}
}
