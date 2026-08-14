// Package httpbinding 绑定 Todo HTTP Handler 与稳定路由。
package httpbinding

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/module"
	"github.com/rin721/go-scaffold-template/internal/module/todo/handler"
	todomiddleware "github.com/rin721/go-scaffold-template/internal/module/todo/middleware"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
)

// Routes 返回 Todo 模块的完整 HTTP 路由贡献。
func Routes(todoHandler *handler.Handler) ([]module.Route, error) {
	if todoHandler == nil {
		return nil, fmt.Errorf("todo HTTP handler is nil")
	}
	return []module.Route{
		{
			Method: httpx.MethodPost, Path: "/api/v1/todos", Handler: todoHandler.Create,
			Middlewares: []httpx.Middleware{todomiddleware.RequireJSONContentType()},
		},
		{Method: httpx.MethodGet, Path: "/api/v1/todos/{id}", Handler: todoHandler.Get},
		{Method: httpx.MethodGet, Path: "/api/v1/todos", Handler: todoHandler.List},
		{Method: httpx.MethodPatch, Path: "/api/v1/todos/{id}/complete", Handler: todoHandler.Complete},
	}, nil
}
