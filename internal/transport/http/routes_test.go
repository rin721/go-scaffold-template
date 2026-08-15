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
	"time"

	"github.com/rin721/go-scaffold-template/internal/transport/http/api"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
)

func TestRouteBindingUsesGeneratedRoutesAndRequestMetadata(t *testing.T) {
	server := &strictServerStub{}
	gate := &operationGateStub{authenticated: true}
	routes := newRouteBinding(t, server, gate)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/todos", strings.NewReader(`{"title":"学习 OpenAPI"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Language", "zh-CN")
	recorder := httptest.NewRecorder()
	routes.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated || recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("create response = status %d headers %#v body %s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
	if server.operationID != "createTodo" || server.language != "zh-CN" || gate.operation != "createTodo" {
		t.Fatalf("request metadata = server operation %q language %q gate operation %q", server.operationID, server.language, gate.operation)
	}
}

func TestRouteBindingRejectsInvalidRequestsAsProblem(t *testing.T) {
	routes := newRouteBinding(t, &strictServerStub{}, &operationGateStub{authenticated: true})
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
		{name: "head not declared", method: http.MethodHead, path: "/api/v1/todos", status: http.StatusMethodNotAllowed, code: "method_not_allowed"},
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
		{name: "dependency failure", gate: &operationGateStub{authenticated: true, enforceErr: errors.New("private auth dependency")}, status: http.StatusInternalServerError, code: "internal_server_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			routes := newRouteBinding(t, &strictServerStub{}, test.gate)
			recorder := httptest.NewRecorder()
			routes.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/todos", nil))
			problem := decodeProblem(t, recorder)
			if recorder.Code != test.status || problem.Code != test.code {
				t.Fatalf("response = status %d problem %#v", recorder.Code, problem)
			}
			if strings.Contains(recorder.Body.String(), "private auth dependency") {
				t.Fatalf("private gate error leaked: %s", recorder.Body.String())
			}
		})
	}
}

func TestRouteBindingRedactsUnexpectedHandlerError(t *testing.T) {
	routes := newRouteBinding(t, &strictServerStub{err: errors.New("dsn=password private SQL")}, &operationGateStub{authenticated: true})
	recorder := httptest.NewRecorder()
	routes.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/todos", nil))
	problem := decodeProblem(t, recorder)
	if recorder.Code != http.StatusInternalServerError || problem.Code != "internal_server_error" || problem.Detail != "" {
		t.Fatalf("problem = %#v", problem)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("password")) || bytes.Contains(recorder.Body.Bytes(), []byte("SQL")) {
		t.Fatalf("private error leaked: %s", recorder.Body.String())
	}
}

func TestNewRouteBindingRejectsNilDependencies(t *testing.T) {
	if _, err := NewRouteBinding(nil, &operationGateStub{authenticated: true}); err == nil {
		t.Fatal("NewRouteBinding(nil server) error = nil")
	}
	if _, err := NewRouteBinding(&strictServerStub{}, nil); err == nil {
		t.Fatal("NewRouteBinding(nil gate) error = nil")
	}
	var typedNil *strictServerStub
	if _, err := NewRouteBinding(typedNil, &operationGateStub{authenticated: true}); err == nil {
		t.Fatal("NewRouteBinding(typed nil server) error = nil")
	}
}

func newRouteBinding(t *testing.T, server api.StrictServerInterface, gate OperationGate) http.Handler {
	t.Helper()
	routes, err := NewRouteBinding(server, gate)
	if err != nil {
		t.Fatalf("NewRouteBinding() error = %v", err)
	}
	return routes
}

type operationGateStub struct {
	authenticated bool
	authErr       error
	enforceErr    error
	operation     string
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

func (s *operationGateStub) Enforce(_ context.Context, operation string) error {
	s.operation = operation
	return s.enforceErr
}

type strictServerStub struct {
	err         error
	operationID string
	language    string
}

func (s *strictServerStub) ListTodos(ctx context.Context, _ api.ListTodosRequestObject) (api.ListTodosResponseObject, error) {
	s.capture(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return api.ListTodos200JSONResponse{Items: []api.Todo{}, Offset: 0, Limit: 20, Total: 0}, nil
}

func (s *strictServerStub) CreateTodo(ctx context.Context, request api.CreateTodoRequestObject) (api.CreateTodoResponseObject, error) {
	s.capture(ctx)
	if s.err != nil {
		return nil, s.err
	}
	now := time.Date(2026, 8, 15, 9, 0, 0, 123000000, time.UTC)
	return api.CreateTodo201JSONResponse{
		Id: "00000000-0000-0000-0000-000000000001", Title: request.Body.Title,
		Status: api.Pending, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *strictServerStub) GetTodo(ctx context.Context, request api.GetTodoRequestObject) (api.GetTodoResponseObject, error) {
	s.capture(ctx)
	if s.err != nil {
		return nil, s.err
	}
	now := time.Date(2026, 8, 15, 9, 0, 0, 123000000, time.UTC)
	return api.GetTodo200JSONResponse{Id: request.Id, Title: "todo", Status: api.Pending, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *strictServerStub) CompleteTodo(ctx context.Context, request api.CompleteTodoRequestObject) (api.CompleteTodoResponseObject, error) {
	s.capture(ctx)
	if s.err != nil {
		return nil, s.err
	}
	now := time.Date(2026, 8, 15, 9, 0, 0, 123000000, time.UTC)
	return api.CompleteTodo200JSONResponse{Id: request.Id, Title: "todo", Status: api.Completed, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *strictServerStub) capture(ctx context.Context) {
	s.operationID, _ = httpx.OperationIDFromContext(ctx)
	s.language = httpx.RequestLanguageFromContext(ctx)
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
var _ api.StrictServerInterface = (*strictServerStub)(nil)
