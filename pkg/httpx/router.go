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
	return &standardRouter{router: router, errorHandle: resolved.ErrorHandler}
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

func (r *standardRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

// DefaultErrorHandler 是默认统一错误处理函数。
func DefaultErrorHandler(ctx *Context, err error) {
	if statusErr, ok := asStatusError(err); ok {
		_ = ctx.JSON(statusErrorStatusCode(statusErr), map[string]string{
			"error":   statusErrorCode(statusErr),
			"message": statusErrorMessage(statusErr),
		})
		return
	}

	_ = ctx.JSON(http.StatusInternalServerError, map[string]string{
		"error": errorCodeInternalServer,
	})
}
