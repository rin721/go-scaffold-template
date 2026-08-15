package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold-template/internal/kernel/logging"
	pkghealth "github.com/rin721/go-scaffold-template/pkg/health"
	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

func TestHostRunsKernelBeforeAndStopsKernelAfterUpperParticipants(t *testing.T) {
	events := &eventLog{}
	assembly := newTestAssembly(t, &mutableSource{values: versionValues("service", "v1")}, Options{})
	assembly.add(t, "service", app.KernelInstanceSwap, events, nil, nil)
	assembly.install(t)
	ctx, cancel := context.WithCancel(t.Context())
	server := &hostParticipant{name: "server", events: events}
	worker := &hostParticipant{
		name:   "worker",
		events: events,
		onStart: func() {
			cancel()
		},
	}
	host, err := NewHost(assembly.coordinator, HostOptions{}, server, worker)
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}

	if err := host.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{
		"build:service:v1",
		"start:service:v1",
		"start:server",
		"start:worker",
		"stop:worker",
		"stop:server",
		"stop:service:v1",
	}
	if got := events.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}
}

func TestHostReadinessAndHealthFollowSupervisedLifecycle(t *testing.T) {
	assembly := newTestAssembly(t, &mutableSource{values: versionValues("service", "v1")}, Options{})
	assembly.add(t, "service", app.KernelInstanceSwap, &eventLog{}, nil, nil)
	assembly.install(t)
	host, err := NewHost(assembly.coordinator, HostOptions{})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- host.Run(ctx) }()
	<-host.Ready()
	diagnostics := host.Diagnostics()
	if !diagnostics.Ready || diagnostics.KernelState != LifecycleRunning {
		t.Fatalf("Diagnostics() = %#v, want ready", diagnostics)
	}
	capability := findResponsibility(t, diagnostics, OwnerCapability, "service")
	if capability.Generation != 1 || capability.State != ResponsibilityServing || capability.ExitPolicy != ExitDrainThenTerminalClose {
		t.Fatalf("capability responsibility = %#v", capability)
	}
	participant := findResponsibility(t, diagnostics, OwnerParticipant, "kernel")
	if participant.State != ResponsibilityReady || participant.ExitPolicy != ExitGracefulShutdown {
		t.Fatalf("kernel participant responsibility = %#v", participant)
	}
	if snapshot := host.Health(t.Context()); snapshot.Status != pkghealth.StatusPass {
		t.Fatalf("Health() = %#v, want pass", snapshot)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if snapshot := host.Health(t.Context()); snapshot.Status != pkghealth.StatusFail {
		t.Fatalf("Health() after stop = %#v, want fail", snapshot)
	}
}

func TestHostDiagnosticsPreservesUncooperativeParticipantAndTask(t *testing.T) {
	assembly := newTestAssembly(t, &mutableSource{values: versionValues("service", "v1")}, Options{})
	assembly.add(t, "service", app.KernelInstanceSwap, &eventLog{}, nil, nil)
	assembly.install(t)
	participantRelease := make(chan struct{})
	participantDone := make(chan struct{})
	taskRelease := make(chan struct{})
	taskDone := make(chan struct{})
	ready := make(chan struct{})
	close(ready)
	participant := &blockingHostParticipant{release: participantRelease, done: participantDone}
	host, err := NewHost(assembly.coordinator, HostOptions{
		ShutdownTimeout:      80 * time.Millisecond,
		ForceShutdownTimeout: 20 * time.Millisecond,
		Runners: []supervisor.Task{{
			Name: "blocking-task", Ready: ready,
			Run: func(context.Context) error {
				defer close(taskDone)
				<-taskRelease
				return nil
			},
		}},
	}, participant)
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- host.Run(ctx) }()
	<-host.Ready()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Run() error = nil, want incomplete shutdown")
		}
	case <-time.After(time.Second):
		t.Fatal("Host did not honor the shared shutdown budget")
	}

	diagnostics := host.Diagnostics()
	if diagnostics.Ready || diagnostics.ProcessState != supervisor.StateFailed || !diagnostics.ShutdownBudget.Exhausted {
		t.Fatalf("Diagnostics() = %#v, want failed exhausted process", diagnostics)
	}
	participantSnapshot := findResponsibility(t, diagnostics, OwnerParticipant, participant.Name())
	if participantSnapshot.Phase != ResponsibilityPhase(supervisor.UnitPhaseStop) || participantSnapshot.State != ResponsibilityPending {
		t.Fatalf("participant responsibility = %#v, want stop pending", participantSnapshot)
	}
	taskSnapshot := findResponsibility(t, diagnostics, OwnerTask, "blocking-task")
	if taskSnapshot.Phase != ResponsibilityPhase(supervisor.UnitPhaseStop) || taskSnapshot.State != ResponsibilityPending {
		t.Fatalf("task responsibility = %#v, want stop pending", taskSnapshot)
	}

	close(participantRelease)
	close(taskRelease)
	waitForTestOwnerDone(t, participantDone, "participant")
	waitForTestOwnerDone(t, taskDone, "task")
}

