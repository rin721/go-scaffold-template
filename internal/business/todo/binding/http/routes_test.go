package httpbinding

import (
	"testing"

	"github.com/rin721/go-scaffold2/internal/business/todo/handler"
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
	}
}

func TestRoutesRejectNilHandler(t *testing.T) {
	t.Parallel()

	if routes, err := Routes(nil); err == nil || routes != nil {
		t.Fatalf("Routes(nil) = %#v, %v", routes, err)
	}
}
