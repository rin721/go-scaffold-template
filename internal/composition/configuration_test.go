package composition

import "testing"

func TestApplicationOwnedConfigurationBindingsAreCompleteAndOrdered(t *testing.T) {
	want := []struct {
		capabilityID string
		configPath   string
	}{
		{capabilityID: "module.auth", configPath: "auth"},
		{capabilityID: "module.migration", configPath: "migration"},
		{capabilityID: "module.todo", configPath: "todo"},
		{capabilityID: "module.ops.management", configPath: "management"},
		{capabilityID: "observability.telemetry", configPath: "observability"},
	}

	bindings := applicationOwnedConfigurationBindings()
	if len(bindings) != len(want) {
		t.Fatalf("bindings = %#v", bindings)
	}
	for index, expected := range want {
		if bindings[index].CapabilityID != expected.capabilityID || bindings[index].ConfigPath != expected.configPath {
			t.Fatalf("binding %d = %#v, want %#v", index, bindings[index], expected)
		}
	}
}
