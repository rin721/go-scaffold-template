package httpx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rin721/go-scaffold-template/pkg/idgen"
	"github.com/rin721/go-scaffold-template/pkg/logger"
)

const headerRequestID = "X-Request-ID"

type requestIDContextKey struct{}
type operationIDContextKey struct{}
type traceIDContextKey struct{}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// RequestIDFromContext 读取 RequestID 中间件写入的 request id。
func RequestIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	value, ok := ctx.Value(requestIDContextKey{}).(string)
	return value, ok && value != ""
}

// WithOperationID 把生成 inventory 中的稳定 operationId 写入请求上下文。
func WithOperationID(ctx context.Context, operationID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, operationIDContextKey{}, operationID)
}

// OperationIDFromContext 读取 strict transport 写入的稳定 operationId。
func OperationIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	value, ok := ctx.Value(operationIDContextKey{}).(string)
	return value, ok && value != ""
}

// WithTraceID 允许观测 Adapter 把已经校验的 trace id 注入项目 HTTP 上下文。
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, traceIDContextKey{}, traceID)
}

// TraceIDFromContext 返回用于结构日志关联的 trace id。
func TraceIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	value, ok := ctx.Value(traceIDContextKey{}).(string)
	return value, ok && value != ""
}

// Recovery 将 panic 转换为统一 500 错误。
func Recovery(log logger.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context) (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					if log != nil {
						log.Error("http panic recovered", logger.String("panic_type", fmt.Sprintf("%T", recovered)))
					}
					err = &StatusError{StatusCode: http.StatusInternalServerError, Code: errorCodeInternalServer, Message: "internal server error", Err: fmt.Errorf("%v", recovered)}
				}
			}()
			return next(ctx)
		}
	}
}

// RequestID 确保每个请求都有 request id，并写入包内上下文。
func RequestID(generator idgen.Generator) Middleware {
	if generator == nil {
		generator = idgen.UUID()
	}
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			requestID := ctx.Request.Header.Get(headerRequestID)
			if !requestIDPattern.MatchString(requestID) {
				generated, err := generator.New()
				if err != nil {
					return err
				}
				requestID = generated
			}
			ctx.ResponseWriter.Header().Set(headerRequestID, requestID)
			ctx.Request = ctx.Request.WithContext(context.WithValue(ctx.Request.Context(), requestIDContextKey{}, requestID))
			return next(ctx)
		}
	}
}

// AccessLog 记录稳定 operation、请求方法、耗时和 request id，不记录原始 URL。
func AccessLog(log logger.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			startedAt := time.Now()
			err := next(ctx)
			if log != nil {
				fields := []logger.Field{
					logger.String("method", ctx.Request.Method),
					logger.Duration("duration", time.Since(startedAt)),
				}
				if operationID, ok := OperationIDFromContext(ctx.Request.Context()); ok {
					fields = append(fields, logger.String("operation", operationID))
				}
				if requestID, ok := RequestIDFromContext(ctx.Request.Context()); ok {
					fields = append(fields, logger.String("request_id", requestID))
				}
				if traceID, ok := TraceIDFromContext(ctx.Request.Context()); ok {
					fields = append(fields, logger.String("trace_id", traceID))
				}
				status, code, recoverable, failed := classifyHTTPLogOutcome(ctx.ResponseWriter, err)
				if failed {
					fields = append(fields,
						logger.Int("status", status),
						logger.String("status_class", fmt.Sprintf("%dxx", status/100)),
						logger.String("error_code", code),
					)
					if err != nil {
						fields = append(fields, logger.String("error_type", fmt.Sprintf("%T", err)))
					}
					if recoverable {
						log.Warn("http request rejected", fields...)
					} else {
						log.Error("http request failed", fields...)
					}
				} else {
					log.Info("http request completed", fields...)
				}
			}
			return err
		}
	}
}

func classifyHTTPLogOutcome(writer http.ResponseWriter, err error) (status int, code string, recoverable bool, failed bool) {
	if responseStatus, problemCode, ok := responseOutcome(writer); ok {
		status = responseStatus
		code = problemCode
		failed = status >= http.StatusBadRequest
	}
	if err == nil && !failed {
		return status, code, false, false
	}
	if status == 0 {
		status = http.StatusInternalServerError
	}
	if code == "" {
		code = errorCodeInternalServer
	}
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		status = statusErrorStatusCode(statusErr)
		code = statusErr.Code
	}
	recoverable = status >= http.StatusBadRequest && status < http.StatusInternalServerError
	if status == http.StatusServiceUnavailable && code == "server_overloaded" {
		recoverable = true
	}
	return status, code, recoverable, true
}

