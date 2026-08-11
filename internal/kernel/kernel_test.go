package kernel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

type testConfig struct {
	Version string `mapstructure:"version"`
}

type testInstance struct {
	name    string
	version string
}

type mutableSource struct {
	mu     sync.Mutex
	values map[string]any
}

func (s *mutableSource) Name() string { return "mutable" }

func (s *mutableSource) Load(context.Context) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]any, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func (s *mutableSource) set(values map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values = values
}

type eventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *eventLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *eventLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.events...)
}

func TestKernelStartsAndStopsInRegistrationOrder(t *testing.T) {
	source := &mutableSource{values: map[string]any{
		"first":  map[string]any{"version": "v1"},
		"second": map[string]any{"version": "v1"},
	}}
	runtime := newTestKernel(t, source, Options{})
	log := &eventLog{}
	registerTestComponent(t, runtime, "first", log, nil)
	registerTestComponent(t, runtime, "second", log, nil)

	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	want := []string{
		"build:first:v1", "start:first:v1",
		"build:second:v1", "start:second:v1",
		"stop:second:v1", "stop:first:v1",
	}
	if got := log.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
}

func TestReloadDrainsOldUseWhileBuildingCandidate(t *testing.T) {
	source := &mutableSource{values: versionValues("service", "v1")}
	runtime := newTestKernel(t, source, Options{ReloadTimeout: 3 * time.Second})
	buildStarted := make(chan struct{})
	allowBuild := make(chan struct{})
	access := registerTestComponent(t, runtime, "service", &eventLog{}, func(ctx context.Context, cfg testConfig) error {
		if cfg.Version != "v2" {
			return nil
		}
		close(buildStarted)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-allowBuild:
			return nil
		}
	})
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Stop(context.Background()) })

	oldUseStarted := make(chan struct{})
	releaseOldUse := make(chan struct{})
	oldUseDone := make(chan error, 1)
	go func() {
		oldUseDone <- access.Use(t.Context(), func(instance *testInstance) error {
			if instance.version != "v1" {
				return fmt.Errorf("old version = %s", instance.version)
			}
			close(oldUseStarted)
			<-releaseOldUse
			return nil
		})
	}()
	<-oldUseStarted

	source.set(versionValues("service", "v2"))
	reloadDone := make(chan error, 1)
	go func() {
		_, err := runtime.Reload(t.Context())
		reloadDone <- err
	}()
	<-buildStarted

	newUseEntered := make(chan string, 1)
	newUseDone := make(chan error, 1)
	go func() {
		newUseDone <- access.Use(t.Context(), func(instance *testInstance) error {
			newUseEntered <- instance.version
			return nil
		})
	}()
	select {
	case version := <-newUseEntered:
		t.Fatalf("new Use entered during drain with version %s", version)
	case <-time.After(100 * time.Millisecond):
	}

	close(allowBuild)
	select {
	case err := <-reloadDone:
		t.Fatalf("Reload() completed before old use drained: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseOldUse)
	if err := <-oldUseDone; err != nil {
		t.Fatalf("old Use() error = %v", err)
	}
	if err := <-reloadDone; err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if err := <-newUseDone; err != nil {
		t.Fatalf("new Use() error = %v", err)
	}
	if version := <-newUseEntered; version != "v2" {
		t.Fatalf("new version = %s, want v2", version)
	}
}

func TestReloadFailureRestoresEveryOldInstance(t *testing.T) {
	source := &mutableSource{values: map[string]any{
		"first":  map[string]any{"version": "v1"},
		"second": map[string]any{"version": "v1"},
	}}
	runtime := newTestKernel(t, source, Options{})
	first := registerTestComponent(t, runtime, "first", &eventLog{}, nil)
	second := registerTestComponent(t, runtime, "second", &eventLog{}, func(_ context.Context, cfg testConfig) error {
		if cfg.Version == "bad" {
			return errors.New("candidate rejected")
		}
		return nil
	})
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Stop(context.Background()) })

	source.set(map[string]any{
		"first":  map[string]any{"version": "v2"},
		"second": map[string]any{"version": "bad"},
	})
	result, err := runtime.Reload(t.Context())
	if err == nil || result.Applied {
		t.Fatalf("Reload() = %#v, %v; want rollback error", result, err)
	}
	assertAccessVersion(t, first, "v1")
	assertAccessVersion(t, second, "v1")
}

