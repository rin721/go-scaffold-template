// Package observability 定义进程观测能力对业务与管理模块开放的最小契约。
package observability

import (
	"context"
	"net/http"
)

// Operation 是 HTTP 观测使用的稳定、低基数路由事实。
type Operation struct {
	ID     string
	Method string
	Path   string
}

// Work 是后台工作 span 的稳定低基数身份，不包含业务参数。
type Work struct {
	Name string
	Kind string
}

// WorkFunc 是在同一 Telemetry provider 下执行的后台工作。
type WorkFunc func(context.Context) error

// Diagnostics 描述 exporter 的低敏、自包含运行状态。
type Diagnostics struct {
	Enabled       bool   `json:"enabled"`
	Ready         bool   `json:"ready"`
	QueueDepth    int64  `json:"queueDepth"`
	DroppedSpans  uint64 `json:"droppedSpans"`
	ExportedSpans uint64 `json:"exportedSpans"`
	LastErrorType string `json:"lastErrorType,omitempty"`
}

// Metrics 提供只连接进程私有 registry 的 exposition 入口。
type Metrics interface {
	Handler() http.Handler
}

// Telemetry 提供请求级观测和低敏诊断，不暴露 tracer、provider 或关闭权。
type Telemetry interface {
	HTTP([]Operation) func(http.Handler) http.Handler
	Observe(context.Context, Work, WorkFunc) error
	Diagnostics(context.Context) (Diagnostics, error)
}

// Capabilities 聚合 application composition 可以连接的观测能力。
type Capabilities struct {
	Metrics   Metrics
	Telemetry Telemetry
}

type traceIDContextKey struct{}

// WithTraceID 把低敏 trace identity 放入项目自有 context，不暴露 SDK Span 类型。
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, traceIDContextKey{}, traceID)
}

// TraceIDFrom 返回由 Telemetry 写入的低敏 trace identity。
func TraceIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	traceID, _ := ctx.Value(traceIDContextKey{}).(string)
	return traceID
}
