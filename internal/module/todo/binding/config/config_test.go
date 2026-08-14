package configbinding

import (
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

func TestBindingDefaultsAndStrictDecode(t *testing.T) {
	binding := Binding()
	if binding.CapabilityID != CapabilityID || binding.ConfigPath != ConfigPath {
		t.Fatalf("Binding() identity = %q/%q", binding.CapabilityID, binding.ConfigPath)
	}
	snapshot, err := config.New(config.MapSource("todo", map[string]any{
		"todo": map[string]any{"titleMaxRunes": 80, "defaultListLimit": 5, "maxListLimit": 25},
	})).Load(t.Context())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	decoded, err := Decode(snapshot)
	if err != nil || decoded.TitleMaxRunes != 80 || decoded.DefaultListLimit != 5 || decoded.MaxListLimit != 25 {
		t.Fatalf("Decode() = %#v, %v", decoded, err)
	}
	if err := config.ValidateCandidate(snapshot, binding); err != nil {
		t.Fatalf("ValidateCandidate() error = %v", err)
	}

	defaults, control, err := defaults{}.Defaults(t.Context())
	if err != nil || control != config.Continue || len(defaults) != 3 {
		t.Fatalf("Defaults() = %#v, %v, %v", defaults, control, err)
	}
}

func TestBindingRejectsUnknownTypeAndSemanticValues(t *testing.T) {
	values := []map[string]any{
		{"titleMaxRunes": 120, "defaultListLimit": 20, "maxListLimit": 100, "unknown": true},
		{"titleMaxRunes": false, "defaultListLimit": 20, "maxListLimit": 100},
		{"titleMaxRunes": 201, "defaultListLimit": 20, "maxListLimit": 100},
		{"titleMaxRunes": 120, "defaultListLimit": 101, "maxListLimit": 100},
	}
	for _, value := range values {
		snapshot, err := config.New(config.MapSource("todo", map[string]any{"todo": value})).Load(t.Context())
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if _, err := Decode(snapshot); err == nil {
			t.Fatalf("Decode(%#v) error = nil", value)
		}
	}
}
