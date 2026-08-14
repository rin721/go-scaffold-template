package idgen

import (
	"testing"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	pkgidgen "github.com/rin721/go-scaffold-template/pkg/idgen"
)

func TestUUIDProvidesDirectGenerator(t *testing.T) {
	plan := app.NewPlan()
	added, err := app.Add(plan, UUID())
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	var output pkgidgen.Generator = added.Output
	value, err := output.New()
	if err != nil || value == "" {
		t.Fatalf("Generator.New() = %q, %v", value, err)
	}
}