// SecureHeaders 写入常用安全响应头。
func SecureHeaders() Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			header := ctx.ResponseWriter.Header()
			header.Set("X-Content-Type-Options", "nosniff")
			header.Set("X-Frame-Options", "DENY")
			header.Set("Referrer-Policy", "no-referrer")
			return next(ctx)
		}
	}
}

// CORS 只允许显式 origin/method/header；空 origin allowlist 表示拒绝跨域。
func CORS(cfg CORSConfig) Middleware {
	origins := stringSet(cfg.AllowedOrigins, false)
	methods := stringSet(cfg.AllowedMethods, true)
	headers := stringSet(cfg.AllowedHeaders, true)
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			origin := ctx.Request.Header.Get("Origin")
			if origin == "" {
				return next(ctx)
			}
			if _, allowed := origins[origin]; !allowed {
				return &StatusError{StatusCode: http.StatusForbidden, Code: "cors_origin_denied", Message: "cross-origin request is not allowed"}
			}
			header := ctx.ResponseWriter.Header()
			header.Add("Vary", "Origin")
			header.Set("Access-Control-Allow-Origin", origin)
			if ctx.Request.Method == string(MethodOptions) {
				requestedMethod := strings.ToUpper(ctx.Request.Header.Get("Access-Control-Request-Method"))
				if _, allowed := methods[requestedMethod]; !allowed {
					return &StatusError{StatusCode: http.StatusForbidden, Code: "cors_method_denied", Message: "cross-origin method is not allowed"}
				}
				for _, requestedHeader := range strings.Split(ctx.Request.Header.Get("Access-Control-Request-Headers"), ",") {
					requestedHeader = strings.ToUpper(strings.TrimSpace(requestedHeader))
					if requestedHeader == "" {
						continue
					}
					if _, allowed := headers[requestedHeader]; !allowed {
						return &StatusError{StatusCode: http.StatusForbidden, Code: "cors_header_denied", Message: "cross-origin header is not allowed"}
					}
				}
				header.Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ","))
				header.Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ","))
				return ctx.NoContent(http.StatusNoContent)
			}
			return next(ctx)
		}
	}
}

func stringSet(values []string, uppercase bool) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if uppercase {
			value = strings.ToUpper(value)
		}
		result[value] = struct{}{}
	}
	return result
}

// BodyLimit 限制请求体大小。
func BodyLimit(maxBytes int64) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			if maxBytes <= 0 {
				return next(ctx)
			}
			ctx.Request.Body = http.MaxBytesReader(ctx.ResponseWriter, ctx.Request.Body, maxBytes)
			return next(ctx)
		}
	}
}

// RequestTimeout 为每个请求建立有界 application deadline。
func RequestTimeout(timeout time.Duration) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			if timeout <= 0 {
				return next(ctx)
			}
			requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), timeout)
			defer cancel()
			ctx.Request = ctx.Request.WithContext(requestCtx)
			err := next(ctx)
			if errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
				return &StatusError{StatusCode: http.StatusGatewayTimeout, Code: "request_timeout", Message: "request deadline exceeded", Err: errors.Join(err, requestCtx.Err())}
			}
			return err
		}
	}
}

type clientIPContextKey struct{}

// ClientIPFromContext 返回经过 trusted proxy policy 解析的客户端地址。
func ClientIPFromContext(ctx context.Context) (net.IP, bool) {
	if ctx == nil {
		return nil, false
	}
	value, ok := ctx.Value(clientIPContextKey{}).(net.IP)
	return append(net.IP(nil), value...), ok && value != nil
}

// TrustedProxy 只在直连 peer 命中显式 CIDR 时采用 X-Forwarded-For 首地址。
func TrustedProxy(cidrs []string) (Middleware, error) {
	networks := make([]*net.IPNet, 0, len(cidrs))
	for _, value := range cidrs {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("parse trusted proxy CIDR %q: %w", value, err)
		}
		networks = append(networks, network)
	}
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			peer := remoteIP(ctx.Request.RemoteAddr)
			client := peer
			if containsIP(networks, peer) {
				forwarded := strings.Split(ctx.Request.Header.Get("X-Forwarded-For"), ",")[0]
				if parsed := net.ParseIP(strings.TrimSpace(forwarded)); parsed != nil {
					client = parsed
				}
			}
			ctx.Request = ctx.Request.WithContext(context.WithValue(ctx.Request.Context(), clientIPContextKey{}, client))
			return next(ctx)
		}
	}, nil
}

func remoteIP(address string) net.IP {
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		return net.ParseIP(host)
	}
	return net.ParseIP(address)
}

