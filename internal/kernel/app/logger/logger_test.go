package logger

import (
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

func TestInstancePublishesIndependentLeasedLogger(t *testing.T) {
	spec := app.Spec{ID: "logging.db2", ConfigPath: "loggers.db2"}
	definition, err := Instance(spec)
	if err != nil {
		t.Fatalf("Instance() error = %v", err)
	}
	plan := app.NewPlan()
	added, err := app.Add(plan, definition)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	if len(frozen.Defaults()) != 1 || frozen.Defaults()[0].ConfigPath != spec.ConfigPath {
		t.Fatalf("Defaults() = %#v", frozen.Defaults())
	}
	snapshot, err := config.New(config.MapSource("logger", map[string]any{"loggers": map[string]any{"db2": map[string]any{"environment": "development", "level": "info", "outputPaths": []any{"stdout"}, "errorOutputPaths": []any{"stderr"}}}})).Load(t.Context())
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
	if err := added.Output.Use(t.Context(), func(log pkglogger.Logger) error { log.Info("independent logger active"); return nil }); err != nil {
		t.Fatalf("Use() error = %v", err)
	}
	drained, err := component.BeginDrain()
	if err != nil {
		t.Fatalf("BeginDrain() error = %v", err)
	}
	<-drained
	component.PrepareStop()
	if err := component.StopCurrent(t.Context()); err != nil {
		t.Fatalf("StopCurrent() error = %v", err)
	}
}

func TestReplacementRequiresConfiguredSpec(t *testing.T) {
	if _, err := Replacement(app.Spec{ID: "logging.main"}); err == nil {
		t.Fatal("Replacement(without config path) error = nil")
	}
}
