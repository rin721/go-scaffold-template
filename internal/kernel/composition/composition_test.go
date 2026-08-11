package composition

import (
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

func TestKernelNewDoesNotComposeCapabilities(t *testing.T) {
	runtime, err := kernel.New(config.New(config.MapSource("empty", map[string]any{})), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() without Compose error = %v", err)
	}
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestComposeRejectsDuplicateCapabilitySet(t *testing.T) {
	runtime, err := kernel.New(config.New(config.MapSource("empty", map[string]any{})), kernel.Options{})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	if _, err := Compose(runtime); err != nil {
		t.Fatalf("first Compose() error = %v", err)
	}
	capabilities, err := Compose(runtime)
	if err == nil {
		t.Fatal("second Compose() error = nil")
	}
	if capabilities.Database != nil {
		t.Fatal("second Compose() returned partial capabilities")
	}
	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}
