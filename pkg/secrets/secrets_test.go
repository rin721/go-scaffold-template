package secrets

import "testing"

func TestSecretStringIsRedacted(t *testing.T) {
	secret := New("plain-value")
	if secret.Value() != "plain-value" {
		t.Fatal("Value() did not return original secret")
	}
	if secret.String() == "plain-value" {
		t.Fatal("String() leaked secret")
	}
}

func TestRedactMapRedactsNestedSensitiveKeys(t *testing.T) {
	redacted := RedactMap(map[string]any{
		"database": map[string]any{
			"password": "secret",
			"host":     "localhost",
		},
	})
	nested := redacted["database"].(map[string]any)
	if nested["password"] != redactedValue {
		t.Fatalf("password = %v, want redacted", nested["password"])
	}
	if nested["host"] != "localhost" {
		t.Fatalf("host = %v, want original", nested["host"])
	}
}

func TestRedactMapTreatsDSNAsSensitive(t *testing.T) {
	redacted := RedactMap(map[string]any{"dsn": "postgres://user:secret@example.invalid/app"})
	if redacted["dsn"] == "postgres://user:secret@example.invalid/app" {
		t.Fatal("RedactMap() leaked dsn")
	}
}

func TestTokenGeneratesDifferentValues(t *testing.T) {
	first, err := Token(16)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	second, err := Token(16)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if first.Value() == "" || first.Value() == second.Value() {
		t.Fatalf("tokens are not unique enough: %q %q", first.Value(), second.Value())
	}
}
