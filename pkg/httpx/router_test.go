package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestRouterMatchesMethodAndPathParam(t *testing.T) {
	router := NewRouter(nil)
	router.Handle(MethodGet, "/users/{id}", func(ctx *Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{
			"id":     ctx.Param("id"),
			"filter": ctx.Query("filter"),
			"trace":  ctx.Header("X-Trace-ID"),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42?filter=active", nil)
	req.Header.Set("X-Trace-ID", "trace-1")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var got map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := map[string]string{"id": "42", "filter": "active", "trace": "trace-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("response = %#v, want %#v", got, want)
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/users/42", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", recorder.Code)
	}
}

func TestRouterBindJSONAndResponseHelpers(t *testing.T) {
	router := NewRouter(nil)
	router.Handle(MethodPost, "/echo", func(ctx *Context) error {
		var payload struct {
			Name string `json:"name"`
		}
		if err := ctx.BindJSON(&payload); err != nil {
			return err
		}
		return ctx.JSON(http.StatusCreated, map[string]string{"name": payload.Name})
	})
	router.Handle(MethodGet, "/text", func(ctx *Context) error {
		return ctx.Text(http.StatusOK, "hello")
	})
	router.Handle(MethodDelete, "/items/{id}", func(ctx *Context) error {
		return ctx.NoContent(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/echo", bytes.NewBufferString(`{"name":"rin"}`)))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("json status = %d, want 201", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"name":"rin"`) {
		t.Fatalf("json body = %q, want name", recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/text", nil))
	if recorder.Body.String() != "hello" {
		t.Fatalf("text body = %q, want hello", recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/items/1", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("no content status = %d, want 204", recorder.Code)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("no content body length = %d, want 0", recorder.Body.Len())
	}
}

func TestRouterMiddlewareOrder(t *testing.T) {
	var calls []string
	router := NewRouter(nil)
	router.Use(namedMiddleware("global-1", &calls), namedMiddleware("global-2", &calls))
	router.Handle(MethodGet, "/ok", func(ctx *Context) error {
		calls = append(calls, "handler")
		return ctx.NoContent(http.StatusNoContent)
	}, namedMiddleware("route", &calls))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ok", nil))

	want := []string{
		"global-1 before",
		"global-2 before",
		"route before",
		"handler",
		"route after",
		"global-2 after",
		"global-1 after",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRouterUseHTTPMiddlewareOrder(t *testing.T) {
	var calls []string
	router := NewRouter(nil)
	router.UseHTTP(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, "http before")
			next.ServeHTTP(w, r)
			calls = append(calls, "http after")
		})
	})
	router.Use(namedMiddleware("project", &calls))
	router.Handle(MethodGet, "/ok", func(ctx *Context) error {
		calls = append(calls, "handler")
		return ctx.NoContent(http.StatusNoContent)
	}, namedMiddleware("route", &calls))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ok", nil))

	want := []string{
		"http before",
		"project before",
		"route before",
		"handler",
		"route after",
		"project after",
		"http after",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRouterDefaultErrorHandler(t *testing.T) {
	router := NewRouter(nil)
	router.Handle(MethodPost, "/bind", func(ctx *Context) error {
		var payload map[string]any
		return ctx.BindJSON(&payload)
	})
	router.Handle(MethodGet, "/status", func(ctx *Context) error {
		return &StatusError{StatusCode: http.StatusTeapot, Code: "teapot", Message: "short and stout"}
	})
	router.Handle(MethodGet, "/panicless-error", func(ctx *Context) error {
		return errors.New("boom")
	})
	router.Handle(MethodGet, "/empty-status", func(ctx *Context) error {
		return &StatusError{}
	})

	tests := []struct {
		name       string
		req        *http.Request
		wantStatus int
		wantBody   string
	}{
		{name: "bind json error", req: httptest.NewRequest(http.MethodPost, "/bind", bytes.NewBufferString(`{`)), wantStatus: http.StatusBadRequest, wantBody: errorCodeInvalidJSON},
		{name: "status error", req: httptest.NewRequest(http.MethodGet, "/status", nil), wantStatus: http.StatusTeapot, wantBody: "teapot"},
		{name: "ordinary error", req: httptest.NewRequest(http.MethodGet, "/panicless-error", nil), wantStatus: http.StatusInternalServerError, wantBody: errorCodeInternalServer},
		{name: "empty status error", req: httptest.NewRequest(http.MethodGet, "/empty-status", nil), wantStatus: http.StatusInternalServerError, wantBody: errorCodeHTTPStatus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, tt.req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestRouterCustomErrorHandler(t *testing.T) {
	router := NewRouter(&RouterConfig{
		ErrorHandler: func(ctx *Context, err error) {
			_ = ctx.Text(http.StatusBadGateway, "custom")
		},
	})
	router.Handle(MethodGet, "/err", func(ctx *Context) error {
		return errors.New("boom")
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/err", nil))
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", recorder.Code)
	}
	if recorder.Body.String() != "custom" {
		t.Fatalf("body = %q, want custom", recorder.Body.String())
	}
}

func namedMiddleware(name string, calls *[]string) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			*calls = append(*calls, name+" before")
			err := next(ctx)
			*calls = append(*calls, name+" after")
			return err
		}
	}
}