func TestReloadCleanupErrorKeepsCommittedCandidate(t *testing.T) {
	cleanupErr := errors.New("close old failed")
	source := &mutableSource{values: versionValues("service", "v1")}
	runtime := newTestKernel(t, source, Options{})
	access := registerTestComponentWithStop(t, runtime, "service", func(instance *testInstance) error {
		if instance.version == "v1" {
			return cleanupErr
		}
		return nil
	})
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Stop(context.Background()) })

	source.set(versionValues("service", "v2"))
	result, err := runtime.Reload(t.Context())
	var committed *CommittedCleanupError
	if !result.Applied || !errors.As(err, &committed) || !errors.Is(err, cleanupErr) {
		t.Fatalf("Reload() = %#v, %v; want committed cleanup error", result, err)
	}
	assertAccessVersion(t, access, "v2")
}

func TestReloadTimeoutRestoresOldInstance(t *testing.T) {
	source := &mutableSource{values: versionValues("service", "v1")}
	runtime := newTestKernel(t, source, Options{ReloadTimeout: 100 * time.Millisecond})
	access := registerTestComponent(t, runtime, "service", &eventLog{}, nil)
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Stop(context.Background()) })

	holding := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = access.Use(t.Context(), func(*testInstance) error {
			close(holding)
			<-release
			return nil
		})
	}()
	<-holding
	source.set(versionValues("service", "v2"))
	result, err := runtime.Reload(t.Context())
	if err == nil || result.Applied {
		t.Fatalf("Reload() = %#v, %v; want timeout rollback", result, err)
	}
	close(release)
	assertAccessVersion(t, access, "v1")
}

func TestReloadSkipsUnchangedComponent(t *testing.T) {
	source := &mutableSource{values: map[string]any{
		"service": map[string]any{"version": "v1"},
		"other":   map[string]any{"value": "one"},
	}}
	runtime := newTestKernel(t, source, Options{})
	log := &eventLog{}
	registerTestComponent(t, runtime, "service", log, nil)
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Stop(context.Background()) })

	source.set(map[string]any{
		"service": map[string]any{"version": "v1"},
		"other":   map[string]any{"value": "two"},
	})
	result, err := runtime.Reload(t.Context())
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if result.Applied || len(result.Changed) != 0 {
		t.Fatalf("Reload() result = %#v, want unchanged", result)
	}
	if got := log.snapshot(); !reflect.DeepEqual(got, []string{"build:service:v1", "start:service:v1"}) {
		t.Fatalf("events = %#v", got)
	}
}

func TestRegisterRejectsDuplicateID(t *testing.T) {
	source := &mutableSource{values: versionValues("service", "v1")}
	runtime := newTestKernel(t, source, Options{})
	registerTestComponent(t, runtime, "service", &eventLog{}, nil)
	definition := testDefinition("service", &eventLog{}, nil, nil)
	if _, err := Register(runtime, definition); err == nil {
		t.Fatal("Register(duplicate) error = nil")
	}
}

func newTestKernel(t *testing.T, source config.Source, options Options) *Kernel {
	t.Helper()
	runtime, err := New(config.New(source), options)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return runtime
}

func registerTestComponent(
	t *testing.T,
	runtime *Kernel,
	name string,
	log *eventLog,
	beforeBuild func(context.Context, testConfig) error,
) *Handle[*testInstance] {
	t.Helper()
	definition := testDefinition(name, log, beforeBuild, nil)
	access, err := Register(runtime, definition)
	if err != nil {
		t.Fatalf("Register(%s) error = %v", name, err)
	}
	return access
}

