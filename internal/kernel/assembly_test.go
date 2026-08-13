package kernel

import (
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	builtincli "github.com/rin721/go-scaffold2/internal/kernel/builtin/cli"
	builtinconfig "github.com/rin721/go-scaffold2/internal/kernel/builtin/config"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
)

func TestAssemblyRejectsForeignFrozenPlan(t *testing.T) {
	assembly, err := NewAssembly(AssemblyOptions{Config: builtinconfig.Options{Sources: []config.Source{config.MapSource("empty", map[string]any{})}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := assembly.Plan(); err != nil {
		t.Fatal(err)
	}
	foreign, err := app.NewPlan().Freeze()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assembly.Install(foreign); err == nil {
		t.Fatal("Install(foreign plan) error = nil")
	}
	if err := assembly.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestAssemblyRequiresSelectedCLIActivationBeforeInstall(t *testing.T) {
	assembly, err := NewAssembly(AssemblyOptions{Config: builtinconfig.Options{Sources: []config.Source{config.MapSource("empty", map[string]any{})}}, CLI: &builtincli.Options{App: pkgcli.Config{Name: "test", DisableInteractiveHome: true}}})
	if err != nil {
		t.Fatal(err)
	}
	plan, _, err := assembly.Plan()
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assembly.Install(frozen); err == nil {
		t.Fatal("Install(without CLI activation) error = nil")
	}
	if _, err := assembly.ActivateCLI(); err != nil {
		t.Fatal(err)
	}
	runtime, err := assembly.Install(frozen)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
}
