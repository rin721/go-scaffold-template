package composition

import (
	"context"
	"errors"
	"testing"
	"time"

	authmodel "github.com/rin721/go-scaffold-template/internal/module/auth/model"
	httptransport "github.com/rin721/go-scaffold-template/internal/transport/http"
	"github.com/rin721/go-scaffold-template/internal/transport/http/api"
)

func TestStrictAPIServerDelegatesToTodoOperations(t *testing.T) {
	operations := &todoOperationsStub{}
	server, err := newStrictAPIServer(operations)
	if err != nil {
		t.Fatalf("newStrictAPIServer() error = %v", err)
	}
	if _, err := server.CreateTodo(t.Context(), api.CreateTodoRequestObject{}); err != nil {
		t.Fatalf("CreateTodo() error = %v", err)
	}
	if operations.createCalls != 1 {
		t.Fatalf("Todo CreateTodo() calls = %d", operations.createCalls)
	}
	if _, err := newStrictAPIServer(nil); err == nil {
		t.Fatal("newStrictAPIServer(nil) error = nil")
	}
	var typedNil *todoOperationsStub
	if _, err := newStrictAPIServer(typedNil); err == nil {
		t.Fatal("newStrictAPIServer(typed nil) error = nil")
	}
}

func TestOperationGateMapsAuthenticationAuthorizationAndDependencyErrors(t *testing.T) {
	privateErr := errors.New("private auth dependency")
	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want error
	}{
		{name: "missing principal", ctx: t.Context(), want: httptransport.ErrUnauthenticated},
		{name: "permission denied", ctx: principalContext(t), err: authmodel.ErrPermissionDenied, want: httptransport.ErrPermissionDenied},
		{name: "auth rejected", ctx: principalContext(t), err: authmodel.ErrUnauthenticated, want: httptransport.ErrUnauthenticated},
		{name: "dependency", ctx: principalContext(t), err: privateErr, want: privateErr},
		{name: "allowed", ctx: principalContext(t)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer := &operationAuthorizerStub{err: test.err}
			gate, err := newOperationGate(authorizer)
			if err != nil {
				t.Fatalf("newOperationGate() error = %v", err)
			}
			err = gate.Enforce(test.ctx, "createTodo")
			if !errors.Is(err, test.want) {
				t.Fatalf("Enforce() error = %v, want %v", err, test.want)
			}
		})
	}
	if _, err := newOperationGate(nil); err == nil {
		t.Fatal("newOperationGate(nil) error = nil")
	}
}

func TestTodoActorAccessMapsOnlyAuthenticatedPrincipal(t *testing.T) {
	access := todoActorAccessAdapter{}
	if _, ok := access.Actor(t.Context()); ok {
		t.Fatal("Actor() authenticated without principal")
	}
	actor, ok := access.Actor(principalContext(t))
	if !ok || actor.Subject != "subject-1" || actor.Kind != "service" || len(actor.Scopes) != 1 || actor.Scopes[0] != "todos:write" {
		t.Fatalf("Actor() = %#v, %t", actor, ok)
	}
}

func principalContext(t *testing.T) context.Context {
	t.Helper()
	now := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	principal, err := authmodel.NewPrincipal("subject-1", authmodel.ActorService, []authmodel.Scope{"todos:write"}, now, now)
	if err != nil {
		t.Fatalf("NewPrincipal() error = %v", err)
	}
	return authmodel.WithPrincipal(t.Context(), principal)
}

type operationAuthorizerStub struct{ err error }

func (s *operationAuthorizerStub) EnforceOperation(context.Context, authmodel.Principal, string) error {
	return s.err
}

type todoOperationsStub struct{ createCalls int }

func (*todoOperationsStub) ListTodos(context.Context, api.ListTodosRequestObject) (api.ListTodosResponseObject, error) {
	return api.ListTodos200JSONResponse{}, nil
}

func (s *todoOperationsStub) CreateTodo(context.Context, api.CreateTodoRequestObject) (api.CreateTodoResponseObject, error) {
	s.createCalls++
	return api.CreateTodo201JSONResponse{}, nil
}

func (*todoOperationsStub) GetTodo(context.Context, api.GetTodoRequestObject) (api.GetTodoResponseObject, error) {
	return api.GetTodo200JSONResponse{}, nil
}

func (*todoOperationsStub) CompleteTodo(context.Context, api.CompleteTodoRequestObject) (api.CompleteTodoResponseObject, error) {
	return api.CompleteTodo200JSONResponse{}, nil
}

var _ operationAuthorizer = (*operationAuthorizerStub)(nil)
