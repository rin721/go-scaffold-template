// Package prometheusadapter 封装进程级稳定 Prometheus registry 与低基数指标。
package prometheusadapter

import (
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry 在进程期只注册一次 collector，跨 generation 复用同一 identity。
type Registry struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge
	dropped  prometheus.Counter
	exported prometheus.Counter
	depth    atomic.Int64
}

// New 创建独立于 Prometheus global registry 的稳定注册表。
func New() (*Registry, error) {
	result := &Registry{registry: prometheus.NewRegistry()}
	result.requests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "app", Subsystem: "http", Name: "requests_total", Help: "按稳定 operation 汇总的 HTTP 请求数。",
	}, []string{"operation", "method", "status_class", "error_class"})
	result.duration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "app", Subsystem: "http", Name: "request_duration_seconds", Help: "按稳定 operation 汇总的 HTTP 请求耗时。",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation", "method"})
	result.inFlight = prometheus.NewGauge(prometheus.GaugeOpts{Namespace: "app", Subsystem: "http", Name: "in_flight_requests", Help: "当前业务 HTTP 请求数。"})
	result.dropped = prometheus.NewCounter(prometheus.CounterOpts{Namespace: "app", Subsystem: "telemetry", Name: "dropped_spans_total", Help: "因队列或导出失败丢弃的 span 数。"})
	result.exported = prometheus.NewCounter(prometheus.CounterOpts{Namespace: "app", Subsystem: "telemetry", Name: "exported_spans_total", Help: "已成功导出的 span 数。"})
	if err := result.registry.Register(result.requests); err != nil {
		return nil, fmt.Errorf("register HTTP request metric: %w", err)
	}
	if err := result.registry.Register(result.duration); err != nil {
		return nil, fmt.Errorf("register HTTP duration metric: %w", err)
	}
	if err := result.registry.Register(result.inFlight); err != nil {
		return nil, fmt.Errorf("register HTTP in-flight metric: %w", err)
	}
	if err := result.registry.Register(result.dropped); err != nil {
		return nil, fmt.Errorf("register telemetry dropped metric: %w", err)
	}
	if err := result.registry.Register(result.exported); err != nil {
		return nil, fmt.Errorf("register telemetry exported metric: %w", err)
	}
	return result, nil
}

// Handler 返回只连接本 registry 的 Prometheus exposition handler。
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

// ObserveHTTP 记录一个已经归一化的低基数请求结果。
func (r *Registry) ObserveHTTP(operation, method string, status int, duration time.Duration) {
	statusClass := strconv.Itoa(status/100) + "xx"
	errorClass := "none"
	if status >= 400 && status < 500 {
		errorClass = "client"
	}
	if status >= 500 {
		errorClass = "server"
	}
	r.requests.WithLabelValues(operation, method, statusClass, errorClass).Inc()
	r.duration.WithLabelValues(operation, method).Observe(duration.Seconds())
}

func (r *Registry) IncInFlight() { r.inFlight.Inc() }
func (r *Registry) DecInFlight() { r.inFlight.Dec() }

// RecordDropped 把 SDK queue/drop 与 exporter failure 统一投影为低敏计数。
func (r *Registry) RecordDropped(count int) {
	if count <= 0 {
		return
	}
	r.dropped.Add(float64(count))
}

// RecordExported 记录成功导出的 span 数。
func (r *Registry) RecordExported(count int) {
	if count <= 0 {
		return
	}
	r.exported.Add(float64(count))
}

// SetQueueDepth 保存 diagnostics 使用的当前队列深度。
func (r *Registry) SetQueueDepth(depth int64) { r.depth.Store(depth) }

// QueueDepth 返回当前 exporter queue 深度。
func (r *Registry) QueueDepth() int64 { return r.depth.Load() }
