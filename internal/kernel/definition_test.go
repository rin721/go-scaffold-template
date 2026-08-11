package kernel

import (
	"context"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

func TestRegisterRequiresDefaultsAndReturnsStableBindingWithoutCallingContract(t *testing.T) {
	runtime := newTestKernel(t, &mutableSource{values: versionValues("service", "v1")}, Options{})
	definition := testDefinition("service", &eventLog{}, nil, nil)
	called := false
	definition.Defaults = config.DefaultContractFunc(func(context.Context) (config.Object, config.Control, error) {
		called = true
		return config.Object{}, config.Continue, nil
	})
	registration, err := Register(runtime, definition)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if registration.Access == nil || registration.Defaults.CapabilityID != "service" || registration.Defaults.ConfigPath != "service" {
		t.Fatalf("Register() = %#v", registration)
	}
	if called {
		t.Fatal("Register() called defaults contract")
	}
}

func TestRegisterMissingDefaultsReturnsZeroRegistration(t *testing.T) {
	runtime := newTestKernel(t, &mutableSource{values: versionValues("service", "v1")}, Options{})
	definition := testDefinition("service", &eventLog{}, nil, nil)
	definition.Defaults = nil
	registration, err := Register(runtime, definition)
	if err == nil {
		t.Fatal("Register() error = nil")
	}
	if registration.Access != nil || registration.Defaults != (config.Binding{}) {
		t.Fatalf("Register() = %#v, want zero registration", registration)
	}
}
