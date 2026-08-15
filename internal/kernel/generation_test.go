package kernel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold-template/internal/kernel/logging"
	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
)

type testGenerationFactory struct {
	mu         sync.Mutex
	nextID     uint64
	prepareErr error
	prepared   []*testGeneration
	stopCalls  int
}

func (f *testGenerationFactory) Prepare(
	_ context.Context,
	snapshot config.Snapshot,
	_ ActiveGeneration,
) (PreparedGeneration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.prepareErr != nil {
		return nil, f.prepareErr
	}
	f.nextID++
	generation := &testGeneration{id: f.nextID, snapshot: snapshot}
	f.prepared = append(f.prepared, generation)
	return generation, nil
}

func (f *testGenerationFactory) Stop(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCalls++
	return nil
}

func (f *testGenerationFactory) generation(index int) *testGeneration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.prepared[index]
}

func (f *testGenerationFactory) preparedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.prepared)
}

type testGeneration struct {
	mu             sync.Mutex
	id             uint64
	snapshot       config.Snapshot
	retireErr      error
	stopErr        error
	retireCalls    int
	stopCalls      int
	forceStopCalls int
	abortCalls     int
}

func (g *testGeneration) ID() uint64                { return g.id }
func (g *testGeneration) Snapshot() config.Snapshot { return g.snapshot }
func (*testGeneration) ConfiguredAddress() string   { return "127.0.0.1:8080" }
func (*testGeneration) BoundAddress() string        { return "127.0.0.1:8080" }
func (*testGeneration) ManagementBoundAddress() string {
	return "127.0.0.1:9090"
}
func (*testGeneration) ActiveConnections() int64 { return 0 }
func (*testGeneration) ActiveRequests() int64    { return 0 }
func (*testGeneration) ResourceStats() GenerationResourceStats {
	return GenerationResourceStats{Built: []string{"test"}}
}
func (g *testGeneration) Commit(ActiveGeneration) (ActiveGeneration, error) { return g, nil }

func (g *testGeneration) Abort(context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.abortCalls++
	return nil
}

func (g *testGeneration) Retire(context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.retireCalls++
	return g.retireErr
}

func (g *testGeneration) Stop(context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopCalls++
	return g.stopErr
}

func (g *testGeneration) ForceStop(context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.forceStopCalls++
	return nil
}

func TestGenerationCoordinatorNoOpDoesNotPrepare(t *testing.T) {
	coordinator, _, factory := newTestGenerationCoordinator(t)
	if err := coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	result, err := coordinator.Reload(t.Context())
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if result.Applied || factory.preparedCount() != 1 {
		t.Fatalf("no-op result = %+v, prepared = %d", result, factory.preparedCount())
	}
	if diagnostics := coordinator.Diagnostics(); diagnostics.Phase != "no-op" || !diagnostics.Ready {
		t.Fatalf("Diagnostics() = %+v", diagnostics)
	}
}

func TestGenerationCoordinatorLogsLifecycleMilestones(t *testing.T) {
	log := pkglogger.NewTestLogger()
	manager, err := kernellogging.New(log)
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	coordinator, _, _ := newTestGenerationCoordinatorWithLogging(t, manager)
	if err := coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if _, err := coordinator.Reload(t.Context()); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if err := coordinator.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	entries := log.Entries()
	want := []struct{ level, message string }{
		{"debug", "application generation load started"},
		{"debug", "application generation prepare started"},
		{"debug", "application generation candidate ready"},
		{"debug", "application generation committed"},
		{"info", "application generation started"},
		{"debug", "application generation reload load started"},
		{"debug", "application generation reload unchanged"},
		{"debug", "application generation stopping"},
		{"debug", "application generation stopped"},
	}
	if len(entries) != len(want) {
		t.Fatalf("entries = %#v", entries)
	}
	for index := range want {
		if entries[index].Level != want[index].level || entries[index].Message != want[index].message {
			t.Fatalf("entry %d = %#v, want %#v", index, entries[index], want[index])
		}
	}
}

