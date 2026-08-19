package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rin721/go-scaffold-template/internal/module/todo/binding/http"
	todohandler "github.com/rin721/go-scaffold-template/internal/module/todo/handler"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"github.com/rin721/go-scaffold-template/pkg/httpx/contract"
)

type dispatcherStub struct {
	authenticated bool
	gateErr       error
	handlerErr    error
	operationID   string
	language      string
	pathParams    map[string]string
	queryParams   map[string][]string
	handlers      map[contract.OperationID]contract.Handler
}

func newDispatcherStub() *dispatcherStub {
	return &dispatcherStub{
		authenticated: true,
		handlers: map[contract.OperationID]contract.Handler{
			"createTodo": contract.JSONBody(func(ctx context.Context, body todohandler.CreateTodoRequest) (todohandler.Todo, error) {
				return todohandler.Todo{ID: "00000000-0000-0000-0000-000000000001", Title: body.Title, Status: todohandler.StatusPending}, nil
			}, http.StatusCreated),
			"listTodos": contract.Query(func(ctx context.Context, params todohandler.ListTodosParams) (todohandler.TodoList, error) {
				return todohandler.TodoList{}, nil
			}, http.StatusOK),
			"getTodo": contract.Path("id", func(ctx context.Context, id string) (todohandler.Todo, error) { return todohandler.Todo{ID: id}, nil }, http.StatusOK),
			"completeTodo": contract.Path("id", func(ctx context.Context, id string) (todohandler.Todo, error) {
				return todohandler.Todo{ID: id, Status: todohandler.StatusCompleted}, nil
			}, http.StatusOK),
		},
	}
}

func (s *dispatcherStub) Modules() []contract.Module {
	return []contract.Module{httpbinding.ModuleContract()}
}

func (s *dispatcherStub) Operations() []contract.Operation {
	return httpbinding.ModuleContract().Operations
}

func (s *dispatcherStub) Handler(operationID contract.OperationID) (contract.Handler, bool) {
	handler, ok := s.handlers[operationID]
	return handler, ok
}

func TestRouteBindingUsesGeneratedRoutesAndRequestMetadata(t *testing.T) {
	dispatcher := newDispatcherStub()
	gate := &operationGateStub{authenticated: true}
	routes := newRouteBinding(t, dispatcher, gate)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/todos", strings.NewReader(`{"title":"学习 OpenAPI"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Language", "zh-CN")
	recorder := httptest.NewRecorder()
	routes.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated || recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("create response = status %d headers %#v body %s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
	var created todohandler.Todo
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create body: %v", err)
	}
	if created.ID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("created = %#v", created)
	}
}

func TestRouteBindingRejectsInvalidRequestsAsProblem(t *testing.T) {
	routes := newRouteBinding(t, newDispatcherStub(), &operationGateStub{authenticated: true})
	tests := []struct {
		name        string
		method      string
		path        string
		contentType string
		body        string
		status      int
		code        string
	}{
		{name: "missing content type", method: http.MethodPost, path: "/api/v1/todos", body: `{"title":"x"}`, status: http.StatusUnsupportedMediaType, code: "unsupported_media_type"},
		{name: "unknown field", method: http.MethodPost, path: "/api/v1/todos", contentType: "application/json", body: `{"title":"x","private":"secret"}`, status: http.StatusBadRequest, code: "invalid_request"},
		{name: "trailing token", method: http.MethodPost, path: "/api/v1/todos", contentType: "application/json", body: `{"title":"x"}{}`, status: http.StatusBadRequest, code: "invalid_request"},
		{name: "invalid status", method: http.MethodGet, path: "/api/v1/todos?status=unknown", status: http.StatusBadRequest, code: "invalid_request"},
		{name: "not found", method: http.MethodGet, path: "/missing", status: http.StatusNotFound, code: "route_not_found"},
		{name: "method not allowed", method: http.MethodDelete, path: "/api/v1/todos", status: http.StatusMethodNotAllowed, code: "method_not_allowed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			recorder := httptest.NewRecorder()
			routes.ServeHTTP(recorder, request)
			problem := decodeProblem(t, recorder)
			if recorder.Code != test.status || problem.Status != test.status || problem.Code != test.code ||
				recorder.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("response = status %d problem %#v headers %#v", recorder.Code, problem, recorder.Header())
			}
		})
	}
}

func TestRouteBindingEnforcesOperationGate(t *testing.T) {
	tests := []struct {
		name   string
		gate   *operationGateStub
		status int
		code   string
	}{
		{name: "unauthenticated", gate: &operationGateStub{}, status: http.StatusUnauthorized, code: "unauthenticated"},
		{name: "permission denied", gate: &operationGateStub{authenticated: true, enforceErr: ErrPermissionDenied}, status: http.StatusForbidden, code: "permission_denied"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			routes := newRouteBinding(t, newDispatcherStub(), test.gate)
			recorder := httptest.NewRecorder()
			routes.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/todos", nil))
			problem := decodeProblem(t, recorder)
			if recorder.Code != test.status || problem.Code != test.code {
				t.Fatalf("response = status %d problem %#v", recorder.Code, problem)
			}
		})
	}
}

func TestRouteBindingRedactsUnexpectedHandlerError(t *testing.T) {
	dispatcher := newDispatcherStub()
	delete(dispatcher.handlers, "listTodos")
	dispatcher.handlers["listTodos"] = contract.Query(func(ctx context.Context, params todohandler.ListTodosParams) (todohandler.TodoList, error) {
		return todohandler.TodoList{}, errors.New("dsn=password private SQL")
	}, http.StatusOK)
	routes := newRouteBinding(t, dispatcher, &operationGateStub{authenticated: true})
	recorder := httptest.NewRecorder()
	routes.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/todos", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("response = %d", recorder.Code)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("password")) || bytes.Contains(recorder.Body.Bytes(), []byte("SQL")) {
		t.Fatalf("private error leaked: %s", recorder.Body.String())
	}
}

func TestNewRouteBindingRejectsNilDependencies(t *testing.T) {
	dispatcher := newDispatcherStub()
	if _, err := NewRouteBinding(dispatcher, nil); err == nil {
		t.Fatal("NewRouteBinding(nil gate) error = nil")
	}
	if _, err := NewRouteBinding(nil, &operationGateStub{authenticated: true}); err == nil {
		t.Fatal("NewRouteBinding(nil dispatcher) error = nil")
	}
}

func newRouteBinding(t *testing.T, dispatcher Dispatcher, gate OperationGate) http.Handler {
	t.Helper()
	routes, err := NewRouteBinding(dispatcher, gate)
	if err != nil {
		t.Fatalf("NewRouteBinding() error = %v", err)
	}
	return routes
}

type operationGateStub struct {
	authenticated bool
	authErr       error
	enforceErr    error
}

func (s *operationGateStub) Authenticate(context.Context) error {
	if s.authErr != nil {
		return s.authErr
	}
	if !s.authenticated {
		return ErrUnauthenticated
	}
	return nil
}

func (s *operationGateStub) Enforce(_ context.Context, _ string) error {
	return s.enforceErr
}

func decodeProblem(t *testing.T, recorder *httptest.ResponseRecorder) httpx.Problem {
	t.Helper()
	var problem httpx.Problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode Problem from %q: %v", recorder.Body.String(), err)
	}
	return problem
}

var _ OperationGate = (*operationGateStub)(nil)
