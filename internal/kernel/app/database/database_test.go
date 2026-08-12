package database

import (
	"reflect"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
)

func TestDefinitionContributesConfigWithoutExposingClose(t *testing.T) {
	definition, err := Definition()
	if err != nil {
		t.Fatalf("Definition() error = %v", err)
	}
	plan := app.NewPlan()
	added, err := app.Add(plan, definition)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if added.Output == nil {
		t.Fatal("Database Access is nil")
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	bindings := frozen.Defaults()
	if len(bindings) != 1 || bindings[0].CapabilityID != string(ID) || bindings[0].ConfigPath != ConfigPath {
		t.Fatalf("Defaults() = %#v", bindings)
	}
	clientType := reflect.TypeOf((*Client)(nil)).Elem()
	if _, exists := clientType.MethodByName("Close"); exists {
		t.Fatal("Database Client exposes shared Close ownership")
	}
}
