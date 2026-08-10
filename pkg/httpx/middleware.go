package httpx

// Handler 定义服务端路由处理函数。
type Handler func(*Context) error

// Middleware 定义路由中间件。
type Middleware func(Handler) Handler

// ErrorHandler 定义统一错误处理函数。
type ErrorHandler func(*Context, error)

func chain(handler Handler, middlewares ...Middleware) Handler {
	next := handler
	for index := len(middlewares) - 1; index >= 0; index-- {
		if middlewares[index] != nil {
			next = middlewares[index](next)
		}
	}
	return next
}
