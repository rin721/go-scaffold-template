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

	authmodel "github.com/rin721/go-scaffold-template/internal/module/auth/model"
	"github.com/rin721/go-scaffold-template/internal/module/todo/model"
	"github.com/rin721/go-scaffold-template/internal/module/todo/service"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"github.com/rin721/go-scaffold-template/pkg/i18n"
)

func TestTodoHTTPContractUsesGeneratedRoutesAndOperationIdentity(t *testing.T) {
	useCases := &stubUseCases{}
	handler := newTodoContractHandler(t, useCases)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/todos", strings.NewReader(`{"title":"学习 OpenAPI"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated || recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("create response = status %d headers %#v body %s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
	if useCases.operationID != "createTodo" {
		t.Fatalf("operation id = %q", useCases.operationID)
	}
	var response struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if response.ID != "00000000-0000-0000-0000-000000000001" || response.Status != "pending" {
		t.Fatalf("create response = %#v", response)
	}
}

func TestTodoHTTPContractRejectsInvalidRequestsAsProblem(t *testing.T) {
	handler := newTodoContractHandler(t, &stubUseCases{})
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
			handler.ServeHTTP(recorder, request)
			problem := decodeProblem(t, recorder)
			if recorder.Code != test.status || problem.Status != test.status || problem.Code != test.code ||
				recorder.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("response = status %d problem %#v headers %#v", recorder.Code, problem, recorder.Header())
			}
		})
	}
}

func TestTodoHTTPContractRedactsUnexpectedUseCaseError(t *testing.T) {
	handler := newTodoContractHandler(t, &stubUseCases{err: errors.New("dsn=password private SQL")})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/todos", nil))
	problem := decodeProblem(t, recorder)
	if recorder.Code != http.StatusInternalServerError || problem.Code != "internal_server_error" || problem.Detail != "" {
		t.Fatalf("problem = %#v", problem)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("password")) || bytes.Contains(recorder.Body.Bytes(), []byte("SQL")) {
		t.Fatalf("private error leaked: %s", recorder.Body.String())
	}
}

func newTodoContractHandler(t *testing.T, useCases service.UseCases) http.Handler {
	t.Helper()
	translator, err := i18n.New(nil)
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	handler, err := NewTodoHTTPHandler(useCases, translator, allowOperationAuthorizer{})
	if err != nil {
		t.Fatalf("NewTodoHTTPHandler() error = %v", err)
	}
	now := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	principal, err := authmodel.NewPrincipal(
		"http-test", authmodel.ActorService, []authmodel.Scope{"todos:read", "todos:write"}, now, now,
	)
	if err != nil {
		t.Fatalf("NewPrincipal() error = %v", err)
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		handler.ServeHTTP(writer, request.WithContext(authmodel.WithPrincipal(request.Context(), principal)))
	})
}

type allowOperationAuthorizer struct{}

func (allowOperationAuthorizer) EnforceOperation(context.Context, authmodel.Principal, string) error {
	return nil
}

func decodeProblem(t *testing.T, recorder *httptest.ResponseRecorder) httpx.Problem {
	t.Helper()
	var problem httpx.Problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode Problem from %q: %v", recorder.Body.String(), err)
	}
	return problem
}

type stubUseCases struct {
	err         error
	operationID string
}

func (s *stubUseCases) Create(ctx context.Context, command service.CreateCommand) (model.Todo, error) {
	s.operationID, _ = httpx.OperationIDFromContext(ctx)
	if s.err != nil {
		return model.Todo{}, s.err
	}
	now := time.Date(2026, 8, 15, 9, 0, 0, 123000000, time.UTC)
	return model.Todo{
		ID: "00000000-0000-0000-0000-000000000001", Title: command.Title,
		Status: model.StatusPending, CreatedAt: now, UpdatedAt: now, Version: 1,
	}, nil
}

func (s *stubUseCases) Get(context.Context, service.GetQuery) (model.Todo, error) {
	return model.Todo{}, s.err
}

func (s *stubUseCases) List(context.Context, service.ListQuery) (service.ListResult, error) {
	if s.err != nil {
		return service.ListResult{}, s.err
	}
	return service.ListResult{Items: []model.Todo{}, Limit: 20}, nil
}

func (s *stubUseCases) Complete(context.Context, service.CompleteCommand) (model.Todo, error) {
	return model.Todo{}, s.err
}

var _ service.UseCases = (*stubUseCases)(nil)
