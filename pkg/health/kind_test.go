package health

import "testing"

func TestDegradedResult(t *testing.T) {
	result := Degraded("remote dependency slow")
	if result.Status != StatusWarn {
		t.Fatalf("Status = %q", result.Status)
	}
}
