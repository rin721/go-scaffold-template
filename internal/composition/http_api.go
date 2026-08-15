package composition

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	authmodel "github.com/rin721/go-scaffold-template/internal/module/auth/model"
	todohttp "github.com/rin721/go-scaffold-template/internal/module/todo/binding/http"
	httptransport "github.com/rin721/go-scaffold-template/internal/transport/http"
	"github.com/rin721/go-scaffold-template/internal/transport/http/api"
)

type operationAuthorizer interface {
	EnforceOperation(context.Context, authmodel.Principal, string) error
}

// operationGateAdapter 在应用 Auth 与公共 HTTP operation 之间保持单一授权边界。
type operationGateAdapter struct{ auth operationAuthorizer }

func newOperationGate(auth operationAuthorizer) (httptransport.OperationGate, error) {
	if nilDependency(auth) {
		return nil, fmt.Errorf("auth service for HTTP operation gate is nil")
	}
	return operationGateAdapter{auth: auth}, nil
}

func (a operationGateAdapter) Authenticate(ctx context.Context) error {
	if _, ok := authmodel.PrincipalFromContext(ctx); !ok {
		return httptransport.ErrUnauthenticated
	}
	return nil
}

func (a operationGateAdapter) Enforce(ctx context.Context, operation string) error {
	principal, ok := authmodel.PrincipalFromContext(ctx)
	if !ok {
		return httptransport.ErrUnauthenticated
	}
	if err := a.auth.EnforceOperation(ctx, principal, operation); err != nil {
		switch {
		case errors.Is(err, authmodel.ErrUnauthenticated):
			return httptransport.ErrUnauthenticated
		case errors.Is(err, authmodel.ErrPermissionDenied):
			return httptransport.ErrPermissionDenied
		default:
			return err
		}
	}
	return nil
}

// strictAPIServer 是唯一满足完整生成接口的应用静态聚合，只转发到模块 operation Handler。
type strictAPIServer struct {
	todo todohttp.Operations
}

func newStrictAPIServer(todoOperations todohttp.Operations) (*strictAPIServer, error) {
	if nilDependency(todoOperations) {
		return nil, fmt.Errorf("Todo HTTP operations are nil")
	}
	return &strictAPIServer{todo: todoOperations}, nil
}

func (s *strictAPIServer) ListTodos(ctx context.Context, request api.ListTodosRequestObject) (api.ListTodosResponseObject, error) {
	return s.todo.ListTodos(ctx, request)
}

func (s *strictAPIServer) CreateTodo(ctx context.Context, request api.CreateTodoRequestObject) (api.CreateTodoResponseObject, error) {
	return s.todo.CreateTodo(ctx, request)
}

func (s *strictAPIServer) GetTodo(ctx context.Context, request api.GetTodoRequestObject) (api.GetTodoResponseObject, error) {
	return s.todo.GetTodo(ctx, request)
}

func (s *strictAPIServer) CompleteTodo(ctx context.Context, request api.CompleteTodoRequestObject) (api.CompleteTodoResponseObject, error) {
	return s.todo.CompleteTodo(ctx, request)
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

var _ httptransport.OperationGate = operationGateAdapter{}
var _ api.StrictServerInterface = (*strictAPIServer)(nil)