func findResponsibility(t *testing.T, diagnostics ProcessDiagnostics, kind OwnerKind, owner string) ResponsibilitySnapshot {
	t.Helper()
	for _, responsibility := range diagnostics.Responsibilities {
		if responsibility.Kind == kind && responsibility.Owner == owner {
			return responsibility
		}
	}
	t.Fatalf("responsibility %s/%s not found in %#v", kind, owner, diagnostics.Responsibilities)
	return ResponsibilitySnapshot{}
}

func TestNewHostValidatesExplicitWatchOptions(t *testing.T) {
	if _, err := NewHost(nil, HostOptions{}); err == nil {
		t.Fatal("NewHost(nil) error = nil")
	}

	path := createHostVersionFile(t, "v1")
	runtime, err := New(config.New(config.FileSource(path)), Options{Logging: newTestLoggingManager(t)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	coordinator, err := NewCoordinator(runtime)
	if err != nil {
		t.Fatalf("NewCoordinator() error = %v", err)
	}
	if _, err := NewHost(coordinator, HostOptions{Watch: &WatchOptions{}}); err == nil {
		t.Fatal("NewHost(nil reload callback) error = nil")
	}

	mapAssembly := newTestAssembly(t, &mutableSource{values: versionValues("service", "v1")}, Options{})
	mapAssembly.install(t)
	if _, err := NewHost(mapAssembly.coordinator, HostOptions{Watch: &WatchOptions{OnReloadError: func(error) {}}}); err == nil {
		t.Fatal("NewHost(watch without FileSource) error = nil")
	}
}

func TestHostWatchReloadsCapabilityAndStopsOnCancellation(t *testing.T) {
	path := createHostVersionFile(t, "v1")
	baseline := pkglogger.NewTestLogger()
	logging, err := kernellogging.New(baseline)
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	assembly := newTestAssembly(t, config.FileSource(path), Options{
		Debounce:      30 * time.Millisecond,
		ReloadTimeout: time.Second,
		Logging:       logging,
	})
	access := assembly.add(t, "service", app.KernelInstanceSwap, &eventLog{}, nil, nil)
	assembly.install(t)
	reloadErrors := make(chan error, 1)
	host, err := NewHost(assembly.coordinator, HostOptions{
		Watch: &WatchOptions{OnReloadError: func(err error) { reloadErrors <- err }},
	})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- host.Run(ctx) }()
	diagnosticsCtx, stopDiagnostics := context.WithCancel(t.Context())
	diagnosticsDone := make(chan struct{})
	go func() {
		defer close(diagnosticsDone)
		for {
			select {
			case <-diagnosticsCtx.Done():
				return
			default:
				_ = host.Diagnostics()
				runtime.Gosched()
			}
		}
	}()
	waitForAccessVersion(t, access, "v1")
	waitForLogMessage(t, baseline, "kernel reload unchanged")
	watchTask := findResponsibility(t, host.Diagnostics(), OwnerTask, configWatchTaskName)
	if watchTask.ExitPolicy != ExitCancelAndWait {
		t.Fatalf("watch task responsibility = %#v", watchTask)
	}

	writeHostVersionFile(t, path, "v2")
	waitForAccessVersion(t, access, "v2")
	select {
	case err := <-reloadErrors:
		t.Fatalf("unexpected reload error: %v", err)
	default:
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Host did not stop after cancellation")
	}
	stopDiagnostics()
	<-diagnosticsDone
}

func waitForLogMessage(t *testing.T, logging *pkglogger.TestLogger, message string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		for _, entry := range logging.Entries() {
			if entry.Message == message {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("log does not contain %q", message)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestHostWatchReconcilesChangeMadeBeforeWatcherReady(t *testing.T) {
	path := createHostVersionFile(t, "v1")
	assembly := newTestAssembly(t, config.FileSource(path), Options{
		Debounce:      30 * time.Millisecond,
		ReloadTimeout: time.Second,
	})
	access := assembly.add(t, "service", app.KernelInstanceSwap, &eventLog{}, nil, nil)
	assembly.install(t)
	participant := &hostParticipant{
		name:   "application",
		events: &eventLog{},
		onStart: func() {
			writeHostVersionFile(t, path, "v2")
		},
	}
	reloadErrors := make(chan error, 1)
	host, err := NewHost(assembly.coordinator, HostOptions{
		Watch: &WatchOptions{OnReloadError: func(err error) { reloadErrors <- err }},
	}, participant)
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- host.Run(ctx) }()
	waitForAccessVersion(t, access, "v2")
	select {
	case err := <-reloadErrors:
		t.Fatalf("unexpected reconciliation error: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Host did not stop after cancellation")
	}
}

func TestHostWatchInfrastructureFailureStopsUpperParticipantAndKernel(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.yaml")
	writeHostVersionFile(t, path, "v1")
	events := &eventLog{}
	assembly := newTestAssembly(t, config.FileSource(path), Options{})
	access := assembly.add(t, "service", app.KernelInstanceSwap, events, nil, nil)
	assembly.install(t)
	participant := &hostParticipant{
		name:   "application",
		events: events,
		onStart: func() {
			if err := os.Remove(path); err != nil {
				t.Errorf("remove config before watcher ready: %v", err)
			}
			if err := os.Remove(directory); err != nil {
				t.Errorf("remove config directory before watcher ready: %v", err)
			}
		},
	}
	host, err := NewHost(assembly.coordinator, HostOptions{
		Watch: &WatchOptions{OnReloadError: func(error) {}},
	}, participant)
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}

	err = host.Run(t.Context())
	if err == nil || !strings.Contains(err.Error(), "watch config directory") {
		t.Fatalf("Host.Run() error = %v, want watcher registration failure", err)
	}
	want := []string{
		"build:service:v1", "start:service:v1",
		"start:application", "stop:application", "stop:service:v1",
	}
	if got := events.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}
	if err := access.Use(t.Context(), func(*testInstance) error { return nil }); !errors.Is(err, app.ErrStopped) {
		t.Fatalf("Database access after watcher failure error = %v, want ErrStopped", err)
	}
}

type hostParticipant struct {
	name    string
	events  *eventLog
	onStart func()
}

type blockingHostParticipant struct {
	release <-chan struct{}
	done    chan<- struct{}
}

func (*blockingHostParticipant) Name() string { return "blocking-participant" }

func (*blockingHostParticipant) Start(context.Context) error { return nil }

func (p *blockingHostParticipant) Stop(context.Context) error {
	defer close(p.done)
	<-p.release
	return nil
}

func waitForTestOwnerDone(t *testing.T, done <-chan struct{}, owner string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("%s test owner did not exit", owner)
	}
}

func (p *hostParticipant) Name() string { return p.name }

func (p *hostParticipant) Start(context.Context) error {
	p.events.add("start:" + p.name)
	if p.onStart != nil {
		p.onStart()
	}
	return nil
}

func (p *hostParticipant) Stop(context.Context) error {
	p.events.add("stop:" + p.name)
	return nil
}

func createHostVersionFile(t *testing.T, version string) string {
	t.Helper()
	path := t.TempDir() + "/config.yaml"
	writeHostVersionFile(t, path, version)
	return path
}

func writeHostVersionFile(t *testing.T, path string, version string) {
	t.Helper()
	content := []byte("service:\n  version: " + version + "\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config %s: %v", version, err)
	}
}
