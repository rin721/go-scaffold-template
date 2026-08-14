package i18n

import (
	"context"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	pkgi18n "github.com/rin721/go-scaffold2/pkg/i18n"
)

func TestDefinitionContributesConfiguredTranslator(t *testing.T) {
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
		t.Fatal("Translator output is nil")
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	bindings := frozen.Configurations()
	if len(bindings) != 1 || bindings[0].CapabilityID != string(ID) || bindings[0].ConfigPath != ConfigPath {
		t.Fatalf("Configurations() = %#v", bindings)
	}
	components := frozen.Components()
	if len(components) != 1 || components[0].Policy() != app.KernelInstanceSwap {
		t.Fatalf("Components() = %#v", components)
	}
}

func TestTranslatorFacadeUsesCurrentLeaseInstance(t *testing.T) {
	useIDConfig := pkgi18n.DefaultConfig()
	useIDConfig.MissingBehavior = pkgi18n.MissingBehaviorUseID
	useID, err := pkgi18n.New(&useIDConfig)
	if err != nil {
		t.Fatalf("New(use-id) error = %v", err)
	}
	errorTranslator, err := pkgi18n.New(nil)
	if err != nil {
		t.Fatalf("New(error) error = %v", err)
	}
	lease := &fakeTranslatorLease{current: useID}
	facade, err := newTranslator(lease)
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}
	text, err := facade.Translate("zh-CN", pkgi18n.Text("missing"))
	if err != nil || text != "missing" {
		t.Fatalf("Translate(use-id) text=%q error=%v", text, err)
	}
	lease.current = errorTranslator
	if _, err := facade.Translate("zh-CN", pkgi18n.Text("missing")); err == nil {
		t.Fatal("Translate(error) error = nil")
	}
}

type fakeTranslatorLease struct{ current pkgi18n.Translator }

func (f *fakeTranslatorLease) Use(ctx context.Context, use func(pkgi18n.Translator) error) error {
	if ctx == nil {
		return app.ErrNilContext
	}
	return use(f.current)
}