func containsIP(networks []*net.IPNet, value net.IP) bool {
	for _, network := range networks {
		if value != nil && network.Contains(value) {
			return true
		}
	}
	return false
}

// AcceptJSON 拒绝无法接收 JSON 或 Problem Details 的显式 Accept。
func AcceptJSON() Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			accept := ctx.Request.Header.Get("Accept")
			if accept == "" || acceptsJSON(accept) {
				return next(ctx)
			}
			return &StatusError{StatusCode: http.StatusNotAcceptable, Code: "not_acceptable", Message: "response representation is not acceptable"}
		}
	}
}

func acceptsJSON(value string) bool {
	for _, item := range strings.Split(value, ",") {
		mediaType := strings.TrimSpace(strings.SplitN(item, ";", 2)[0])
		switch mediaType {
		case "*/*", "application/*", "application/json", problemContentType:
			return true
		}
	}
	return false
}

// RejectUpgrade 确定性拒绝当前同步 HTTP profile 未治理的升级连接。
func RejectUpgrade() Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			if ctx.Request.Header.Get("Upgrade") != "" || strings.Contains(strings.ToLower(ctx.Request.Header.Get("Connection")), "upgrade") {
				return &StatusError{StatusCode: http.StatusUpgradeRequired, Code: "upgrade_not_supported", Message: "connection upgrade is not supported"}
			}
			return next(ctx)
		}
	}
}

// RateLimiter 使用惰性补充的令牌桶限制入口请求速率。
type RateLimiter struct {
	mu              sync.Mutex
	tokens          float64
	capacity        float64
	tokensPerSecond float64
	lastRefill      time.Time
}

// NewRateLimiter 创建不持有后台 goroutine 的入口限流器。
func NewRateLimiter(tokensPerSecond int) *RateLimiter {
	return NewRateLimiterWithBurst(tokensPerSecond, tokensPerSecond)
}

// NewRateLimiterWithBurst 创建显式速率和突发容量的无 goroutine 令牌桶。
func NewRateLimiterWithBurst(tokensPerSecond, burst int) *RateLimiter {
	if tokensPerSecond <= 0 {
		tokensPerSecond = 1
	}
	if burst <= 0 {
		burst = tokensPerSecond
	}
	rate := float64(tokensPerSecond)
	return &RateLimiter{
		tokens:          float64(burst),
		capacity:        float64(burst),
		tokensPerSecond: rate,
		lastRefill:      time.Now(),
	}
}

func (r *RateLimiter) allow(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	elapsed := now.Sub(r.lastRefill).Seconds()
	if elapsed > 0 {
		r.tokens += elapsed * r.tokensPerSecond
		if r.tokens > r.capacity {
			r.tokens = r.capacity
		}
		r.lastRefill = now
	}
	if r.tokens < 1 {
		return false
	}
	r.tokens--
	return true
}

// Middleware 返回 HTTP 中间件，限流状态由 RateLimiter 实例持有。
func (r *RateLimiter) Middleware() Middleware {
	if r == nil {
		r = NewRateLimiter(1)
	}
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			if err := ctx.Request.Context().Err(); err != nil {
				return ctx.Request.Context().Err()
			}
			if !r.allow(time.Now()) {
				return &StatusError{StatusCode: http.StatusTooManyRequests, Code: "rate_limited", Message: "request quota exceeded", RetryAfter: 1}
			}
			return next(ctx)
		}
	}
}

// OverloadLimiter 以非阻塞 semaphore 限制单进程 in-flight 请求。
type OverloadLimiter struct{ slots chan struct{} }

// NewOverloadLimiter 创建不启动后台 goroutine 的并发门禁。
func NewOverloadLimiter(maxInFlight int) *OverloadLimiter {
	if maxInFlight <= 0 {
		maxInFlight = 1
	}
	return &OverloadLimiter{slots: make(chan struct{}, maxInFlight)}
}

// Middleware 在容量耗尽时返回 503，不排队占用未知预算。
func (l *OverloadLimiter) Middleware() Middleware {
	if l == nil {
		l = NewOverloadLimiter(1)
	}
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			select {
			case l.slots <- struct{}{}:
				defer func() { <-l.slots }()
				return next(ctx)
			default:
				return &StatusError{StatusCode: http.StatusServiceUnavailable, Code: "server_overloaded", Message: "server is overloaded", RetryAfter: 1}
			}
		}
	}
}

// RateLimit 使用默认 RateLimiter 创建限流中间件。
func RateLimit(tokensPerSecond int) Middleware {
	return NewRateLimiter(tokensPerSecond).Middleware()
}
