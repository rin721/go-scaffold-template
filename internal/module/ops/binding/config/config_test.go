package configbinding

import (
	"context"
	"testing"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
)

func TestDefaultsAreLoopbackAndTracingDisabled(t *testing.T) {
	bindings := Bindings()
	loader := config.New(config.MapSource("test", map[string]any{}))
	snapshot, err := loader.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range bindings {
		if _, _, err := binding.Contract.Defaults(t.Context()); err != nil {
			t.Fatalf("Defaults() error = %v", err)
		}
	}
	resolved, err := Decode(snapshot)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if resolved.Management.Addr != "127.0.0.1:9090" || resolved.Observability.Tracing.Enabled {
		t.Fatalf("defaults = %#v", resolved)
	}
}

func TestTracingRejectsInsecureNonLoopbackEndpoint(t *testing.T) {
	values := map[string]any{"observability": map[string]any{"tracing": map[string]any{"enabled": true, "endpoint": "http://192.0.2.10:4318", "insecure": true}}}
	snapshot, err := config.New(config.MapSource("test", values)).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(snapshot); err == nil {
		t.Fatal("Decode() error = nil")
	}
}
