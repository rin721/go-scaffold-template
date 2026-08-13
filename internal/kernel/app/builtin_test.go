package app_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	loggerapp "github.com/rin721/go-scaffold2/internal/kernel/app/logger"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

type stringConfig struct {
	Value string `mapstructure:"value"`
}
type stringAccess interface {
	Use(context.Context, func(string) error) error
}

func TestRuntimeBuiltinBaselineReplacementReloadAndRestore(t *testing.T) {
	plan := app.NewPlan()
	definition, err := app.BorrowedRuntimeBuiltin(
		app.RoleID("test.logging"), app.Bootstrap, app.RequiredActivation, app.AppVisible,
		func() (string, error) { return "baseline", nil },
		app.Leased(func(lease app.Lease[string]) (stringAccess, error) { return lease, nil }),
	)
	if err != nil {
		t.Fatalf("BorrowedRuntimeBuiltin() error = %v", err)
	}
	role, root, access, err := app.RegisterBuiltin(plan, definition, true)
	if err != nil {
		t.Fatalf("RegisterBuiltin() error = %v", err)
	}
	assertStringAccess(t, access, "baseline")
	source, err := app.Configured("logger", func(snapshot config.Snapshot) (stringConfig, error) {
		var value stringConfig
		if err := snapshot.DecodeSection("logger", &value); err != nil {
			return stringConfig{}, err
		}
		if value.Value == "" {
			return stringConfig{}, errors.New("value is required")
		}
		return value, nil
	}, nil)
	if err != nil {
		t.Fatalf("Configured() error = %v", err)
	}
	stopped := []string{}
	replacement, err := app.ManagedReplacement(app.Spec{ID: "logging.main", ConfigPath: "logger"}, source, app.FixedDependencies(struct{}{}), func(_ context.Context, cfg stringConfig, _ struct{}) (string, error) { return cfg.Value, nil }, func(value string) (string, error) { return value, nil }, app.WithStop(func(_ context.Context, value string) error { stopped = append(stopped, value); return nil }))
	if err != nil {
		t.Fatalf("ManagedReplacement() error = %v", err)
	}
	if err := app.Replace(plan, role, replacement); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	if err := app.Replace(plan, role, replacement); err == nil {
		t.Fatal("duplicate Replace() error = nil")
	}
	consumerDependencies, err := app.DependencySet(func(values app.Values) (stringAccess, error) { return app.Resolve(values, app.InputOf(root.Binding())) }, app.InputOf(root.Binding()))
	if err != nil {
		t.Fatalf("DependencySet() error = %v", err)
	}
	consumer, err := app.ManagedFixed("consumer", struct{}{}, consumerDependencies, func(_ context.Context, _ struct{}, resolved stringAccess) (*stringAccess, error) {
		return &resolved, nil
	}, app.Leased(func(app.Lease[*stringAccess]) (string, error) { return "consumer", nil }))
	if err != nil {
		t.Fatalf("ManagedFixed() error = %v", err)
	}
	if _, err := app.Add(plan, consumer); err != nil {
		t.Fatalf("Add(consumer) error = %v", err)
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	component := frozen.Components()[0]
	initial := loadRoleSnapshot(t, "one")
	changed, err := component.Stage(initial)
	if err != nil || !changed {
		t.Fatalf("initial Stage() = %v, %v", changed, err)
	}
	if err := component.Build(t.Context()); err != nil {
		t.Fatalf("initial Build() error = %v", err)
	}
	component.PublishInitial()
	assertStringAccess(t, access, "one")
	candidate := loadRoleSnapshot(t, "two")
	changed, err = component.Stage(candidate)
	if err != nil || !changed {
		t.Fatalf("reload Stage() = %v, %v", changed, err)
	}
	if err := component.Build(t.Context()); err != nil {
		t.Fatalf("reload Build() error = %v", err)
	}
	assertStringAccess(t, access, "one")
	drained, err := component.BeginDrain()
	if err != nil {
		t.Fatalf("BeginDrain() error = %v", err)
	}
	<-drained
	component.Commit()
	component.Resume()
	assertStringAccess(t, access, "two")
	if err := component.StopPrevious(t.Context()); err != nil {
		t.Fatalf("StopPrevious() error = %v", err)
	}
	drained, err = component.BeginDrain()
	if err != nil {
		t.Fatalf("final BeginDrain() error = %v", err)
	}
	<-drained
	component.PrepareStop()
	assertStringAccess(t, access, "baseline")
	if err := component.StopCurrent(t.Context()); err != nil {
		t.Fatalf("StopCurrent() error = %v", err)
	}
	if !reflect.DeepEqual(stopped, []string{"one", "two"}) {
		t.Fatalf("stopped = %#v", stopped)
	}
}

func TestReplaceRejectsCrossPlanLateConsumerAndPolicyMismatch(t *testing.T) {
	plan, role, root := newStringBuiltin(t, app.RuntimeTransaction)
	otherPlan := app.NewPlan()
	replacement := newStringReplacement(t, "logging.main", "logger")
	if err := app.Replace(otherPlan, role, replacement); err == nil {
		t.Fatal("cross-plan Replace() error = nil")
	}
	input := app.InputOf(root.Binding())
	dependencies, err := app.DependencySet(func(values app.Values) (stringAccess, error) { return app.Resolve(values, input) }, input)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := app.ManagedFixed("consumer", struct{}{}, dependencies, func(context.Context, struct{}, stringAccess) (string, error) { return "ready", nil }, app.Leased(func(app.Lease[string]) (string, error) { return "output", nil }))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Add(plan, consumer); err != nil {
		t.Fatal(err)
	}
	if err := app.Replace(plan, role, replacement); err == nil {
		t.Fatal("late Replace() error = nil")
	}
	startupPlan, startupRole, _ := newStringBuiltin(t, app.StartupReplace)
	if err := app.Replace(startupPlan, startupRole, replacement); err == nil {
		t.Fatal("policy mismatch Replace() error = nil")
	}
}

func TestPlanRejectsConfigPathParentChildOverlap(t *testing.T) {
	plan, role, _ := newStringBuiltin(t, app.RuntimeTransaction)
	if err := app.Replace(plan, role, newStringReplacement(t, "logging.main", "logger")); err != nil {
		t.Fatal(err)
	}
	definition, err := loggerapp.Instance(app.Spec{ID: "logging.child", ConfigPath: "logger.output"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Add(plan, definition); err == nil {
		t.Fatal("Add(overlapping config path) error = nil")
	}
}

func TestSelectedBuiltinActivatesOnlyAfterExplicitRequest(t *testing.T) {
	count := 0
	definition, err := app.StartupBuiltin(app.RoleID("test.cli"), app.PreStart, app.SelectedActivation, app.KernelOnly, app.BorrowedBaseline, func() (string, error) { count++; return "cli", nil }, func(value string) (string, error) { return value, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := app.NewPlan()
	role, _, output, err := app.RegisterBuiltin(plan, definition, true)
	if err != nil {
		t.Fatal(err)
	}
	if output != "" || count != 0 {
		t.Fatalf("registration output/count = %q/%d", output, count)
	}
	if _, err := app.ActivateSelected(role); err == nil {
		t.Fatal("ActivateSelected(before Freeze) error = nil")
	}
	if _, err := plan.Freeze(); err != nil {
		t.Fatal(err)
	}
	activated, err := app.ActivateSelected(role)
	if err != nil {
		t.Fatal(err)
	}
	if activated != "cli" || count != 1 {
		t.Fatalf("activation output/count = %q/%d", activated, count)
	}
}

func TestFixedBuiltinRejectsReplacement(t *testing.T) {
	definition, err := app.FixedBuiltin(app.RoleID("test.fixed"), app.Bootstrap, app.RequiredActivation, app.KernelOnly, app.BorrowedBaseline, func() (string, error) { return "fixed", nil }, func(value string) (string, error) { return value, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := app.NewPlan()
	role, _, _, err := app.RegisterBuiltin(plan, definition, true)
	if err != nil {
		t.Fatal(err)
	}
	startup, err := app.StartupReplacement(app.Spec{ID: "fixed.replacement"}, "replacement", app.FixedDependencies(struct{}{}), func(context.Context, string, struct{}) (string, error) { return "replacement", nil }, func(value string) (string, error) { return value, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Replace(plan, role, startup); err == nil {
		t.Fatal("Replace(Fixed) error = nil")
	}
}

func TestCloseBuiltinStopsRootAccessWithoutClosingBorrowedBaseline(t *testing.T) {
	plan := app.NewPlan()
	builtin, err := app.BorrowedRuntimeBuiltin(app.RoleID("test.close"), app.Bootstrap, app.RequiredActivation, app.AppVisible, func() (string, error) { return "baseline", nil }, app.Leased(func(lease app.Lease[string]) (stringAccess, error) { return lease, nil }))
	if err != nil {
		t.Fatal(err)
	}
	_, _, access, err := app.RegisterBuiltin(plan, builtin, true)
	if err != nil {
		t.Fatal(err)
	}
	assertStringAccess(t, access, "baseline")
	if err := app.CloseBuiltins(t.Context(), plan); err != nil {
		t.Fatal(err)
	}
	if err := access.Use(t.Context(), func(string) error { return nil }); !errors.Is(err, app.ErrStopped) {
		t.Fatalf("Use() after CloseBuiltins = %v, want ErrStopped", err)
	}
}

func newStringBuiltin(t *testing.T, policy app.ReplacementPolicy) (*app.Plan, app.BuiltinRole[string], app.BuiltinOutput[stringAccess]) {
	t.Helper()
	plan := app.NewPlan()
	var definition app.BuiltinDefinition[string, stringAccess]
	var err error
	if policy == app.RuntimeTransaction {
		definition, err = app.BorrowedRuntimeBuiltin(app.RoleID("test.logging"), app.Bootstrap, app.RequiredActivation, app.AppVisible, func() (string, error) { return "baseline", nil }, app.Leased(func(lease app.Lease[string]) (stringAccess, error) { return lease, nil }))
	} else {
		definition, err = app.StartupBuiltin(app.RoleID("test.startup"), app.Bootstrap, app.RequiredActivation, app.AppVisible, app.BorrowedBaseline, func() (string, error) { return "baseline", nil }, func(string) (stringAccess, error) { return fixedStringAccess("baseline"), nil }, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	role, output, _, err := app.RegisterBuiltin(plan, definition, true)
	if err != nil {
		t.Fatal(err)
	}
	return plan, role, output
}

func newStringReplacement(t *testing.T, id app.ID, path string) app.ReplacementDefinition[string] {
	t.Helper()
	source, err := app.Configured(path, func(config.Snapshot) (stringConfig, error) { return stringConfig{Value: "value"}, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := app.ManagedReplacement(app.Spec{ID: id, ConfigPath: path}, source, app.FixedDependencies(struct{}{}), func(context.Context, stringConfig, struct{}) (string, error) { return "value", nil }, func(value string) (string, error) { return value, nil })
	if err != nil {
		t.Fatal(err)
	}
	return replacement
}
func loadRoleSnapshot(t *testing.T, value string) config.Snapshot {
	t.Helper()
	snapshot, err := config.New(config.MapSource("test", map[string]any{"logger": map[string]any{"value": value}})).Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
func assertStringAccess(t *testing.T, access stringAccess, want string) {
	t.Helper()
	var got string
	if err := access.Use(t.Context(), func(value string) error { got = value; return nil }); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("access value = %q, want %q", got, want)
	}
}

type fixedStringAccess string

func (a fixedStringAccess) Use(_ context.Context, use func(string) error) error {
	return use(string(a))
}
