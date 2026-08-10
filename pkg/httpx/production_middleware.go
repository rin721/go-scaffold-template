package httpx

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rin721/go-scaffold2/pkg/idgen"
	"github.com/rin721/go-scaffold2/pkg/logger"
)

const headerRequestID = "X-Request-ID"

type requestIDContextKey struct{}

// RequestIDFromContext 读取 RequestID 中间件写入的 request id。
func RequestIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	value, ok := ctx.Value(requestIDContextKey{}).(string)
	return value, ok && value != ""
}

// Recovery 将 panic 转换为统一 500 错误。
func Recovery(log logger.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context) (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					if log != nil {
						log.Error("http panic recovered", logger.Any("panic", recovered))
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
			if requestID == "" {
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

// AccessLog 记录请求方法、路径、耗时和 request id。
func AccessLog(log logger.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			startedAt := time.Now()
			err := next(ctx)
			if log != nil {
				fields := []logger.Field{
					logger.String("method", ctx.Request.Method),
					logger.String("path", ctx.Request.URL.Path),
					logger.Duration("duration", time.Since(startedAt)),
				}
				if requestID, ok := RequestIDFromContext(ctx.Request.Context()); ok {
					fields = append(fields, logger.String("request_id", requestID))
				}
				if err != nil {
					fields = append(fields, logger.Error(err))
					log.Error("http request failed", fields...)
				} else {
					log.Info("http request completed", fields...)
				}
			}
			return err
		}
	}
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

// CORSConfig 定义 CORS 策略。
type CORSConfig struct {
	AllowOrigin  string
	AllowMethods string
	AllowHeaders string
}

// CORS 写入受控 CORS 响应头。
func CORS(cfg CORSConfig) Middleware {
	if cfg.AllowOrigin == "" {
		cfg.AllowOrigin = "*"
	}
	if cfg.AllowMethods == "" {
		cfg.AllowMethods = "GET,POST,PUT,PATCH,DELETE,OPTIONS"
	}
	if cfg.AllowHeaders == "" {
		cfg.AllowHeaders = "Content-Type,Authorization,X-Request-ID"
	}
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			header := ctx.ResponseWriter.Header()
			header.Set("Access-Control-Allow-Origin", cfg.AllowOrigin)
			header.Set("Access-Control-Allow-Methods", cfg.AllowMethods)
			header.Set("Access-Control-Allow-Headers", cfg.AllowHeaders)
			if ctx.Request.Method == string(MethodOptions) {
				return ctx.NoContent(http.StatusNoContent)
			}
			return next(ctx)
		}
	}
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
	if tokensPerSecond <= 0 {
		tokensPerSecond = 1
	}
	rate := float64(tokensPerSecond)
	return &RateLimiter{
		tokens:          rate,
		capacity:        rate,
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
				return &StatusError{StatusCode: http.StatusTooManyRequests, Code: "rate_limited", Message: "too many requests"}
			}
			return next(ctx)
		}
	}
}

// RateLimit 使用默认 RateLimiter 创建限流中间件。
func RateLimit(tokensPerSecond int) Middleware {
	return NewRateLimiter(tokensPerSecond).Middleware()
}