func TestGenerationCoordinatorLogsSuccessfulReload(t *testing.T) {
	log := pkglogger.NewTestLogger()
	manager, err := kernellogging.New(log)
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	coordinator, source, _ := newTestGenerationCoordinatorWithLogging(t, manager)
	if err := coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := coordinator.Stop(t.Context()); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})
	source.set(map[string]any{"app": map[string]any{"version": "v2"}})
	result, err := coordinator.Reload(t.Context())
	if err != nil || !result.Applied {
		t.Fatalf("Reload() = %#v, %v", result, err)
	}
	entries := log.Entries()
	if len(entries) == 0 || entries[len(entries)-1].Level != "info" || entries[len(entries)-1].Message != "application generation reload completed" {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestGenerationCoordinatorPrepareFailurePreservesCurrent(t *testing.T) {
	coordinator, source, factory := newTestGenerationCoordinator(t)
	if err := coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	factory.prepareErr = errors.New("candidate is not ready")
	source.set(map[string]any{"app": map[string]any{"version": "v2"}})
	if _, err := coordinator.Reload(t.Context()); err == nil {
		t.Fatal("Reload() error = nil, want prepare failure")
	} else {
		var operationErr *GenerationOperationError
		if !errors.As(err, &operationErr) || operationErr.Phase != "prepare" || operationErr.Owner != "application-generation" || !errors.Is(err, factory.prepareErr) {
			t.Fatalf("Reload() error = %#v", err)
		}
	}
	diagnostics := coordinator.Diagnostics()
	if diagnostics.CurrentGeneration != 1 || diagnostics.ConfigDigest != factory.generation(0).snapshot.Digest() {
		t.Fatalf("Diagnostics() changed active generation: %+v", diagnostics)
	}
	if diagnostics.State != LifecycleRunning || !diagnostics.Ready || diagnostics.Phase != "rejected:prepare" {
		t.Fatalf("Diagnostics() = %+v", diagnostics)
	}
}

func TestGenerationCoordinatorRetireFailureCreatesCleanupDebt(t *testing.T) {
	coordinator, source, factory := newTestGenerationCoordinator(t)
	if err := coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	oldGeneration := factory.generation(0)
	oldGeneration.retireErr = errors.New("old requests did not drain")
	source.set(map[string]any{"app": map[string]any{"version": "v2"}})
	result, err := coordinator.Reload(t.Context())
	var cleanupErr *CommittedCleanupError
	if !errors.As(err, &cleanupErr) {
		t.Fatalf("Reload() error = %v, want CommittedCleanupError", err)
	}
	if !result.Applied || result.CurrentGeneration != 2 {
		t.Fatalf("Reload() result = %+v", result)
	}
	diagnostics := coordinator.Diagnostics()
	if diagnostics.State != LifecycleDegraded || diagnostics.Ready || !diagnostics.CleanupRequired || diagnostics.RetiringGeneration != 1 ||
		diagnostics.RetiringAddress == "" || diagnostics.LastFailurePhase != "retire" || diagnostics.LastFailureOwner != "application-generation" {
		t.Fatalf("Diagnostics() = %+v", diagnostics)
	}
	if _, err := coordinator.Reload(t.Context()); err == nil {
		t.Fatal("second Reload() error = nil, want cleanup-debt rejection")
	}
	if factory.preparedCount() != 2 {
		t.Fatalf("prepared generations = %d, want 2", factory.preparedCount())
	}
}

func TestGenerationCoordinatorForceStopFinishesFailedShutdown(t *testing.T) {
	coordinator, _, factory := newTestGenerationCoordinator(t)
	if err := coordinator.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	generation := factory.generation(0)
	generation.stopErr = errors.New("graceful shutdown timed out")
	if err := coordinator.Stop(t.Context()); err == nil {
		t.Fatal("Stop() error = nil, want graceful shutdown failure")
	}
	if err := coordinator.ForceStop(t.Context()); err != nil {
		t.Fatalf("ForceStop() error = %v", err)
	}
	if generation.stopCalls != 1 || generation.forceStopCalls != 1 || factory.stopCalls != 1 {
		t.Fatalf("shutdown calls: stop=%d force=%d factory=%d", generation.stopCalls, generation.forceStopCalls, factory.stopCalls)
	}
	if diagnostics := coordinator.Diagnostics(); diagnostics.State != LifecycleStopped || diagnostics.CleanupRequired {
		t.Fatalf("Diagnostics() = %+v", diagnostics)
	}
}

func newTestGenerationCoordinator(t *testing.T) (*GenerationCoordinator, *mutableSource, *testGenerationFactory) {
	t.Helper()
	return newTestGenerationCoordinatorWithLogging(t, newTestLoggingManager(t))
}

func newTestGenerationCoordinatorWithLogging(t *testing.T, logging *kernellogging.Manager) (*GenerationCoordinator, *mutableSource, *testGenerationFactory) {
	t.Helper()
	source := &mutableSource{values: map[string]any{"app": map[string]any{"version": "v1"}}}
	factory := &testGenerationFactory{}
	coordinator, err := NewGenerationCoordinator(
		config.New(source),
		[]config.Binding{{
			CapabilityID: "app",
			ConfigPath:   "app",
			Validate: func(snapshot config.Snapshot) error {
				if _, ok := snapshot.Value("app.version"); !ok {
					return fmt.Errorf("app.version is required")
				}
				return nil
			},
		}},
		factory,
		Options{Logging: logging, ReloadTimeout: time.Second, Debounce: time.Millisecond},
	)
	if err != nil {
		t.Fatalf("NewGenerationCoordinator() error = %v", err)
	}
	return coordinator, source, factory
}

var _ PreparedGeneration = (*testGeneration)(nil)
var _ ActiveGeneration = (*testGeneration)(nil)
