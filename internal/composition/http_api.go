package composition

import (
	"context"
	"errors"
	"fmt"

	authmodel "github.com/rin721/go-scaffold-template/internal/module/auth/model"
	todohttp "github.com/rin721/go-scaffold-template/internal/module/todo/binding/http"
	todohandler "github.com/rin721/go-scaffold-template/internal/module/todo/handler"
	httptransport "github.com/rin721/go-scaffold-template/internal/transport/http"
	"github.com/rin721/go-scaffold-template/pkg/httpx/contract"
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

// contractDispatcher 聚合模块契约与运行期 handler，是 route binding 的唯一操作表来源。
type contractDispatcher struct {
	module   contract.Module
	handlers map[contract.OperationID]contract.Handler
}

func newContractDispatcher(todoOperations todohandler.Operations) (*contractDispatcher, error) {
	if nilDependency(todoOperations) {
		return nil, fmt.Errorf("Todo HTTP operations are nil")
	}
	modules := applicationHTTPModules()
	if len(modules) == 0 {
		return nil, fmt.Errorf("no application HTTP modules are registered")
	}
	module := modules[0]
	handlers := todohttp.RuntimeHandlers(todoOperations)
	if len(handlers) == 0 {
		return nil, fmt.Errorf("Todo HTTP runtime handlers are empty")
	}
	for _, operation := range module.Operations {
		if _, ok := handlers[operation.ID]; !ok {
			return nil, fmt.Errorf("Todo operation %q has no runtime handler", operation.ID)
		}
	}
	return &contractDispatcher{module: module, handlers: handlers}, nil
}

func (d *contractDispatcher) Modules() []contract.Module {
	return []contract.Module{d.module}
}

func (d *contractDispatcher) Operations() []contract.Operation {
	return append([]contract.Operation(nil), d.module.Operations...)
}

func (d *contractDispatcher) Handler(operationID contract.OperationID) (contract.Handler, bool) {
	handler, ok := d.handlers[operationID]
	return handler, ok
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	return false
}

var _ httptransport.OperationGate = operationGateAdapter{}
var _ httptransport.Dispatcher = (*contractDispatcher)(nil)
