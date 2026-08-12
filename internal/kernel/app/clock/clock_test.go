package clock

import (
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	pkgclock "github.com/rin721/go-scaffold2/pkg/clock"
)

func TestSystemProvidesDirectClockWithoutRuntimeContracts(t *testing.T) {
	plan := app.NewPlan()
	added, err := app.Add(plan, System())
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	var output pkgclock.Clock = added.Output
	if output.Now().IsZero() {
		t.Fatal("Clock.Now() returned zero time")
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	if len(frozen.Components()) != 0 || len(frozen.Defaults()) != 0 || len(frozen.CLIContracts()) != 0 {
		t.Fatalf("direct clock contributed runtime contracts")
	}
}
