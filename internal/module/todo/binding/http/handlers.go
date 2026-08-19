package httpbinding

import (
	"net/http"

	todohandler "github.com/rin721/go-scaffold-template/internal/module/todo/handler"
	"github.com/rin721/go-scaffold-template/pkg/httpx/contract"
)

// RuntimeHandlers 把模块顶层 typed Operations 装箱为按 operationId 索引的运行期执行器，供
// internal/transport/http 的单一 Dispatcher 使用。模块不创建 Router、不加载 OpenAPI。
func RuntimeHandlers(operations todohandler.Operations) map[contract.OperationID]contract.Handler {
	if operations == nil {
		return nil
	}
	return map[contract.OperationID]contract.Handler{
		"createTodo":   contract.JSONBody(operations.CreateTodo, http.StatusCreated),
		"listTodos":    contract.Query(operations.ListTodos, http.StatusOK),
		"getTodo":      contract.Path("id", operations.GetTodo, http.StatusOK),
		"completeTodo": contract.Path("id", operations.CompleteTodo, http.StatusOK),
	}
}
