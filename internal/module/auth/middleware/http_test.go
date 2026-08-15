package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/module/auth/model"
)

func TestHTTPAuthenticatesBearerAndInjectsPrincipal(t *testing.T) {
	principal := testPrincipal(t)
	authenticator := &testAuthenticator{principal: principal}
	middleware, err := HTTP(authenticator)
	if err != nil {
		t.Fatalf("HTTP() error = %v", err)
	}
	handler := middleware(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		actual, ok := model.PrincipalFromContext(request.Context())
		if !ok || actual.Subject != principal.Subject {
			t.Fatalf("PrincipalFromContext() = %#v, %t", actual, ok)
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/todos", nil)
	request.Header.Set("Authorization", "Bearer opaque-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || authenticator.credential.Value != "opaque-token" {
		t.Fatalf("response = %d, credential = %#v", response.Code, authenticator.credential)
	}
}

func TestHTTPRejectsMalformedCredentialWithoutLeakingIt(t *testing.T) {
	authenticator := &testAuthenticator{principal: testPrincipal(t)}
	middleware, err := HTTP(authenticator)
	if err != nil {
		t.Fatalf("HTTP() error = %v", err)
	}
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler was called")
	}))
	request := httptest.NewRequest(http.MethodGet, "/todos", nil)
	request.Header["Authorization"] = []string{"Bearer secret-one", "Bearer secret-two"}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || authenticator.failures != 1 {
		t.Fatalf("response = %d, failures = %d", response.Code, authenticator.failures)
	}
	if body := response.Body.String(); body == "" || containsAny(body, "secret-one", "secret-two") {
		t.Fatalf("problem body leaked credential: %q", body)
	}
}

type testAuthenticator struct {
	principal  model.Principal
	credential model.Credential
	failures   int
}

func (a *testAuthenticator) Authenticate(_ context.Context, credential model.Credential) (model.Principal, error) {
	a.credential = credential
	return a.principal, nil
}

func (a *testAuthenticator) DevelopmentPrincipal(context.Context) (model.Principal, error) {
	return model.Principal{}, model.ErrUnauthenticated
}

func (a *testAuthenticator) RecordAuthenticationFailure(context.Context) error {
	a.failures++
	return nil
}

func testPrincipal(t *testing.T) model.Principal {
	t.Helper()
	now := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	principal, err := model.NewPrincipal("actor", model.ActorService, []model.Scope{"todos:read"}, now, now)
	if err != nil {
		t.Fatalf("NewPrincipal() error = %v", err)
	}
	return principal
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if len(candidate) > 0 && len(value) >= len(candidate) {
			for index := 0; index+len(candidate) <= len(value); index++ {
				if value[index:index+len(candidate)] == candidate {
					return true
				}
			}
		}
	}
	return false
}
