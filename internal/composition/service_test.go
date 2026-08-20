package composition

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rin721/go-scaffold-template/internal/kernel"
	kernellogging "github.com/rin721/go-scaffold-template/internal/kernel/logging"
	"github.com/rin721/go-scaffold-template/internal/module/migration"
	"github.com/rin721/go-scaffold-template/pkg/logger"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

func TestExampleConfigSatisfiesApplicationBindings(t *testing.T) {
	manager, err := kernellogging.New(logger.Noop())
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	application, err := New(Config{
		Name: "go-scaffold-template", Description: "test application",
		ConfigPath:        filepath.Join("..", "..", "config.example.yaml"),
		EnvironmentPrefix: "GO_SCAFFOLD2_TEST_014_EXAMPLE_", Logging: manager,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	prepared, err := application.prepareTodo(t.Context())
	if err != nil {
		t.Fatalf("prepareTodo(config.example.yaml) error = %v", err)
	}
	if prepared.module.Service == nil || prepared.coordinator == nil {
		t.Fatalf("prepared application = %#v", prepared)
	}
}

func TestApplicationLifecycleUsesInjectedLogger(t *testing.T) {
	log := logger.NewTestLogger()
	lifecycle := applicationLifecycle{applicationName: "test-app", logging: log}
	if err := lifecycle.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := lifecycle.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	entries := log.Entries()
	if len(entries) != 2 || entries[0].Message != "application ready" || entries[1].Message != "application draining" {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestApplicationServiceFailureUsesSingleStructuredBoundary(t *testing.T) {
	log := logger.NewTestLogger()
	manager, err := kernellogging.New(log)
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	application, err := New(Config{
		Name: "test-app", Description: "test application",
		ConfigPath: filepath.Join(t.TempDir(), "missing.yaml"), EnvironmentPrefix: "TEST_LOGGING_",
		Logging: manager,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := application.runService(t.Context()); err == nil {
		t.Fatal("runService() error = nil")
	}
	entries := log.Entries()
	var failures int
	for _, entry := range entries {
		if entry.Level == "error" && entry.Message == "application service failed" {
			failures++
		}
	}
	if failures != 1 {
		t.Fatalf("structured service failures = %d, entries = %#v", failures, entries)
	}
}

func TestTodoOperationSupervisorOwnsOnlyRuntimeParticipants(t *testing.T) {
	events := make([]string, 0, 3)
	owner, err := newTodoOperationSupervisor([]supervisor.Participant{
		&compositionParticipant{name: "kernel", events: &events},
	})
	if err != nil {
		t.Fatalf("newTodoOperationSupervisor() error = %v", err)
	}
	if err := owner.RunOperation(t.Context(), func(context.Context) error {
		events = append(events, "operation")
		return nil
	}); err != nil {
		t.Fatalf("RunOperation() error = %v", err)
	}
	want := []string{"start:kernel", "operation", "stop:kernel"}
	if fmt.Sprint(events) != fmt.Sprint(want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
	snapshot := owner.Snapshot()
	if snapshot.State != supervisor.StateStopped || len(snapshot.Units) != 1 || snapshot.Units[0].Owner != "kernel" {
		t.Fatalf("Snapshot() = %#v", snapshot)
	}
	for _, unit := range snapshot.Units {
		if unit.State != supervisor.UnitStopped {
			t.Fatalf("unit = %#v, want stopped", unit)
		}
	}
}

type compositionParticipant struct {
	name   string
	events *[]string
}

func (p *compositionParticipant) Name() string { return p.name }
func (p *compositionParticipant) Start(context.Context) error {
	*p.events = append(*p.events, "start:"+p.name)
	return nil
}
func (p *compositionParticipant) Stop(context.Context) error {
	*p.events = append(*p.events, "stop:"+p.name)
	return nil
}

func TestReloadErrorReporterClassifiesAndRedacts(t *testing.T) {
	tests := []struct {
		err     error
		level   string
		message string
		fields  int
	}{
		{errors.New("candidate failed"), "warn", "application generation reload rejected; previous generation remains active", 1},
		{&kernel.GenerationOperationError{Phase: "prepare", Owner: "application-generation", Generation: 2, Err: errors.New("candidate failed")}, "warn", "application generation reload rejected; previous generation remains active", 5},
		{&kernel.CommittedCleanupError{Err: &kernel.GenerationOperationError{Phase: "retire", Owner: "application-generation", Generation: 1, Err: errors.New("close failed")}}, "error", "application generation reload applied with cleanup debt", 5},
	}
	for _, test := range tests {
		log := logger.NewTestLogger()
		reloadErrorReporter(log)(test.err)
		entries := log.Entries()
		if len(entries) != 1 || entries[0].Level != test.level || entries[0].Message != test.message || len(entries[0].Fields) != test.fields {
			t.Fatalf("entries = %#v", entries)
		}
	}

	path := filepath.Join(t.TempDir(), "reload.log")
	addCaller := false
	resource, err := logger.New(&logger.Config{
		Environment: logger.EnvironmentProduction, OutputPaths: []string{path}, ErrorOutputPaths: []string{path}, AddCaller: &addCaller,
	})
	if err != nil {
		t.Fatalf("logger.New() error = %v", err)
	}
	secret := "postgres://user:top-secret@example.invalid/app"
	reloadErrorReporter(resource)(errors.New("connect " + secret))
	if err := resource.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if bytes.Contains(payload, []byte(secret)) || bytes.Contains(payload, []byte("top-secret")) {
		t.Fatalf("reload log leaked secret: %s", payload)
	}
}

func TestReportServiceFailureClassifiesWithoutErrorText(t *testing.T) {
	log := logger.NewTestLogger()
	sensitiveDetail := errors.New("unsafe detail UNSAFE_ERROR_DETAIL_SENTINEL")
	reportServiceFailure(log, "run", &kernel.GenerationOperationError{
		Phase: "prepare", Owner: "application-generation", Generation: 2, Err: sensitiveDetail,
	})
	entries := log.Entries()
	if len(entries) != 1 || entries[0].Level != "error" || entries[0].Message != "application service failed" || len(entries[0].Fields) != 7 {
		t.Fatalf("entries = %#v", entries)
	}

	path := filepath.Join(t.TempDir(), "service-failure.log")
	addCaller := false
	resource, err := logger.New(&logger.Config{
		Environment: logger.EnvironmentProduction,
		OutputPaths: []string{path}, ErrorOutputPaths: []string{path}, AddCaller: &addCaller,
	})
	if err != nil {
		t.Fatalf("logger.New() error = %v", err)
	}
	reportServiceFailure(resource, "run", &kernel.GenerationOperationError{
		Phase: "prepare", Owner: "application-generation", Generation: 2, Err: sensitiveDetail,
	})
	if err := resource.Close(); err != nil {
		t.Fatalf("logger.Close() error = %v", err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if bytes.Contains(payload, []byte("UNSAFE_ERROR_DETAIL_SENTINEL")) || !bytes.Contains(payload, []byte("cause_type")) {
		t.Fatalf("service failure log is not safely classified: %s", payload)
	}
}

func TestMigrationLogsClassifyCompletionAndFailureWithoutErrorText(t *testing.T) {
	log := logger.NewTestLogger()
	logMigrationCompleted(log, "db.migrate.status", migration.Status{Current: 1, Target: 1, Compatible: true})
	logMigrationCompleted(log, "db.migrate.status", migration.Status{Current: 1, Target: 2, Compatible: false})
	entries := log.Entries()
	if len(entries) != 2 || entries[0].Level != "info" || entries[1].Level != "warn" {
		t.Fatalf("migration completion entries = %#v", entries)
	}

	path := filepath.Join(t.TempDir(), "migration.log")
	addCaller := false
	resource, err := logger.New(&logger.Config{
		Environment: logger.EnvironmentProduction,
		OutputPaths: []string{path}, ErrorOutputPaths: []string{path}, AddCaller: &addCaller,
	})
	if err != nil {
		t.Fatalf("logger.New() error = %v", err)
	}
	logMigrationFailed(resource, "db.migrate.up", "run", errors.New("connect postgres://user:secret@example.invalid/app"))
	if err := resource.Close(); err != nil {
		t.Fatalf("logger.Close() error = %v", err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if bytes.Contains(payload, []byte("secret")) || !bytes.Contains(payload, []byte("error_type")) {
		t.Fatalf("migration log is not safely classified: %s", payload)
	}
}

func TestExpectedServiceShutdownRequiresCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if !expectedServiceShutdown(ctx, context.Canceled) {
		t.Fatal("cancelled service shutdown was not recognized")
	}
	if expectedServiceShutdown(t.Context(), context.Canceled) {
		t.Fatal("active context treated cancellation error as expected shutdown")
	}
}

func TestReloadErrorReporterIgnoresNilInputs(t *testing.T) {
	reloadErrorReporter(nil)(errors.New("ignored"))
	log := logger.NewTestLogger()
	reloadErrorReporter(log)(nil)
	if len(log.Entries()) != 0 {
		t.Fatalf("entries = %#v", log.Entries())
	}
}
