package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProductionMiddlewareSetsRequestIDAndSecurityHeaders(t *testing.T) {
	router := NewRouter(nil)
	router.Use(RequestID(nil), SecureHeaders())
	router.Handle(MethodGet, "/ok", func(ctx *Context) error {
		if requestID, ok := RequestIDFromContext(ctx.Request.Context()); !ok || requestID == "" {
			t.Fatal("request id missing from context")
		}
		return ctx.Text(http.StatusOK, "ok")
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if recorder.Header().Get(headerRequestID) == "" {
		t.Fatal("request id header is empty")
	}
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security header missing")
	}
}

func TestRecoveryMapsPanicToStatusError(t *testing.T) {
	handler := Recovery(nil)(func(*Context) error {
		panic("boom")
	})
	err := handler(&Context{ResponseWriter: httptest.NewRecorder(), Request: httptest.NewRequest(http.MethodGet, "/", nil)})
	statusErr, ok := asStatusError(err)
	if !ok || statusErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("err = %#v", err)
	}
}
