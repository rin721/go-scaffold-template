package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/pkg/logger"
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

func TestRequestIDRejectsUnsafeInboundValue(t *testing.T) {
	router := NewRouter(nil)
	router.Use(RequestID(nil))
	router.Handle(MethodGet, "/ok", func(ctx *Context) error { return ctx.NoContent(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/ok", nil)
	request.Header.Set(headerRequestID, "unsafe value\r\ninjected")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	got := recorder.Header().Get(headerRequestID)
	if got == "" || got == request.Header.Get(headerRequestID) || !requestIDPattern.MatchString(got) {
		t.Fatalf("sanitized request id = %q", got)
	}
}

func TestTrustedProxyOnlyUsesForwardedAddressForTrustedPeer(t *testing.T) {
	middleware, err := TrustedProxy([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("TrustedProxy() error = %v", err)
	}
	for _, test := range []struct {
		name, remote, want string
	}{
		{name: "trusted", remote: "10.1.2.3:8443", want: "203.0.113.9"},
		{name: "untrusted", remote: "192.0.2.4:8443", want: "192.0.2.4"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = test.remote
			request.Header.Set("X-Forwarded-For", "203.0.113.9, 10.1.2.3")
			var got string
			err := middleware(func(ctx *Context) error {
				ip, ok := ClientIPFromContext(ctx.Request.Context())
				if !ok {
					t.Fatal("client IP missing")
				}
				got = ip.String()
				return nil
			})(&Context{ResponseWriter: httptest.NewRecorder(), Request: request})
			if err != nil || got != test.want {
				t.Fatalf("middleware error = %v, client IP = %q, want %q", err, got, test.want)
			}
		})
	}
}

func TestCORSDefaultsDenyAndExplicitPreflightAllows(t *testing.T) {
	tests := []struct {
		name string
		cfg  CORSConfig
		want int
	}{
		{name: "default deny", cfg: DefaultServerConfig().CORS, want: http.StatusForbidden},
		{name: "explicit allow", cfg: CORSConfig{
			AllowedOrigins: []string{"https://console.example"}, AllowedMethods: []string{http.MethodPost}, AllowedHeaders: []string{"Content-Type"},
		}, want: http.StatusNoContent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(nil)
			router.Use(CORS(test.cfg))
			router.Handle(MethodOptions, "/api/v1/todos", func(ctx *Context) error {
				return ctx.NoContent(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodOptions, "/api/v1/todos", nil)
			request.Header.Set("Origin", "https://console.example")
			request.Header.Set("Access-Control-Request-Method", http.MethodPost)
			request.Header.Set("Access-Control-Request-Headers", "Content-Type")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, test.want, recorder.Body.String())
			}
			if test.want == http.StatusNoContent && recorder.Header().Get("Access-Control-Allow-Origin") != "https://console.example" {
				t.Fatalf("allow origin = %q", recorder.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestProtocolBudgetsReturnStableProblems(t *testing.T) {
	tests := []struct {
		name string
		use  Middleware
		req  *http.Request
		want int
		code string
	}{
		{name: "body", use: BodyLimit(4), req: httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"value":"large"}`)), want: http.StatusRequestEntityTooLarge, code: "request_body_too_large"},
		{name: "accept", use: AcceptJSON(), req: requestWithHeader(http.MethodGet, "Accept", "text/html"), want: http.StatusNotAcceptable, code: "not_acceptable"},
		{name: "upgrade", use: RejectUpgrade(), req: requestWithHeader(http.MethodGet, "Upgrade", "websocket"), want: http.StatusUpgradeRequired, code: "upgrade_not_supported"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(nil)
			router.Use(test.use)
			router.Handle(MethodGet, "/", func(ctx *Context) error { return ctx.NoContent(http.StatusNoContent) })
			router.Handle(MethodPost, "/", func(ctx *Context) error {
				var body map[string]any
				return ctx.BindJSON(&body)
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, test.req)
			assertProblem(t, recorder, test.want, test.code)
		})
	}
}

func TestRequestTimeoutUsesApplicationDeadline(t *testing.T) {
	router := NewRouter(nil)
	router.Use(RequestTimeout(10 * time.Millisecond))
	router.Handle(MethodGet, "/", func(ctx *Context) error {
		<-ctx.Request.Context().Done()
		return ctx.Request.Context().Err()
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	assertProblem(t, recorder, http.StatusGatewayTimeout, "request_timeout")
}

func TestRateAndOverloadLimitsFailWithoutQueueing(t *testing.T) {
	rateRouter := NewRouter(nil)
	rateRouter.Use(NewRateLimiterWithBurst(1, 1).Middleware())
	rateRouter.Handle(MethodGet, "/", func(ctx *Context) error { return ctx.NoContent(http.StatusNoContent) })
	rateRouter.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	rateRecorder := httptest.NewRecorder()
	rateRouter.ServeHTTP(rateRecorder, httptest.NewRequest(http.MethodGet, "/", nil))
	assertProblem(t, rateRecorder, http.StatusTooManyRequests, "rate_limited")
	if rateRecorder.Header().Get("Retry-After") != "1" {
		t.Fatalf("rate Retry-After = %q", rateRecorder.Header().Get("Retry-After"))
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	overloadRouter := NewRouter(nil)
	overloadRouter.Use(NewOverloadLimiter(1).Middleware())
	overloadRouter.Handle(MethodGet, "/", func(ctx *Context) error {
		close(entered)
		<-release
		return ctx.NoContent(http.StatusNoContent)
	})
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		overloadRouter.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()
	<-entered
	overloadRecorder := httptest.NewRecorder()
	overloadRouter.ServeHTTP(overloadRecorder, httptest.NewRequest(http.MethodGet, "/", nil))
	assertProblem(t, overloadRecorder, http.StatusServiceUnavailable, "server_overloaded")
	close(release)
	wait.Wait()
}

func requestWithHeader(method, name, value string) *http.Request {
	request := httptest.NewRequest(method, "/", nil)
	request.Header.Set(name, value)
	return request
}

func assertProblem(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var problem Problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode Problem: %v; body = %s", err, recorder.Body.String())
	}
	if recorder.Code != status || problem.Status != status || problem.Code != code || recorder.Header().Get("Content-Type") != problemContentType {
		t.Fatalf("response = status %d problem %#v headers %#v", recorder.Code, problem, recorder.Header())
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

func TestRecoveryDoesNotLogPanicValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panic.log")
	addCaller := false
	resource, err := logger.New(&logger.Config{
		Environment: logger.EnvironmentProduction,
		OutputPaths: []string{path}, ErrorOutputPaths: []string{path}, AddCaller: &addCaller,
	})
	if err != nil {
		t.Fatalf("logger.New() error = %v", err)
	}
	sensitiveDetail := "UNSAFE_PANIC_DETAIL_SENTINEL"
	handler := Recovery(resource)(func(*Context) error { panic(sensitiveDetail) })
	_ = handler(&Context{ResponseWriter: httptest.NewRecorder(), Request: httptest.NewRequest(http.MethodGet, "/", nil)})
	if err := resource.Close(); err != nil {
		t.Fatalf("logger.Close() error = %v", err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if bytes.Contains(payload, []byte(sensitiveDetail)) || !bytes.Contains(payload, []byte("panic_type")) {
		t.Fatalf("panic log is not safely classified: %s", payload)
	}
}

func TestAccessLogClassifiesRequestOutcome(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantLevel   string
		wantMessage string
	}{
		{name: "success", wantLevel: "info", wantMessage: "http request completed"},
		{name: "client rejection", err: &StatusError{StatusCode: http.StatusBadRequest, Code: "invalid_request"}, wantLevel: "warn", wantMessage: "http request rejected"},
		{name: "rate limit", err: &StatusError{StatusCode: http.StatusTooManyRequests, Code: "rate_limited"}, wantLevel: "warn", wantMessage: "http request rejected"},
		{name: "overload", err: &StatusError{StatusCode: http.StatusServiceUnavailable, Code: "server_overloaded"}, wantLevel: "warn", wantMessage: "http request rejected"},
		{name: "server failure", err: &StatusError{StatusCode: http.StatusInternalServerError, Code: "internal_error"}, wantLevel: "error", wantMessage: "http request failed"},
		{name: "unknown failure", err: errors.New("unknown"), wantLevel: "error", wantMessage: "http request failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := logger.NewTestLogger()
			request := httptest.NewRequest(http.MethodGet, "/private?detail=UNSAFE_QUERY_DETAIL_SENTINEL", nil)
			request = request.WithContext(WithOperationID(request.Context(), "testOperation"))
			request = request.WithContext(context.WithValue(request.Context(), requestIDContextKey{}, "request-123"))
			request = request.WithContext(WithTraceID(request.Context(), "trace-123"))
			err := AccessLog(log)(func(*Context) error { return test.err })(
				&Context{ResponseWriter: httptest.NewRecorder(), Request: request},
			)
			if !errors.Is(err, test.err) {
				t.Fatalf("AccessLog() error = %v, want %v", err, test.err)
			}
			entries := log.Entries()
			if len(entries) != 1 || entries[0].Level != test.wantLevel || entries[0].Message != test.wantMessage {
				t.Fatalf("entries = %#v", entries)
			}
		})
	}
}

func TestAccessLogUsesCommittedProblemOutcome(t *testing.T) {
	log := logger.NewTestLogger()
	router := NewRouter(nil)
	router.Use(AccessLog(log))
	router.Handle(MethodGet, "/failure", func(*Context) error {
		return &StatusError{StatusCode: http.StatusInternalServerError, Code: "internal_failure"}
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/missing", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/failure", nil))
	entries := log.Entries()
	if len(entries) != 2 || entries[0].Level != "warn" || entries[0].Message != "http request rejected" ||
		entries[1].Level != "error" || entries[1].Message != "http request failed" {
		t.Fatalf("entries = %#v", entries)
	}
}
