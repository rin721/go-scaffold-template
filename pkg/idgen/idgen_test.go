package idgen

import "testing"

func TestUUIDGenerator(t *testing.T) {
	generator := UUID()
	first, err := generator.New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	second, err := generator.New()
	if err != nil {
		t.Fatalf("New() second error = %v", err)
	}
	if first == "" || first == second {
		t.Fatalf("unexpected generated IDs: %q %q", first, second)
	}
}
