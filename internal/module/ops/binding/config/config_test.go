package configbinding

import (
	"testing"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
)

func TestDefaultsAreLoopback(t *testing.T) {
	binding := Binding()
	loader := config.New(config.MapSource("test", map[string]any{}))
	snapshot, err := loader.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := binding.Contract.Defaults(t.Context()); err != nil {
		t.Fatalf("Defaults() error = %v", err)
	}
	resolved, err := Decode(snapshot)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if resolved.Management.Addr != "127.0.0.1:9090" {
		t.Fatalf("defaults = %#v", resolved)
	}
}
