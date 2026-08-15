package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Router 定义服务端路由能力。
type Router interface {
	Use(middlewares ...Middleware)
	UseHTTP(middlewares ...func(http.Handler) http.Handler)
	Handle(method Method, pattern string, handler Handler, middlewares ...Middleware)
	Mount(pattern string, handler http.Handler)
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type standardRouter struct {
	router      chi.Router
	errorHandle ErrorHandler
}

// NewRouter 根据配置创建 Router。
func NewRouter(cfg *RouterConfig) Router {
	resolved := resolveRouterConfig(cfg)
	router := chi.NewRouter()
	result := &standardRouter{router: router, errorHandle: resolved.ErrorHandler}
	router.NotFound(func(w http.ResponseWriter, request *http.Request) {
		result.errorHandle(&Context{ResponseWriter: w, Request: request}, &StatusError{
			StatusCode: http.StatusNotFound, Code: "route_not_found", Message: "route not found",
		})
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, request *http.Request) {
		result.errorHandle(&Context{ResponseWriter: w, Request: request}, &StatusError{
			StatusCode: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: "method not allowed",
		})
	})
	return result
}

func (r *standardRouter) Use(middlewares ...Middleware) {
	for _, middleware := range middlewares {
		if middleware == nil {
			continue
		}
		r.router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				ctx := &Context{ResponseWriter: w, Request: req}
				if err := middleware(func(ctx *Context) error {
					next.ServeHTTP(ctx.ResponseWriter, ctx.Request)
					return nil
				})(ctx); err != nil {
					r.errorHandle(ctx, err)
				}
			})
		})
	}
}

func (r *standardRouter) UseHTTP(middlewares ...func(http.Handler) http.Handler) {
	for _, middleware := range middlewares {
		if middleware != nil {
			r.router.Use(middleware)
		}
	}
}

func (r *standardRouter) Handle(method Method, pattern string, handler Handler, middlewares ...Middleware) {
	finalHandler := chain(handler, middlewares...)

	r.router.MethodFunc(string(methodOrDefault(method)), pattern, func(w http.ResponseWriter, req *http.Request) {
		ctx := &Context{ResponseWriter: w, Request: req}
		if err := finalHandler(ctx); err != nil {
			r.errorHandle(ctx, err)
		}
	})
}

func (r *standardRouter) Mount(pattern string, handler http.Handler) {
	if handler == nil {
		return
	}
	r.router.Mount(pattern, handler)
}

func (r *standardRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if _, ok := w.(*responseStateWriter); !ok {
		w = &responseStateWriter{ResponseWriter: w}
	}
	r.router.ServeHTTP(w, req)
}

// DefaultErrorHandler 是默认统一错误处理函数。
func DefaultErrorHandler(ctx *Context, err error) {
	WriteProblem(ctx.ResponseWriter, ctx.Request, err)
}
