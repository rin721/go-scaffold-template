package composition

import (
	"reflect"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	databaseapp "github.com/rin721/go-scaffold2/internal/kernel/app/database"
	loggerapp "github.com/rin721/go-scaffold2/internal/kernel/app/logger"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

func TestBuiltinScenarioBaselineOnlyPublishesNoLoggerDefaults(t *testing.T) {
	assembly := newTestAssembly(t, config.MapSource("empty", map[string]any{}), nil)
	plan, builtins, err := assembly.Plan()
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	databaseDefinition, err := databaseapp.Definition(app.Spec{ID: "database.db1", ConfigPath: "databases.db1"}, app.InputOf(builtins.Logging.Output.Binding()))
	if err != nil {
		t.Fatalf("Definition(database.db1) error = %v", err)
	}
	if _, err := app.Add(plan, databaseDefinition); err != nil {
		t.Fatalf("Add(database.db1) error = %v", err)
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	assertDefaultPaths(t, frozen.Defaults(), []string{"databases.db1"})
	if err := assembly.Close(t.Context()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestBuiltinScenarioMainReplacementAndIndependentDB2Coexist(t *testing.T) {
	assembly := newTestAssembly(t, config.MapSource("empty", map[string]any{}), nil)
	plan, builtins, err := assembly.Plan()
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	mainReplacement, err := loggerapp.Replacement(app.Spec{ID: "logging.main", ConfigPath: "logger"})
	if err != nil {
		t.Fatalf("Replacement(main) error = %v", err)
	}
	if err := app.Replace(plan, builtins.Logging.Role, mainReplacement); err != nil {
		t.Fatalf("Replace(main) error = %v", err)
	}
	db1Definition, err := databaseapp.Definition(app.Spec{ID: "database.db1", ConfigPath: "databases.db1"}, app.InputOf(builtins.Logging.Output.Binding()))
	if err != nil {
		t.Fatalf("Definition(db1) error = %v", err)
	}
	if _, err := app.Add(plan, db1Definition); err != nil {
		t.Fatalf("Add(db1) error = %v", err)
	}
	db2LoggerDefinition, err := loggerapp.Instance(app.Spec{ID: "logging.db2", ConfigPath: "loggers.db2"})
	if err != nil {
		t.Fatalf("Instance(db2 logger) error = %v", err)
	}
	db2Logger, err := app.Add(plan, db2LoggerDefinition)
	if err != nil {
		t.Fatalf("Add(db2 logger) error = %v", err)
	}
	db2Definition, err := databaseapp.Definition(app.Spec{ID: "database.db2", ConfigPath: "databases.db2"}, app.InputOf(db2Logger.Binding))
	if err != nil {
		t.Fatalf("Definition(db2) error = %v", err)
	}
	if _, err := app.Add(plan, db2Definition); err != nil {
		t.Fatalf("Add(db2) error = %v", err)
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	assertDefaultPaths(t, frozen.Defaults(), []string{"logger", "databases.db1", "loggers.db2", "databases.db2"})
	if err := assembly.Close(t.Context()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func assertDefaultPaths(t *testing.T, bindings []config.Binding, want []string) {
	t.Helper()
	got := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		got = append(got, binding.ConfigPath)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default paths = %#v, want %#v", got, want)
	}
}
