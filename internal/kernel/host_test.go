package kernel

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

func TestHostRunsKernelBeforeAndStopsKernelAfterUpperParticipants(t *testing.T) {
	events := &eventLog{}
	assembly := newTestAssembly(t, &mutableSource{values: versionValues("service", "v1")}, Options{})
	assembly.add(t, "service", app.KernelInstanceSwap, events, nil, nil)
	assembly.install(t)
	runtime := assembly.runtime
	ctx, cancel := context.WithCancel(t.Context())
	server := &hostParticipant{name: "server", events: events}
	worker := &hostParticipant{
		name:   "worker",
		events: events,
		onStart: func() {
			cancel()
		},
	}
	host, err := NewHost(runtime, HostOptions{}, server, worker)
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

func TestNewHostValidatesExplicitWatchOptions(t *testing.T) {
	if _, err := NewHost(nil, HostOptions{}); err == nil {
		t.Fatal("NewHost(nil) error = nil")
	}

	path := createHostVersionFile(t, "v1")
	runtime, err := New(config.New(config.FileSource(path)), Options{Logging: newTestLoggingManager(t)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := NewHost(runtime, HostOptions{Watch: &WatchOptions{}}); err == nil {
		t.Fatal("NewHost(nil reload callback) error = nil")
	}

	mapAssembly := newTestAssembly(t, &mutableSource{values: versionValues("service", "v1")}, Options{})
	mapAssembly.install(t)
	mapRuntime := mapAssembly.runtime
	if _, err := NewHost(mapRuntime, HostOptions{Watch: &WatchOptions{OnReloadError: func(error) {}}}); err == nil {
		t.Fatal("NewHost(watch without FileSource) error = nil")
	}
}

func TestHostWatchReloadsCapabilityAndStopsOnCancellation(t *testing.T) {
	path := createHostVersionFile(t, "v1")
	assembly := newTestAssembly(t, config.FileSource(path), Options{
		Debounce:      30 * time.Millisecond,
		ReloadTimeout: time.Second,
	})
	access := assembly.add(t, "service", app.KernelInstanceSwap, &eventLog{}, nil, nil)
	assembly.install(t)
	runtime := assembly.runtime
	reloadErrors := make(chan error, 1)
	host, err := NewHost(runtime, HostOptions{
		Watch: &WatchOptions{OnReloadError: func(err error) { reloadErrors <- err }},
	})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- host.Run(ctx) }()
	waitForAccessVersion(t, access, "v1")
	time.Sleep(100 * time.Millisecond)

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
}

type hostParticipant struct {
	name    string
	events  *eventLog
	onStart func()
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
