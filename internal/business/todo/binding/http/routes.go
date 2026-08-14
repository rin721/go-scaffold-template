// Package httpbinding 绑定 Todo HTTP Handler 与稳定路由。
package httpbinding

import (
	"fmt"

	"github.com/rin721/go-scaffold2/internal/business"
	"github.com/rin721/go-scaffold2/internal/business/todo/handler"
	todomiddleware "github.com/rin721/go-scaffold2/internal/business/todo/middleware"
	"github.com/rin721/go-scaffold2/pkg/httpx"
)

// Routes 返回 Todo 模块的完整 HTTP 路由贡献。
func Routes(todoHandler *handler.Handler) ([]business.Route, error) {
	if todoHandler == nil {
		return nil, fmt.Errorf("todo HTTP handler is nil")
	}
	return []business.Route{
		{
			Method: httpx.MethodPost, Path: "/api/v1/todos", Handler: todoHandler.Create,
			Middlewares: []httpx.Middleware{todomiddleware.RequireJSONContentType()},
		},
		{Method: httpx.MethodGet, Path: "/api/v1/todos/{id}", Handler: todoHandler.Get},
		{Method: httpx.MethodGet, Path: "/api/v1/todos", Handler: todoHandler.List},
		{Method: httpx.MethodPatch, Path: "/api/v1/todos/{id}/complete", Handler: todoHandler.Complete},
	}, nil
}
