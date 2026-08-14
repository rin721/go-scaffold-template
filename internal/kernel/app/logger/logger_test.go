package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

func TestReplacementSwitchesTypedTargetAndRestoresBaseline(t *testing.T) {
	baseline := pkglogger.NewTestLogger()
	manager, err := kernellogging.New(baseline)
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	plan := app.NewPlan()
	targetDefinition, err := app.Value[kernellogging.Target]("kernel.logger", manager)
	if err != nil {
		t.Fatalf("Value(target) error = %v", err)
	}
	target, err := app.Add(plan, targetDefinition)
	if err != nil {
		t.Fatalf("Add(target) error = %v", err)
	}
	replacement, err := Replacement()
	if err != nil {
		t.Fatalf("Replacement() error = %v", err)
	}
	if err := app.Replace(plan, target.Binding, replacement); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	if len(frozen.Configurations()) != 1 || frozen.Configurations()[0].CapabilityID != string(ID) || frozen.Configurations()[0].ConfigPath != ConfigPath {
		t.Fatalf("Configurations() = %#v", frozen.Configurations())
	}
	logPath := filepath.Join(t.TempDir(), "configured.log")
	snapshot, err := config.New(config.MapSource("logger", map[string]any{
		"logger": map[string]any{
			"environment": "production", "level": "info", "encoding": "json",
			"outputPaths": []any{logPath}, "errorOutputPaths": []any{logPath},
		},
	})).Load(t.Context())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	component := frozen.Components()[0]
	changed, err := component.Stage(snapshot)
	if err != nil || !changed {
		t.Fatalf("Stage() = %v, %v", changed, err)
	}
	if err := component.Build(t.Context()); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	component.PublishInitial()
	target.Output.Logger().Info("configured logger active")
	drained, err := component.BeginDrain()
	if err != nil {
		t.Fatalf("BeginDrain() error = %v", err)
	}
	<-drained
	component.PrepareStop()
	if err := component.StopCurrent(t.Context()); err != nil {
		t.Fatalf("StopCurrent() error = %v", err)
	}
	target.Output.Logger().Info("baseline restored")

	payload, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(configured log) error = %v", err)
	}
	if !strings.Contains(string(payload), "configured logger active") {
		t.Fatalf("configured log = %s", payload)
	}
	entries := baseline.Entries()
	if len(entries) != 1 || entries[0].Message != "baseline restored" {
		t.Fatalf("baseline entries = %#v", entries)
	}
}

func TestReplacementCannotBeAddedAsOrdinaryDefinition(t *testing.T) {
	// Replacement() 的结果只能传给 app.Replace；app.Add 的参数类型是
	// app.Definition[T]。该约束由 Go 编译器保证，这里同时确认声明可正常建立。
	if _, err := Replacement(); err != nil {
		t.Fatalf("Replacement() error = %v", err)
	}
}

func TestReplacementReloadSwitchesOnlyAfterCommit(t *testing.T) {
	baseline := pkglogger.NewTestLogger()
	manager, err := kernellogging.New(baseline)
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	component := newReplacementComponent(t, manager)
	firstPath := filepath.Join(t.TempDir(), "first.log")
	secondPath := filepath.Join(t.TempDir(), "second.log")

	stageAndBuild(t, component, loggerSnapshot(t, firstPath))
	component.PublishInitial()
	manager.Info("first active")

	changed, err := component.Stage(loggerSnapshot(t, secondPath))
	if err != nil || !changed {
		t.Fatalf("Stage(second) = %v, %v", changed, err)
	}
	if err := component.Build(t.Context()); err != nil {
		t.Fatalf("Build(second) error = %v", err)
	}
	manager.Info("first remains before commit")
	drained, err := component.BeginDrain()
	if err != nil {
		t.Fatalf("BeginDrain() error = %v", err)
	}
	<-drained
	component.Commit()
	component.Resume()
	manager.Info("second active")
	if err := component.StopPrevious(t.Context()); err != nil {
		t.Fatalf("StopPrevious() error = %v", err)
	}
	stopReplacement(t, component)

	assertLogContains(t, firstPath, "first active", "first remains before commit")
	assertLogContains(t, secondPath, "second active")
	if strings.Contains(readLog(t, firstPath), "second active") {
		t.Fatalf("first log received committed second message: %s", readLog(t, firstPath))
	}
}

func TestReplacementBuildFailureKeepsBaseline(t *testing.T) {
	baseline := pkglogger.NewTestLogger()
	manager, err := kernellogging.New(baseline)
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	component := newReplacementComponent(t, manager)
	missingDirectoryPath := filepath.Join(t.TempDir(), "missing", "configured.log")
	changed, err := component.Stage(loggerSnapshot(t, missingDirectoryPath))
	if err != nil || !changed {
		t.Fatalf("Stage() = %v, %v", changed, err)
	}
	if err := component.Build(t.Context()); err == nil {
		t.Fatal("Build(invalid output path) error = nil")
	}
	manager.Info("baseline after candidate failure")
	entries := baseline.Entries()
	if len(entries) != 1 || entries[0].Message != "baseline after candidate failure" {
		t.Fatalf("baseline entries = %#v", entries)
	}
	component.StopPending()
}

func newReplacementComponent(t *testing.T, manager *kernellogging.Manager) app.RuntimeComponent {
	t.Helper()
	plan := app.NewPlan()
	targetDefinition, err := app.Value[kernellogging.Target]("kernel.logger", manager)
	if err != nil {
		t.Fatalf("Value(target) error = %v", err)
	}
	target, err := app.Add(plan, targetDefinition)
	if err != nil {
		t.Fatalf("Add(target) error = %v", err)
	}
	replacement, err := Replacement()
	if err != nil {
		t.Fatalf("Replacement() error = %v", err)
	}
	if err := app.Replace(plan, target.Binding, replacement); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	return frozen.Components()[0]
}

func loggerSnapshot(t *testing.T, outputPath string) config.Snapshot {
	t.Helper()
	snapshot, err := config.New(config.MapSource("logger", map[string]any{
		"logger": map[string]any{
			"environment": "production", "level": "info", "encoding": "json",
			"outputPaths": []any{outputPath}, "errorOutputPaths": []any{outputPath},
		},
	})).Load(t.Context())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return snapshot
}

func stageAndBuild(t *testing.T, component app.RuntimeComponent, snapshot config.Snapshot) {
	t.Helper()
	changed, err := component.Stage(snapshot)
	if err != nil || !changed {
		t.Fatalf("Stage() = %v, %v", changed, err)
	}
	if err := component.Build(t.Context()); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
}

func stopReplacement(t *testing.T, component app.RuntimeComponent) {
	t.Helper()
	drained, err := component.BeginDrain()
	if err != nil {
		t.Fatalf("BeginDrain(stop) error = %v", err)
	}
	<-drained
	component.PrepareStop()
	if err := component.StopCurrent(t.Context()); err != nil {
		t.Fatalf("StopCurrent() error = %v", err)
	}
}

func assertLogContains(t *testing.T, path string, messages ...string) {
	t.Helper()
	payload := readLog(t, path)
	for _, message := range messages {
		if !strings.Contains(payload, message) {
			t.Fatalf("log %s misses %q: %s", path, message, payload)
		}
	}
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(payload)
}
