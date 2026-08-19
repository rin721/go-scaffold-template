package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/module/todo/model"
	"github.com/rin721/go-scaffold-template/internal/module/todo/service"
	"github.com/rin721/go-scaffold-template/pkg/fault"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"github.com/rin721/go-scaffold-template/pkg/i18n"
)

func TestHandlerMapsDTOToTodoUseCases(t *testing.T) {
	useCases := &stubUseCases{}
	handler := newHandler(t, useCases, actorAccessStub{authenticated: true})
	ctx := httpx.WithOperationID(t.Context(), "createTodo")
	created, err := handler.CreateTodo(ctx, CreateTodoRequest{Title: "学习 OpenAPI"})
	if err != nil {
		t.Fatalf("CreateTodo() error = %v", err)
	}
	if created.ID != "00000000-0000-0000-0000-000000000001" || created.Status != StatusPending {
		t.Fatalf("CreateTodo() response = %#v", created)
	}
	if useCases.operationID != "createTodo" || useCases.actor.Subject != "http-test" {
		t.Fatalf("use case metadata = operation %q actor %#v", useCases.operationID, useCases.actor)
	}
}

func TestHandlerUsesTypedRequestLanguageForProblemPresentation(t *testing.T) {
	translator := &translatorStub{}
	handler, err := NewHandler(&stubUseCases{err: fault.New(fault.CodeNotFound, "missing")}, translator, actorAccessStub{authenticated: true})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	ctx := httpx.WithRequestLanguage(t.Context(), "zh-CN")
	_, err = handler.GetTodo(ctx, "missing")
	var statusError *httpx.StatusError
	if !errors.As(err, &statusError) {
		t.Fatalf("GetTodo() error = %T %v", err, err)
	}
	if statusError.StatusCode != http.StatusNotFound || statusError.Code != "todo_not_found" || translator.language != "zh-CN" {
		t.Fatalf("problem = %#v language = %q", statusError, translator.language)
	}
}

func TestHandlerFailsClosedWithoutActor(t *testing.T) {
	handler := newHandler(t, &stubUseCases{}, actorAccessStub{})
	_, err := handler.ListTodos(t.Context(), ListTodosParams{})
	var statusError *httpx.StatusError
	if !errors.As(err, &statusError) || statusError.StatusCode != http.StatusUnauthorized || statusError.Code != "unauthenticated" {
		t.Fatalf("ListTodos() error = %T %v", err, err)
	}
}

func TestNewHandlerRejectsIncompleteDependencies(t *testing.T) {
	translator, err := i18n.New(nil)
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	for _, test := range []struct {
		name       string
		useCases   service.UseCases
		translator i18n.Translator
		actors     ActorAccess
	}{
		{name: "service", translator: translator, actors: actorAccessStub{authenticated: true}},
		{name: "translator", useCases: &stubUseCases{}, actors: actorAccessStub{authenticated: true}},
		{name: "actors", useCases: &stubUseCases{}, translator: translator},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewHandler(test.useCases, test.translator, test.actors); err == nil {
				t.Fatal("NewHandler() error = nil")
			}
		})
	}
}

func newHandler(t *testing.T, useCases service.UseCases, actors ActorAccess) *Handler {
	t.Helper()
	translator, err := i18n.New(nil)
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	handler, err := NewHandler(useCases, translator, actors)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
}

type actorAccessStub struct{ authenticated bool }

func (s actorAccessStub) Actor(context.Context) (service.Actor, bool) {
	return service.Actor{Subject: "http-test", Kind: "service", Scopes: []string{"todos:read", "todos:write"}}, s.authenticated
}

type translatorStub struct{ language string }

func (s *translatorStub) Translate(language string, message i18n.Message) (string, error) {
	s.language = language
	return message.DefaultMessage, nil
}

func (s *translatorStub) MustTranslate(language string, message i18n.Message) string {
	translated, _ := s.Translate(language, message)
	return translated
}

type stubUseCases struct {
	err         error
	operationID string
	actor       service.Actor
}

func (s *stubUseCases) Create(ctx context.Context, command service.CreateCommand) (model.Todo, error) {
	s.operationID, _ = httpx.OperationIDFromContext(ctx)
	s.actor = command.Actor
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

var _ Operations = (*Handler)(nil)
var _ service.UseCases = (*stubUseCases)(nil)
