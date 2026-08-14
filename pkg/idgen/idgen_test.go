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
	if err := Validate(first); err != nil {
		t.Fatalf("Validate(generated UUID) error = %v", err)
	}
}

func TestValidateRejectsNonCanonicalUUID(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"",
		"not-a-uuid",
		"550E8400-E29B-41D4-A716-446655440000",
		"{550e8400-e29b-41d4-a716-446655440000}",
	} {
		if err := Validate(value); err == nil {
			t.Fatalf("Validate(%q) error = nil", value)
		}
	}
}
