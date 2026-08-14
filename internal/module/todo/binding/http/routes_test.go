package httpbinding

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rin721/go-scaffold2/internal/module/todo/handler"
	todomiddleware "github.com/rin721/go-scaffold2/internal/module/todo/middleware"
	"github.com/rin721/go-scaffold2/pkg/httpx"
)

func TestRoutesExposeTodoContractInStableOrder(t *testing.T) {
	t.Parallel()

	routes, err := Routes(new(handler.Handler))
	if err != nil {
		t.Fatalf("Routes() error = %v", err)
	}
	want := []struct {
		method httpx.Method
		path   string
	}{
		{httpx.MethodPost, "/api/v1/todos"},
		{httpx.MethodGet, "/api/v1/todos/{id}"},
		{httpx.MethodGet, "/api/v1/todos"},
		{httpx.MethodPatch, "/api/v1/todos/{id}/complete"},
	}
	if len(routes) != len(want) {
		t.Fatalf("len(Routes()) = %d, want %d", len(routes), len(want))
	}
	for index, expected := range want {
		route := routes[index]
		if route.Method != expected.method || route.Path != expected.path || route.Handler == nil {
			t.Fatalf("route[%d] = %#v, want %s %s with handler", index, route, expected.method, expected.path)
		}
		wantMiddlewares := 0
		if index == 0 {
			wantMiddlewares = 1
		}
		if len(route.Middlewares) != wantMiddlewares {
			t.Fatalf("route[%d] middleware count = %d, want %d", index, len(route.Middlewares), wantMiddlewares)
		}
	}

	calls := 0
	wrapped := routes[0].Middlewares[0](func(*httpx.Context) error {
		calls++
		return nil
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/todos", nil)
	err = wrapped(&httpx.Context{ResponseWriter: httptest.NewRecorder(), Request: request})
	var statusErr *httpx.StatusError
	if calls != 0 || !errors.As(err, &statusErr) ||
		statusErr.StatusCode != http.StatusUnsupportedMediaType ||
		statusErr.Code != todomiddleware.ReasonUnsupportedMediaType {
		t.Fatalf("create route middleware calls = %d, error = %#v", calls, err)
	}
}

func TestRoutesRejectNilHandler(t *testing.T) {
	t.Parallel()

	if routes, err := Routes(nil); err == nil || routes != nil {
		t.Fatalf("Routes(nil) = %#v, %v", routes, err)
	}
}
