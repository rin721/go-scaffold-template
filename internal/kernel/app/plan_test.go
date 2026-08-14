package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgclock "github.com/rin721/go-scaffold-template/pkg/clock"
)

type clockConsumer struct{ clock pkgclock.Clock }

type clockDependencies struct{ clock pkgclock.Clock }

type replaceableTarget interface {
	Current() string
	Replace(string)
	Restore()
}

type testTarget struct{ current string }

func (t *testTarget) Current() string      { return t.current }
func (t *testTarget) Replace(value string) { t.current = value }
func (t *testTarget) Restore()             { t.current = "baseline" }

func TestPlanUsesTypedDirectOutputAndBuildTimeInput(t *testing.T) {
	plan := app.NewPlan()
	fixedTime := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	clockDefinition, err := app.Value[pkgclock.Clock]("clock", pkgclock.Fixed(fixedTime))
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	clockAdded, err := app.Add(plan, clockDefinition)
	if err != nil {
		t.Fatalf("Add(clock) error = %v", err)
	}
	clockInput := app.InputOf(clockAdded.Binding)
	dependencies, err := app.DependencySet(func(values app.Values) (clockDependencies, error) {
		clockOutput, err := app.Resolve(values, clockInput)
		return clockDependencies{clock: clockOutput}, err
	}, clockInput)
	if err != nil {
		t.Fatalf("DependencySet() error = %v", err)
	}
	consumerDefinition, err := app.ManagedFixed(
		app.ID("clock-consumer"), struct{}{}, dependencies,
		func(_ context.Context, _ struct{}, deps clockDependencies) (*clockConsumer, error) {
			return &clockConsumer{clock: deps.clock}, nil
		},
		app.Leased(func(lease app.Lease[*clockConsumer]) (app.Lease[*clockConsumer], error) { return lease, nil }),
	)
	if err != nil {
		t.Fatalf("ManagedFixed() error = %v", err)
	}
	consumerAdded, err := app.Add(plan, consumerDefinition)
	if err != nil {
		t.Fatalf("Add(consumer) error = %v", err)
	}
	if !clockAdded.Output.Now().Equal(fixedTime) {
		t.Fatalf("direct clock Now() = %v", clockAdded.Output.Now())
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	components := frozen.Components()
	if len(components) != 1 {
		t.Fatalf("runtime component count = %d, want 1", len(components))
	}
	snapshot := loadSnapshot(t, map[string]any{})
	changed, err := components[0].Stage(snapshot)
	if err != nil || !changed {
		t.Fatalf("Stage() = %v, %v", changed, err)
	}
	if err := components[0].Build(t.Context()); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	components[0].PublishInitial()
	if err := consumerAdded.Output.Use(t.Context(), func(consumer *clockConsumer) error {
		if !consumer.clock.Now().Equal(fixedTime) {
			t.Fatalf("consumer clock Now() = %v", consumer.clock.Now())
		}
		return nil
	}); err != nil {
		t.Fatalf("Use() error = %v", err)
	}
}

func TestPlanRejectsInvalidBindingsWithoutPartialAdd(t *testing.T) {
	first := app.NewPlan()
	second := app.NewPlan()
	clockDefinition, err := app.Value[pkgclock.Clock]("clock", pkgclock.System())
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	clockAdded, err := app.Add(first, clockDefinition)
	if err != nil {
		t.Fatalf("Add(first clock) error = %v", err)
	}
	foreignInput := app.InputOf(clockAdded.Binding)
	dependencies, err := app.DependencySet(func(values app.Values) (pkgclock.Clock, error) {
		return app.Resolve(values, foreignInput)
	}, foreignInput)
	if err != nil {
		t.Fatalf("DependencySet() error = %v", err)
	}
	invalid, err := app.ManagedFixed(
		app.ID("consumer"), struct{}{}, dependencies,
		func(context.Context, struct{}, pkgclock.Clock) (*clockConsumer, error) { return &clockConsumer{}, nil },
		app.Leased(func(lease app.Lease[*clockConsumer]) (app.Lease[*clockConsumer], error) { return lease, nil }),
	)
	if err != nil {
		t.Fatalf("ManagedFixed() error = %v", err)
	}
	if _, err := app.Add(second, invalid); err == nil {
		t.Fatal("Add(cross-plan input) error = nil")
	}
	validValue, err := app.Value("consumer", "still-available")
	if err != nil {
		t.Fatalf("Value(valid) error = %v", err)
	}
	if _, err := app.Add(second, validValue); err != nil {
		t.Fatalf("Add(valid after failure) error = %v", err)
	}
	if _, err := app.Add(second, validValue); err == nil {
		t.Fatal("Add(duplicate) error = nil")
	}
	if _, err := second.Freeze(); err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	if _, err := app.Add(second, validValue); err == nil {
		t.Fatal("Add(after Freeze) error = nil")
	}
	if _, err := second.Freeze(); err == nil {
		t.Fatal("second Freeze() error = nil")
	}
}

func TestValueRejectsTypedNil(t *testing.T) {
	var output *clockConsumer
	if _, err := app.Value("nil", output); err == nil {
		t.Fatal("Value(typed nil) error = nil")
	}
}

func TestConfiguredAndManagedRejectInvalidContracts(t *testing.T) {
	if _, err := app.Configured[struct{}]("", func(config.Snapshot) (struct{}, error) { return struct{}{}, nil }, nil); err == nil {
		t.Fatal("Configured(empty path) error = nil")
	}
	source, err := app.Configured("component", func(config.Snapshot) (struct{}, error) { return struct{}{}, nil }, nil)
	if err != nil {
		t.Fatalf("Configured() error = %v", err)
	}
	_, err = app.ManagedConfigured(
		app.ID("component"), source, app.FixedDependencies(struct{}{}),
		func(context.Context, struct{}, struct{}) (*clockConsumer, error) { return &clockConsumer{}, nil },
		app.Leased(func(lease app.Lease[*clockConsumer]) (app.Lease[*clockConsumer], error) { return lease, nil }),
		app.NoReload,
	)
	if err == nil {
		t.Fatal("ManagedConfigured(NoReload) error = nil")
	}
}

func TestLeaseHonorsContextAndStops(t *testing.T) {
	plan := app.NewPlan()
	definition, err := app.ManagedFixed(
		app.ID("managed"), struct{}{}, app.FixedDependencies(struct{}{}),
		func(context.Context, struct{}, struct{}) (*clockConsumer, error) { return &clockConsumer{}, nil },
		app.Leased(func(lease app.Lease[*clockConsumer]) (app.Lease[*clockConsumer], error) { return lease, nil }),
	)
	if err != nil {
		t.Fatalf("ManagedFixed() error = %v", err)
	}
	added, err := app.Add(plan, definition)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := added.Output.Use(ctx, func(*clockConsumer) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("Use(pending canceled) error = %v", err)
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	component := frozen.Components()[0]
	component.StopPending()
	if err := added.Output.Use(t.Context(), func(*clockConsumer) error { return nil }); !errors.Is(err, app.ErrStopped) {
		t.Fatalf("Use(stopped) error = %v", err)
	}
}

func TestValuesCannotBeUsedAfterDependencyDecode(t *testing.T) {
	plan := app.NewPlan()
	clockDefinition, err := app.Value[pkgclock.Clock]("clock", pkgclock.System())
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	clockAdded, err := app.Add(plan, clockDefinition)
	if err != nil {
		t.Fatalf("Add(clock) error = %v", err)
	}
	clockInput := app.InputOf(clockAdded.Binding)
	var escaped app.Values
	dependencies, err := app.DependencySet(func(values app.Values) (pkgclock.Clock, error) {
		escaped = values
		return app.Resolve(values, clockInput)
	}, clockInput)
	if err != nil {
		t.Fatalf("DependencySet() error = %v", err)
	}
	definition, err := app.ManagedFixed(
		app.ID("consumer"), struct{}{}, dependencies,
		func(context.Context, struct{}, pkgclock.Clock) (*clockConsumer, error) { return &clockConsumer{}, nil },
		app.Leased(func(lease app.Lease[*clockConsumer]) (app.Lease[*clockConsumer], error) { return lease, nil }),
	)
	if err != nil {
		t.Fatalf("ManagedFixed() error = %v", err)
	}
	if _, err := app.Add(plan, definition); err != nil {
		t.Fatalf("Add(consumer) error = %v", err)
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	component := frozen.Components()[0]
	changed, err := component.Stage(loadSnapshot(t, map[string]any{}))
	if err != nil || !changed {
		t.Fatalf("Stage() = %v, %v", changed, err)
	}
	if err := component.Build(t.Context()); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if _, err := app.Resolve(escaped, clockInput); err == nil {
		t.Fatal("Resolve(escaped Values) error = nil")
	}
}

func TestReplaceRequiresSamePlanTargetAndIsAtomic(t *testing.T) {
	first := app.NewPlan()
	second := app.NewPlan()
	targetDefinition, err := app.Value[replaceableTarget]("target", &testTarget{current: "baseline"})
	if err != nil {
		t.Fatalf("Value(target) error = %v", err)
	}
	target, err := app.Add(first, targetDefinition)
	if err != nil {
		t.Fatalf("Add(target) error = %v", err)
	}
	replacement := newStringReplacement(t, "replacement")
	if err := app.Replace(second, target.Binding, replacement); err == nil {
		t.Fatal("Replace(cross-plan target) error = nil")
	}
	validTargetDefinition, err := app.Value[replaceableTarget]("second-target", &testTarget{current: "baseline"})
	if err != nil {
		t.Fatalf("Value(second target) error = %v", err)
	}
	validTarget, err := app.Add(second, validTargetDefinition)
	if err != nil {
		t.Fatalf("Add(second target after failure) error = %v", err)
	}
	if err := app.Replace(second, validTarget.Binding, replacement); err != nil {
		t.Fatalf("Replace(valid after failure) error = %v", err)
	}
	if err := app.Replace(second, validTarget.Binding, newStringReplacement(t, "other")); err == nil {
		t.Fatal("Replace(duplicate target) error = nil")
	}
	frozen, err := second.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	if len(frozen.Components()) != 1 || len(frozen.Configurations()) != 1 {
		t.Fatalf("FrozenPlan components/configurations = %d/%d", len(frozen.Components()), len(frozen.Configurations()))
	}
	if err := app.Replace(second, validTarget.Binding, newStringReplacement(t, "frozen")); err == nil {
		t.Fatal("Replace(after Freeze) error = nil")
	}
}

func TestReplaceRejectsZeroTargetWithoutReservingComponentID(t *testing.T) {
	plan := app.NewPlan()
	replacement := newStringReplacement(t, "replacement")
	if err := app.Replace(plan, app.Binding[replaceableTarget]{}, replacement); err == nil {
		t.Fatal("Replace(zero target) error = nil")
	}
	value, err := app.Value("replacement", "id remains available")
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	if _, err := app.Add(plan, value); err != nil {
		t.Fatalf("Add(same ID after failed Replace) error = %v", err)
	}
}

func TestReplaceRejectsDuplicateComponentIDWithoutReservingTarget(t *testing.T) {
	plan := app.NewPlan()
	existing, err := app.Value("replacement", "existing")
	if err != nil {
		t.Fatalf("Value(existing) error = %v", err)
	}
	if _, err := app.Add(plan, existing); err != nil {
		t.Fatalf("Add(existing) error = %v", err)
	}
	targetDefinition, err := app.Value[replaceableTarget]("target", &testTarget{current: "baseline"})
	if err != nil {
		t.Fatalf("Value(target) error = %v", err)
	}
	target, err := app.Add(plan, targetDefinition)
	if err != nil {
		t.Fatalf("Add(target) error = %v", err)
	}
	if err := app.Replace(plan, target.Binding, newStringReplacement(t, "replacement")); err == nil {
		t.Fatal("Replace(duplicate component ID) error = nil")
	}
	if err := app.Replace(plan, target.Binding, newStringReplacement(t, "valid-replacement")); err != nil {
		t.Fatalf("Replace(valid after duplicate ID failure) error = %v", err)
	}
}

func newStringReplacement(t *testing.T, id app.ID) app.ReplacementDefinition[replaceableTarget] {
	t.Helper()
	source, err := app.Configured("replacement", func(config.Snapshot) (string, error) {
		return "configured", nil
	}, nil)
	if err != nil {
		t.Fatalf("Configured() error = %v", err)
	}
	replacement, err := app.ManagedConfiguredReplacement(
		id,
		source,
		func(context.Context, string, replaceableTarget) (string, error) { return "configured", nil },
		func(target replaceableTarget, current string) { target.Replace(current) },
		func(target replaceableTarget, _ string) { target.Restore() },
	)
	if err != nil {
		t.Fatalf("ManagedConfiguredReplacement() error = %v", err)
	}
	return replacement
}

func loadSnapshot(t *testing.T, values map[string]any) config.Snapshot {
	t.Helper()
	snapshot, err := config.New(config.MapSource("test", values)).Load(t.Context())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return snapshot
}
