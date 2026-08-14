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

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

type testConfig struct {
	Version string `mapstructure:"version"`
}

type testInstance struct {
	name    string
	version string
}

type testAccess interface {
	Use(context.Context, func(*testInstance) error) error
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

type testAssembly struct {
	runtime     *Kernel
	coordinator *Coordinator
	plan        *app.Plan
}

func newTestAssembly(t *testing.T, source config.Source, options Options) *testAssembly {
	t.Helper()
	if options.Logging == nil {
		options.Logging = newTestLoggingManager(t)
	}
	runtime, err := New(config.New(source), options)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return &testAssembly{runtime: runtime, plan: app.NewPlan()}
}

func (a *testAssembly) add(t *testing.T, name string, policy app.ReloadPolicy, log *eventLog, beforeBuild func(context.Context, testConfig) error, stop func(*testInstance) error) testAccess {
	t.Helper()
	source, err := app.Configured(name, func(snapshot config.Snapshot) (testConfig, error) {
		var cfg testConfig
		if err := snapshot.DecodeSection(name, &cfg); err != nil {
			return testConfig{}, err
		}
		if cfg.Version == "" {
			return testConfig{}, fmt.Errorf("version is required")
		}
		return cfg, nil
	}, nil)
	if err != nil {
		t.Fatalf("Configured(%s) error = %v", name, err)
	}
	definition, err := app.ManagedConfigured(
		app.ID(name), source, app.FixedDependencies(struct{}{}),
		func(ctx context.Context, cfg testConfig, _ struct{}) (*testInstance, error) {
			if beforeBuild != nil {
				if err := beforeBuild(ctx, cfg); err != nil {
					return nil, err
				}
			}
			log.add("build:" + name + ":" + cfg.Version)
			return &testInstance{name: name, version: cfg.Version}, nil
		},
		app.Leased(func(lease app.Lease[*testInstance]) (testAccess, error) { return lease, nil }),
		policy,
		app.WithStart(func(_ context.Context, instance *testInstance) error {
			log.add("start:" + name + ":" + instance.version)
			return nil
		}),
		app.WithStop(func(_ context.Context, instance *testInstance) error {
			log.add("stop:" + name + ":" + instance.version)
			if stop != nil {
				return stop(instance)
			}
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("ManagedConfigured(%s) error = %v", name, err)
	}
	added, err := app.Add(a.plan, definition)
	if err != nil {
		t.Fatalf("Add(%s) error = %v", name, err)
	}
	return added.Output
}

func (a *testAssembly) install(t *testing.T) {
	a.installWith(t)
}

func (a *testAssembly) installWith(t *testing.T, bindings ...config.Binding) {
	t.Helper()
	frozen, err := a.plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	if err := a.runtime.Install(frozen); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	a.coordinator, err = NewCoordinator(a.runtime, bindings...)
	if err != nil {
		t.Fatalf("NewCoordinator() error = %v", err)
	}
}

func TestKernelStartsAndStopsInPlanOrder(t *testing.T) {
	assembly := newTestAssembly(t, &mutableSource{values: map[string]any{
		"first": map[string]any{"version": "v1"}, "second": map[string]any{"version": "v1"},
	}}, Options{})
	log := &eventLog{}
	assembly.add(t, "first", app.KernelInstanceSwap, log, nil, nil)
	assembly.add(t, "second", app.KernelInstanceSwap, log, nil, nil)
	assembly.install(t)

	if err := assembly.coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := assembly.coordinator.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	want := []string{
		"build:first:v1", "start:first:v1", "build:second:v1", "start:second:v1",
		"stop:second:v1", "stop:first:v1",
	}
	if got := log.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}
	if err := assembly.coordinator.Stop(t.Context()); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
}

func TestKernelStartFailureCleansPublishedComponentsAndStopsAccess(t *testing.T) {
	assembly := newTestAssembly(t, &mutableSource{values: map[string]any{
		"first": map[string]any{"version": "v1"}, "second": map[string]any{"version": "bad"},
	}}, Options{})
	log := &eventLog{}
	first := assembly.add(t, "first", app.KernelInstanceSwap, log, nil, nil)
	assembly.add(t, "second", app.KernelInstanceSwap, log, func(context.Context, testConfig) error {
		return errors.New("start transaction rejected")
	}, nil)
	assembly.install(t)
	if err := assembly.coordinator.Start(t.Context()); err == nil {
		t.Fatal("Start() error = nil")
	}
	want := []string{"build:first:v1", "start:first:v1", "stop:first:v1"}
	if got := log.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}
	if err := first.Use(t.Context(), func(*testInstance) error { return nil }); !errors.Is(err, app.ErrStopped) {
		t.Fatalf("Use() after failed Start error = %v", err)
	}
	if err := assembly.coordinator.Start(t.Context()); !errors.Is(err, ErrStopped) {
		t.Fatalf("second Start() error = %v, want ErrStopped", err)
	}
}

func TestKernelInstallRequiresFrozenPlanAndIsAtomic(t *testing.T) {
	assembly := newTestAssembly(t, config.MapSource("empty", map[string]any{}), Options{})
	if err := assembly.runtime.Install(app.FrozenPlan{}); err == nil {
		t.Fatal("Install(zero plan) error = nil")
	}
	assembly.install(t)
	second, err := app.NewPlan().Freeze()
	if err != nil {
		t.Fatalf("Freeze(second) error = %v", err)
	}
	if err := assembly.runtime.Install(second); err == nil {
		t.Fatal("Install(second plan) error = nil")
	}
}

func TestReloadKeepsOldAccessAvailableWhileCandidateBuilds(t *testing.T) {
	source := &mutableSource{values: versionValues("service", "v1")}
	assembly := newTestAssembly(t, source, Options{ReloadTimeout: 3 * time.Second})
	buildStarted := make(chan struct{})
	allowBuild := make(chan struct{})
	access := assembly.add(t, "service", app.KernelInstanceSwap, &eventLog{}, func(ctx context.Context, cfg testConfig) error {
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
	}, nil)
	assembly.install(t)
	if err := assembly.coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = assembly.coordinator.Stop(context.Background()) })

	source.set(versionValues("service", "v2"))
	reloadDone := make(chan error, 1)
	go func() {
		_, err := assembly.coordinator.Reload(t.Context())
		reloadDone <- err
	}()
	<-buildStarted
	assertAccessVersion(t, access, "v1")
	close(allowBuild)
	if err := <-reloadDone; err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	assertAccessVersion(t, access, "v2")
}