func registerTestComponentWithStop(t *testing.T, runtime *Kernel, name string, stop func(*testInstance) error) *Handle[*testInstance] {
	t.Helper()
	definition := testDefinition(name, &eventLog{}, nil, stop)
	access, err := Register(runtime, definition)
	if err != nil {
		t.Fatalf("Register(%s) error = %v", name, err)
	}
	return access
}

func testDefinition(
	name string,
	log *eventLog,
	beforeBuild func(context.Context, testConfig) error,
	stop func(*testInstance) error,
) Definition[testConfig, *testInstance] {
	return Definition[testConfig, *testInstance]{
		ID:         ID(name),
		ConfigPath: name,
		Decode: func(snapshot config.Snapshot) (testConfig, error) {
			var cfg testConfig
			if err := snapshot.DecodeSection(name, &cfg); err != nil {
				return testConfig{}, err
			}
			if cfg.Version == "" {
				return testConfig{}, fmt.Errorf("version is required")
			}
			return cfg, nil
		},
		Builder: BuilderFunc[testConfig, *testInstance](func(ctx context.Context, cfg testConfig) (*testInstance, error) {
			if beforeBuild != nil {
				if err := beforeBuild(ctx, cfg); err != nil {
					return nil, err
				}
			}
			log.add("build:" + name + ":" + cfg.Version)
			return &testInstance{name: name, version: cfg.Version}, nil
		}),
		Hooks: InstanceHookFuncs[*testInstance]{
			OnStart: func(_ context.Context, instance *testInstance) error {
				log.add("start:" + name + ":" + instance.version)
				return nil
			},
			OnStop: func(_ context.Context, instance *testInstance) error {
				log.add("stop:" + name + ":" + instance.version)
				if stop != nil {
					return stop(instance)
				}
				return nil
			},
		},
	}
}

func versionValues(name, version string) map[string]any {
	return map[string]any{name: map[string]any{"version": version}}
}

func assertAccessVersion(t *testing.T, access *Handle[*testInstance], want string) {
	t.Helper()
	if err := access.Use(t.Context(), func(instance *testInstance) error {
		if instance.version != want {
			return fmt.Errorf("version = %s, want %s", instance.version, want)
		}
		return nil
	}); err != nil {
		t.Fatalf("Use() error = %v", err)
	}
}

func TestWatchReportsReloadErrorAndContinues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeVersionFile(t, path, "v1")
	runtime, err := New(config.New(config.FileSource(path)), Options{
		Debounce:      30 * time.Millisecond,
		ReloadTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	access := registerTestComponent(t, runtime, "service", &eventLog{}, func(_ context.Context, cfg testConfig) error {
		if cfg.Version == "bad" {
			return errors.New("bad config")
		}
		return nil
	})
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Stop(context.Background()) })

	watchCtx, cancel := context.WithCancel(t.Context())
	errorsSeen := make(chan error, 2)
	watchDone := make(chan error, 1)
	go func() {
		watchDone <- runtime.Watch(watchCtx, func(err error) {
			errorsSeen <- err
		})
	}()
	time.Sleep(100 * time.Millisecond)
	writeVersionFile(t, path, "bad")
	select {
	case err := <-errorsSeen:
		if err == nil {
			t.Fatal("Watch() reported nil reload error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for reload error")
	}

	writeVersionFile(t, path, "v2")
	deadline := time.Now().Add(3 * time.Second)
	for {
		var version string
		err := access.Use(t.Context(), func(instance *testInstance) error {
			version = instance.version
			return nil
		})
		if err == nil && version == "v2" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("watch did not apply v2; version = %s, error = %v", version, err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-watchDone:
		if err != nil {
			t.Fatalf("Watch() shutdown error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out stopping Watch()")
	}
}

func writeVersionFile(t *testing.T, path, version string) {
	t.Helper()
	content := []byte("service:\n  version: " + version + "\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config %s: %v", version, err)
	}
}
