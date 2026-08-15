package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
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