func TestReloadFailureRetainsEveryOldInstance(t *testing.T) {
	source := &mutableSource{values: map[string]any{
		"first": map[string]any{"version": "v1"}, "second": map[string]any{"version": "v1"},
	}}
	assembly := newTestAssembly(t, source, Options{})
	first := assembly.add(t, "first", app.KernelInstanceSwap, &eventLog{}, nil, nil)
	second := assembly.add(t, "second", app.KernelInstanceSwap, &eventLog{}, func(_ context.Context, cfg testConfig) error {
		if cfg.Version == "bad" {
			return errors.New("candidate rejected")
		}
		return nil
	}, nil)
	assembly.install(t)
	if err := assembly.coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = assembly.coordinator.Stop(context.Background()) })
	source.set(map[string]any{
		"first": map[string]any{"version": "v2"}, "second": map[string]any{"version": "bad"},
	})
	result, err := assembly.coordinator.Reload(t.Context())
	if err == nil || result.Applied {
		t.Fatalf("Reload() = %#v, %v; want rollback error", result, err)
	}
	assertAccessVersion(t, first, "v1")
	assertAccessVersion(t, second, "v1")
}

func TestReloadTimeoutRestoresOldInstance(t *testing.T) {
	source := &mutableSource{values: versionValues("service", "v1")}
	assembly := newTestAssembly(t, source, Options{ReloadTimeout: 100 * time.Millisecond})
	access := assembly.add(t, "service", app.KernelInstanceSwap, &eventLog{}, nil, nil)
	assembly.install(t)
	if err := assembly.coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = assembly.coordinator.Stop(context.Background()) })
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
	result, err := assembly.coordinator.Reload(t.Context())
	if err == nil || result.Applied {
		t.Fatalf("Reload() = %#v, %v; want timeout rollback", result, err)
	}
	close(release)
	assertAccessVersion(t, access, "v1")
}

