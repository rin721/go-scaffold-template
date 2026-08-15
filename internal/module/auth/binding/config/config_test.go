package configbinding

import (
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
)

func TestDecodeAcceptsLoopbackDevelopmentAndStrictJWT(t *testing.T) {
	development := snapshot(t, map[string]any{
		"logger": map[string]any{"environment": "development"},
		"http":   map[string]any{"addr": "127.0.0.1:8080"},
		"auth": map[string]any{
			"mode": "development-anonymous", "anonymousSubject": "developer",
			"anonymousScopes": []any{"todos:read"},
		},
	})
	decoded, err := Decode(development)
	if err != nil || decoded.Mode != ModeDevelopmentAnonymous {
		t.Fatalf("Decode(development) = %#v, %v", decoded, err)
	}

	production := snapshot(t, map[string]any{
		"logger": map[string]any{"environment": "production"},
		"http":   map[string]any{"addr": ":8080"},
		"auth": map[string]any{
			"mode": "jwt",
			"jwt": map[string]any{
				"issuer": "https://issuer.example", "audience": "todo-api",
				"jwksURL":    "https://issuer.example/.well-known/jwks.json",
				"algorithms": []any{"RS256"}, "scopesClaim": "scope",
				"requestTimeout": "3s", "refreshInterval": "15m", "refreshTimeout": "3s",
				"leeway": "30s", "maxResponseBodyBytes": 1048576,
			},
		},
	})
	decoded, err = Decode(production)
	if err != nil || decoded.Mode != ModeJWT || decoded.JWT.RequestTimeout != 3*time.Second {
		t.Fatalf("Decode(production) = %#v, %v", decoded, err)
	}
}

func TestDecodeRejectsUnsafeAnonymousAndJWKSProfiles(t *testing.T) {
	tests := []map[string]any{
		{
			"logger": map[string]any{"environment": "production"}, "http": map[string]any{"addr": ":8080"},
			"auth": map[string]any{"mode": "development-anonymous", "anonymousSubject": "developer", "anonymousScopes": []any{"todos:read"}},
		},
		{
			"logger": map[string]any{"environment": "development"}, "http": map[string]any{"addr": ":8080"},
			"auth": map[string]any{"mode": "development-anonymous", "anonymousSubject": "developer", "anonymousScopes": []any{"todos:read"}},
		},
		{
			"logger": map[string]any{"environment": "production"}, "http": map[string]any{"addr": ":8080"},
			"auth": map[string]any{"mode": "jwt", "jwt": map[string]any{
				"issuer": "issuer", "audience": "api", "jwksURL": "http://127.0.0.1:9000/jwks",
				"algorithms": []any{"none"}, "scopesClaim": "scope", "requestTimeout": "3s",
				"refreshInterval": "15m", "refreshTimeout": "3s", "leeway": "30s", "maxResponseBodyBytes": 1024,
			}},
		},
	}
	for _, values := range tests {
		if _, err := Decode(snapshot(t, values)); err == nil {
			t.Fatalf("Decode(%#v) error = nil", values)
		}
	}
}

func snapshot(t *testing.T, values map[string]any) config.Snapshot {
	t.Helper()
	loaded, err := config.New(config.MapSource("auth-test", values)).Load(t.Context())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return loaded
}
