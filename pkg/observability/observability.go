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
	Diagnostics(context.Context) (Diagnostics, error)
}

// Capabilities 聚合 application composition 可以连接的观测能力。
type Capabilities struct {
	Metrics   Metrics
	Telemetry Telemetry
}
