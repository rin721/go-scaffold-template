package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/module/todo/model"
	"github.com/rin721/go-scaffold-template/internal/module/todo/service"
	"github.com/rin721/go-scaffold-template/pkg/fault"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"github.com/rin721/go-scaffold-template/pkg/i18n"
)

func TestHandlerServesTodoRoutes(t *testing.T) {
	now := time.Date(2026, 8, 15, 1, 2, 3, 0, time.UTC)
	useCases := &stubUseCases{todo: model.Todo{
		ID: "11111111-1111-4111-8111-111111111111", Title: "学习 Go", Status: model.StatusPending,
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}}
	handler, err := New(useCases, stubTranslator{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	router := httpx.NewRouter(nil)
	router.Handle(httpx.MethodPost, "/api/v1/todos", handler.Create)
	router.Handle(httpx.MethodGet, "/api/v1/todos/{id}", handler.Get)
	router.Handle(httpx.MethodGet, "/api/v1/todos", handler.List)
	router.Handle(httpx.MethodPatch, "/api/v1/todos/{id}/complete", handler.Complete)

	create := httptest.NewRecorder()
	router.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/v1/todos", strings.NewReader(`{"title":"学习 Go"}`)))
	if create.Code != http.StatusCreated || !strings.Contains(create.Body.String(), `"status":"pending"`) {
		t.Fatalf("create response = %d %s", create.Code, create.Body.String())
	}
	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/todos/11111111-1111-4111-8111-111111111111", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"id":"11111111-1111-4111-8111-111111111111"`) {
		t.Fatalf("get response = %d %s", get.Code, get.Body.String())
	}
	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/todos?limit=5", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list response = %d %s", list.Code, list.Body.String())
	}
	var payload listResponse
	if err := json.Unmarshal(list.Body.Bytes(), &payload); err != nil || payload.Total != 1 || payload.Limit != 5 {
		t.Fatalf("list payload = %#v, %v", payload, err)
	}
	complete := httptest.NewRecorder()
	router.ServeHTTP(complete, httptest.NewRequest(http.MethodPatch, "/api/v1/todos/11111111-1111-4111-8111-111111111111/complete", nil))
	if complete.Code != http.StatusOK {
		t.Fatalf("complete response = %d %s", complete.Code, complete.Body.String())
	}
}

func TestHandlerMapsSafeBusinessErrors(t *testing.T) {
	handler, err := New(&stubUseCases{err: fault.New(fault.CodeNotFound, "database details")}, stubTranslator{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	router := httpx.NewRouter(nil)
	router.Handle(httpx.MethodGet, "/api/v1/todos/{id}", handler.Get)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/todos/11111111-1111-4111-8111-111111111111", nil))
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), `"error":"todo_not_found"`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "database details") {
		t.Fatalf("response leaked internal error: %s", recorder.Body.String())
	}
}

func TestHandlerMapsFaultCategoriesAndInternalFallback(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		reason string
	}{
		{"invalid", fault.New(fault.CodeInvalidArgument, "details"), http.StatusBadRequest, "todo_invalid_argument"},
		{"conflict", fault.New(fault.CodeConflict, "details"), http.StatusConflict, "todo_conflict"},
		{"unavailable", fault.New(fault.CodeUnavailable, "details"), http.StatusServiceUnavailable, "todo_unavailable"},
		{"timeout", fault.New(fault.CodeTimeout, "details"), http.StatusGatewayTimeout, "todo_timeout"},
		{"canceled", fault.New(fault.CodeCanceled, "details"), http.StatusRequestTimeout, "todo_canceled"},
		{"internal", errors.New("private database details"), http.StatusInternalServerError, "internal_server_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			todoHandler, err := New(&stubUseCases{err: test.err}, stubTranslator{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			router := httpx.NewRouter(nil)
			router.Handle(httpx.MethodGet, "/api/v1/todos/{id}", todoHandler.Get)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/todos/11111111-1111-4111-8111-111111111111", nil))
			if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), `"error":"`+test.reason+`"`) {
				t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "details") {
				t.Fatalf("response leaked internal error: %s", recorder.Body.String())
			}
		})
	}
}

func TestHandlerRejectsInvalidPagination(t *testing.T) {
	t.Parallel()

	todoHandler, err := New(&stubUseCases{}, stubTranslator{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	router := httpx.NewRouter(nil)
	router.Handle(httpx.MethodGet, "/api/v1/todos", todoHandler.List)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/todos?offset=not-a-number", nil))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"error":"todo_invalid_argument"`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

type stubUseCases struct {
	todo model.Todo
	err  error
}

func (s *stubUseCases) Create(context.Context, service.CreateCommand) (model.Todo, error) {
	return s.todo, s.err
}
func (s *stubUseCases) Get(context.Context, service.GetQuery) (model.Todo, error) {
	return s.todo, s.err
}
func (s *stubUseCases) List(_ context.Context, query service.ListQuery) (service.ListResult, error) {
	return service.ListResult{Items: []model.Todo{s.todo}, Offset: query.Offset, Limit: query.Limit, Total: 1}, s.err
}
func (s *stubUseCases) Complete(context.Context, service.CompleteCommand) (model.Todo, error) {
	return s.todo, s.err
}

type stubTranslator struct{}

func (stubTranslator) Translate(_ string, message i18n.Message) (string, error) {
	return message.DefaultMessage, nil
}
func (stubTranslator) MustTranslate(language string, message i18n.Message) string {
	translated, _ := stubTranslator{}.Translate(language, message)
	return translated
}
