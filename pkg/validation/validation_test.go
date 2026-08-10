package validation

import (
	"errors"
	"testing"
)

type sampleConfig struct {
	Name string `validate:"required"`
	Port int    `validate:"min=1,max=65535"`
}

func TestStructReturnsProjectFieldErrors(t *testing.T) {
	err := Struct(sampleConfig{Port: -1})
	var validationErr *Error
	if !errors.As(err, &validationErr) {
		t.Fatalf("Struct() error = %T, want *Error", err)
	}
	if len(validationErr.Fields) != 2 {
		t.Fatalf("field errors = %d, want 2", len(validationErr.Fields))
	}
	if validationErr.Fields[0].Field == "" || validationErr.Fields[0].Rule == "" {
		t.Fatalf("field error missing stable data: %#v", validationErr.Fields[0])
	}
}

func TestStructAcceptsValidValue(t *testing.T) {
	if err := Struct(sampleConfig{Name: "api", Port: 8080}); err != nil {
		t.Fatalf("Struct(valid) error = %v", err)
	}
}