func TestReloadCleanupErrorKeepsCommittedCandidate(t *testing.T) {
	cleanupErr := errors.New("close old failed")
	source := &mutableSource{values: versionValues("service", "v1")}
	assembly := newTestAssembly(t, source, Options{})
	access := assembly.add(t, "service", app.KernelInstanceSwap, &eventLog{}, nil, func(instance *testInstance) error {
		if instance.version == "v1" {
			return cleanupErr
		}
		return nil
	})
	assembly.install(t)
	if err := assembly.coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = assembly.coordinator.Stop(context.Background()) })
	source.set(versionValues("service", "v2"))
	result, err := assembly.coordinator.Reload(t.Context())
	var committed *CommittedCleanupError
	if !result.Applied || !errors.As(err, &committed) || !errors.Is(err, cleanupErr) {
		t.Fatalf("Reload() = %#v, %v; want committed cleanup error", result, err)
	}
	assertAccessVersion(t, access, "v2")
	diagnostics := assembly.coordinator.Diagnostics()
	if diagnostics.State != LifecycleDegraded || diagnostics.Ready || !diagnostics.RestartRequired || diagnostics.Generation != 2 {
		t.Fatalf("Diagnostics() = %#v, want degraded generation 2", diagnostics)
	}
	source.set(versionValues("service", "v3"))
	if _, err := assembly.coordinator.Reload(t.Context()); err == nil {
		t.Fatal("Reload() after committed cleanup failure error = nil")
	}
	assertAccessVersion(t, access, "v2")
}

func TestReloadAndStopCloseEachGenerationOnce(t *testing.T) {
	source := &mutableSource{values: versionValues("service", "v1")}
	assembly := newTestAssembly(t, source, Options{})
	log := &eventLog{}
	access := assembly.add(t, "service", app.KernelInstanceSwap, log, nil, nil)
	assembly.install(t)
	if err := assembly.coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	source.set(versionValues("service", "v2"))
	result, err := assembly.coordinator.Reload(t.Context())
	if err != nil || !result.Applied {
		t.Fatalf("Reload() = %#v, %v", result, err)
	}
	assertAccessVersion(t, access, "v2")
	if err := assembly.coordinator.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	want := []string{
		"build:service:v1", "start:service:v1",
		"build:service:v2", "start:service:v2",
		"stop:service:v1", "stop:service:v2",
	}
	if got := log.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}
}

func TestTerminalDrainTimeoutDoesNotResumeOrForceCloseActiveLease(t *testing.T) {
	source := &mutableSource{values: versionValues("service", "v1")}
	assembly := newTestAssembly(t, source, Options{})
	log := &eventLog{}
	access := assembly.add(t, "service", app.KernelInstanceSwap, log, nil, nil)
	assembly.install(t)
	if err := assembly.coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	release := make(chan struct{})
	using := make(chan struct{})
	useDone := make(chan error, 1)
	go func() {
		useDone <- access.Use(context.Background(), func(*testInstance) error {
			close(using)
			<-release
			return nil
		})
	}()
	<-using
	stopCtx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err := assembly.coordinator.Stop(stopCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop() error = %v, want terminal drain deadline", err)
	}
	blockedCtx, blockedCancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer blockedCancel()
	if err := access.Use(blockedCtx, func(*testInstance) error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Use() after terminal drain error = %v, want not resumed", err)
	}
	if got := log.snapshot(); containsEvent(got, "stop:service:v1") {
		t.Fatalf("terminal timeout force-closed active instance: %#v", got)
	}
	close(release)
	if err := <-useDone; err != nil {
		t.Fatalf("active Use() error = %v", err)
	}
}

func containsEvent(events []string, want string) bool {
	for _, event := range events {
		if event == want {
			return true
		}
	}
	return false
}

