package jwtadapter

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/rin721/go-scaffold-template/internal/module/auth/model"
	"github.com/rin721/go-scaffold-template/pkg/clock"
)

func TestVerifierLifecycleClaimsAndKeyRotation(t *testing.T) {
	now := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	first := newTestKey(t, "key-one")
	second := newTestKey(t, "key-two")
	provider := &testJWKSProvider{}
	provider.set(t, first.public)
	server := httptest.NewServer(provider)
	defer server.Close()

	verifier, err := New(Config{
		Issuer: "https://issuer.example", Audience: "todo-api", JWKSURL: server.URL,
		Algorithms: []string{"RS256"}, ScopesClaim: "scope", RequestTimeout: time.Second,
		RefreshInterval: time.Hour, RefreshTimeout: time.Second, Leeway: 5 * time.Second,
		MaxResponseBodyBytes: 1 << 20, AllowLoopbackOrPrivate: true,
	}, clock.Fixed(now))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	credential := signCredential(t, first.private, now, tokenOptions{})
	if _, err := verifier.Verify(t.Context(), credential); err == nil {
		t.Fatal("Verify(before Start) error = nil")
	}
	if err := verifier.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	principal, err := verifier.Verify(t.Context(), credential)
	if err != nil || principal.Subject != "actor-a" || !principal.HasScope("todos:read") {
		t.Fatalf("Verify(valid) = %#v, %v", principal, err)
	}

	invalid := []struct {
		name    string
		options tokenOptions
	}{
		{name: "issuer", options: tokenOptions{issuer: "https://wrong.example"}},
		{name: "audience", options: tokenOptions{audience: "wrong-api"}},
		{name: "expired", options: tokenOptions{expiration: now.Add(-time.Minute)}},
		{name: "missing nbf", options: tokenOptions{omitNotBefore: true}},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if _, err := verifier.Verify(t.Context(), signCredential(t, first.private, now, test.options)); err == nil {
				t.Fatal("Verify(invalid claims) error = nil")
			}
		})
	}

	provider.set(t, second.public)
	rotated := signCredential(t, second.private, now, tokenOptions{})
	principal, err = verifier.Verify(t.Context(), rotated)
	if err != nil || principal.Subject != "actor-a" || provider.requests() < 2 {
		t.Fatalf("Verify(rotated) = %#v, %v, requests = %d", principal, err, provider.requests())
	}
	if err := verifier.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if verifier.Ready() {
		t.Fatal("Ready() = true after Stop")
	}
	if _, err := verifier.Verify(t.Context(), rotated); err == nil {
		t.Fatal("Verify(after Stop) error = nil")
	}
}

func TestVerifierRejectsDisallowedAlgorithmAndFailedInitialFetch(t *testing.T) {
	now := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	verifier, err := New(Config{
		Issuer: "https://issuer.example", Audience: "todo-api", JWKSURL: server.URL,
		Algorithms: []string{"RS256"}, ScopesClaim: "scope", RequestTimeout: time.Second,
		RefreshInterval: time.Hour, RefreshTimeout: time.Second, Leeway: 5 * time.Second,
		MaxResponseBodyBytes: 1024, AllowLoopbackOrPrivate: true,
	}, clock.Fixed(now))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := verifier.Start(t.Context()); err == nil || verifier.Ready() {
		t.Fatalf("Start(failed fetch) error = %v, ready = %t", err, verifier.Ready())
	}

	key := []byte("01234567890123456789012345678901")
	token, err := jwt.NewBuilder().Issuer("https://issuer.example").Audience([]string{"todo-api"}).
		Subject("actor-a").IssuedAt(now).NotBefore(now.Add(-time.Minute)).Expiration(now.Add(time.Minute)).
		Claim("scope", "todos:read").Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	headers := jws.NewHeaders()
	if err := headers.Set(jws.KeyIDKey, "symmetric"); err != nil {
		t.Fatalf("Set(kid) error = %v", err)
	}
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.HS256(), key, jws.WithProtectedHeaders(headers)))
	if err != nil {
		t.Fatalf("Sign(HS256) error = %v", err)
	}
	if _, _, err := verifier.protectedHeader(signed); err == nil {
		t.Fatal("protectedHeader(HS256) error = nil")
	}
}

type testKey struct {
	private jwk.Key
	public  jwk.Key
}

func newTestKey(t *testing.T, kid string) testKey {
	t.Helper()
	raw, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	private, err := jwk.Import(raw)
	if err != nil {
		t.Fatalf("Import(private) error = %v", err)
	}
	if err := private.Set(jwk.KeyIDKey, kid); err != nil {
		t.Fatalf("Set(private kid) error = %v", err)
	}
	if err := private.Set(jwk.AlgorithmKey, jwa.RS256()); err != nil {
		t.Fatalf("Set(private alg) error = %v", err)
	}
	public, err := jwk.PublicKeyOf(private)
	if err != nil {
		t.Fatalf("PublicKeyOf() error = %v", err)
	}
	return testKey{private: private, public: public}
}

type tokenOptions struct {
	issuer        string
	audience      string
	expiration    time.Time
	omitNotBefore bool
}

func signCredential(t *testing.T, key jwk.Key, now time.Time, options tokenOptions) model.Credential {
	t.Helper()
	issuer := options.issuer
	if issuer == "" {
		issuer = "https://issuer.example"
	}
	audience := options.audience
	if audience == "" {
		audience = "todo-api"
	}
	expiration := options.expiration
	if expiration.IsZero() {
		expiration = now.Add(time.Hour)
	}
	builder := jwt.NewBuilder().Issuer(issuer).Audience([]string{audience}).Subject("actor-a").
		IssuedAt(now.Add(-time.Minute)).Expiration(expiration).Claim("scope", "todos:read todos:write")
	if !options.omitNotBefore {
		builder.NotBefore(now.Add(-time.Minute))
	}
	token, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	headers := jws.NewHeaders()
	kid, _ := key.KeyID()
	if err := headers.Set(jws.KeyIDKey, kid); err != nil {
		t.Fatalf("Set(kid) error = %v", err)
	}
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256(), key, jws.WithProtectedHeaders(headers)))
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	return model.Credential{Scheme: "Bearer", Value: string(signed)}
}

type testJWKSProvider struct {
	mu       sync.RWMutex
	payload  []byte
	requestN int
}

func (p *testJWKSProvider) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	p.requestN++
	payload := append([]byte(nil), p.payload...)
	p.mu.Unlock()
	writer.Header().Set("Content-Type", "application/jwk-set+json")
	_, _ = writer.Write(payload)
}

func (p *testJWKSProvider) set(t *testing.T, key jwk.Key) {
	t.Helper()
	set := jwk.NewSet()
	if err := set.AddKey(key); err != nil {
		t.Fatalf("AddKey() error = %v", err)
	}
	payload, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("Marshal(JWKS) error = %v", err)
	}
	p.mu.Lock()
	p.payload = payload
	p.mu.Unlock()
}

func (p *testJWKSProvider) requests() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.requestN
}
