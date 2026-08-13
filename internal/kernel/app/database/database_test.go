package database

import (
	"context"
	"reflect"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

type fixedAccess struct{ logger pkglogger.Logger }

func (a fixedAccess) Use(ctx context.Context, use func(pkglogger.Logger) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return use(a.logger)
}

func TestDefinitionContributesNamedConfigAndRequiresLoggerBinding(t *testing.T) {
	plan := app.NewPlan()
	loggerDefinition, err := app.Value[pkglogger.Access]("logger.input", fixedAccess{logger: pkglogger.Noop()})
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	loggerOutput, err := app.Add(plan, loggerDefinition)
	if err != nil {
		t.Fatalf("Add(logger) error = %v", err)
	}
	spec := app.Spec{ID: "database.db1", ConfigPath: "databases.db1"}
	definition, err := Definition(spec, app.InputOf(loggerOutput.Binding))
	if err != nil {
		t.Fatalf("Definition() error = %v", err)
	}
	added, err := app.Add(plan, definition)
	if err != nil {
		t.Fatalf("Add(database) error = %v", err)
	}
	if added.Output == nil {
		t.Fatal("Database Access is nil")
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	bindings := frozen.Defaults()
	if len(bindings) != 1 || bindings[0].CapabilityID != string(spec.ID) || bindings[0].ConfigPath != spec.ConfigPath {
		t.Fatalf("Defaults() = %#v", bindings)
	}
	clientType := reflect.TypeOf((*Client)(nil)).Elem()
	if _, exists := clientType.MethodByName("Close"); exists {
		t.Fatal("Database Client exposes shared Close ownership")
	}
}