func TestReloadRestartRequiredHasNoPartialSideEffects(t *testing.T) {
	source := &mutableSource{values: map[string]any{
		"swap": map[string]any{"version": "v1"}, "fixed-port": map[string]any{"version": "v1"},
	}}
	assembly := newTestAssembly(t, source, Options{})
	log := &eventLog{}
	swapAccess := assembly.add(t, "swap", app.KernelInstanceSwap, log, nil, nil)
	restartAccess := assembly.add(t, "fixed-port", app.RestartRequired, log, nil, nil)
	assembly.install(t)
	if err := assembly.coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = assembly.coordinator.Stop(context.Background()) })
	before := log.snapshot()
	source.set(map[string]any{
		"swap": map[string]any{"version": "v2"}, "fixed-port": map[string]any{"version": "v2"},
	})
	result, err := assembly.coordinator.Reload(t.Context())
	if !errors.Is(err, app.ErrRestartRequired) || result.Applied {
		t.Fatalf("Reload() = %#v, %v; want restart required", result, err)
	}
	if !reflect.DeepEqual(result.RestartRequired, []app.ID{"fixed-port"}) {
		t.Fatalf("RestartRequired = %#v", result.RestartRequired)
	}
	if result.CurrentDigest != result.PreviousDigest {
		t.Fatalf("restart-required digest changed: %#v", result)
	}
	if got := log.snapshot(); !reflect.DeepEqual(got, before) {
		t.Fatalf("events changed = %#v, want %#v", got, before)
	}
	assertAccessVersion(t, swapAccess, "v1")
	assertAccessVersion(t, restartAccess, "v1")
}

func TestReloadRequiresRestartForApplicationOwnedSection(t *testing.T) {
	source := &mutableSource{values: map[string]any{
		"service": map[string]any{"version": "v1"}, "other": map[string]any{"value": "one"},
	}}
	assembly := newTestAssembly(t, source, Options{})
	log := &eventLog{}
	assembly.add(t, "service", app.KernelInstanceSwap, log, nil, nil)
	assembly.installWith(t, config.Binding{
		CapabilityID: "application.other",
		ConfigPath:   "other",
		Validate: func(snapshot config.Snapshot) error {
			var value struct {
				Value string `mapstructure:"value"`
			}
			return snapshot.DecodeSection("other", &value)
		},
	})
	if err := assembly.coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = assembly.coordinator.Stop(context.Background()) })
	source.set(map[string]any{
		"service": map[string]any{"version": "v1"}, "other": map[string]any{"value": "two"},
	})
	result, err := assembly.coordinator.Reload(t.Context())
	if !errors.Is(err, app.ErrRestartRequired) || result.Applied || len(result.RestartRequired) != 1 {
		t.Fatalf("Reload() = %#v, %v; want application restart requirement", result, err)
	}
}

func TestWatchReportsReloadErrorAndContinues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeVersionFile(t, path, "v1")
	assembly := newTestAssembly(t, config.FileSource(path), Options{Debounce: 30 * time.Millisecond, ReloadTimeout: time.Second})
	access := assembly.add(t, "service", app.KernelInstanceSwap, &eventLog{}, func(_ context.Context, cfg testConfig) error {
		if cfg.Version == "bad" {
			return errors.New("bad config")
		}
		return nil
	}, nil)
	assembly.install(t)
	if err := assembly.coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = assembly.coordinator.Stop(context.Background()) })
	watchCtx, cancel := context.WithCancel(t.Context())
	errorsSeen := make(chan error, 2)
	watchDone := make(chan error, 1)
	go func() { watchDone <- assembly.coordinator.Watch(watchCtx, func(err error) { errorsSeen <- err }) }()
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
	waitForAccessVersion(t, access, "v2")
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

func versionValues(name, version string) map[string]any {
	return map[string]any{name: map[string]any{"version": version}}
}

func assertAccessVersion(t *testing.T, access testAccess, want string) {
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

func waitForAccessVersion(t *testing.T, access testAccess, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		var version string
		err := access.Use(ctx, func(instance *testInstance) error {
			version = instance.version
			return nil
		})
		cancel()
		if err == nil && version == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("access version = %q, error = %v, want %q", version, err, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func writeVersionFile(t *testing.T, path, version string) {
	t.Helper()
	content := []byte("service:\n  version: " + version + "\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config %s: %v", version, err)
	}
}
