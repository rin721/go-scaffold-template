package validation

import (
	"testing"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	pkgvalidation "github.com/rin721/go-scaffold-template/pkg/validation"
)

func TestDefaultProvidesDirectValidator(t *testing.T) {
	plan := app.NewPlan()
	added, err := app.Add(plan, Default())
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	var output pkgvalidation.Validator = added.Output
	if err := output.Struct(struct {
		Name string `validate:"required"`
	}{}); err == nil {
		t.Fatal("Validator.Struct() error = nil")
	}
}
