package observability

import (
	"context"
	"testing"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
)

func TestDefaultConfigDisablesTracing(t *testing.T) {
	snapshot, err := config.New(config.MapSource("test", map[string]any{})).Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := decode(snapshot)
	if err != nil {
		t.Fatalf("decode() error = %v", err)
	}
	if resolved.ServiceName != "go-scaffold-template" || resolved.Tracing.Enabled {
		t.Fatalf("defaults = %#v", resolved)
	}
}

func TestConfigRejectsInsecureNonLoopbackEndpoint(t *testing.T) {
	values := map[string]any{"observability": map[string]any{"tracing": map[string]any{"enabled": true, "endpoint": "http://192.0.2.10:4318", "insecure": true}}}
	snapshot, err := config.New(config.MapSource("test", values)).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decode(snapshot); err == nil {
		t.Fatal("decode() error = nil")
	}
}
