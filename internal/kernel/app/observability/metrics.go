package observability

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	pkgobservability "github.com/rin721/go-scaffold-template/pkg/observability"
)

type metricsResource struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge
	dropped  prometheus.Counter
	exported prometheus.Counter
}

type metricsAccess struct{ delegate app.Lease[*metricsResource] }

type metricsRecorder interface {
	ObserveHTTP(string, string, int, time.Duration) error
	IncInFlight() error
	DecInFlight() error
	RecordDropped(int) error
	RecordExported(int) error
}

func buildMetrics(ctx context.Context, _ struct{}, _ struct{}) (*metricsResource, error) {
	if ctx == nil {
		return nil, fmt.Errorf("observability metrics context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := &metricsResource{registry: prometheus.NewRegistry()}
	result.requests = prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: "app", Subsystem: "http", Name: "requests_total", Help: "按稳定 operation 汇总的 HTTP 请求数。"}, []string{"operation", "method", "status_class", "error_class"})
	result.duration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: "app", Subsystem: "http", Name: "request_duration_seconds", Help: "按稳定 operation 汇总的 HTTP 请求耗时。", Buckets: prometheus.DefBuckets}, []string{"operation", "method"})
	result.inFlight = prometheus.NewGauge(prometheus.GaugeOpts{Namespace: "app", Subsystem: "http", Name: "in_flight_requests", Help: "当前业务 HTTP 请求数。"})
	result.dropped = prometheus.NewCounter(prometheus.CounterOpts{Namespace: "app", Subsystem: "telemetry", Name: "dropped_spans_total", Help: "因队列或导出失败丢弃的 span 数。"})
	result.exported = prometheus.NewCounter(prometheus.CounterOpts{Namespace: "app", Subsystem: "telemetry", Name: "exported_spans_total", Help: "已成功导出的 span 数。"})
	for name, collector := range map[string]prometheus.Collector{"HTTP request": result.requests, "HTTP duration": result.duration, "HTTP in-flight": result.inFlight, "telemetry dropped": result.dropped, "telemetry exported": result.exported} {
		if err := result.registry.Register(collector); err != nil {
			return nil, fmt.Errorf("register %s metric: %w", name, err)
		}
	}
	return result, nil
}

func newMetricsAccess(delegate app.Lease[*metricsResource]) (pkgobservability.Metrics, error) {
	if delegate == nil {
		return nil, fmt.Errorf("observability metrics lease is nil")
	}
	return &metricsAccess{delegate: delegate}, nil
}

func (a *metricsAccess) Handler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		err := a.delegate.Use(request.Context(), func(current *metricsResource) error {
			promhttp.HandlerFor(current.registry, promhttp.HandlerOpts{EnableOpenMetrics: true}).ServeHTTP(writer, request)
			return nil
		})
		if err != nil {
			http.Error(writer, "metrics unavailable", http.StatusServiceUnavailable)
		}
	})
}

func (a *metricsAccess) ObserveHTTP(operation, method string, status int, duration time.Duration) error {
	return a.delegate.Use(context.Background(), func(current *metricsResource) error {
		statusClass := strconv.Itoa(status/100) + "xx"
		errorClass := "none"
		if status >= 400 && status < 500 {
			errorClass = "client"
		} else if status >= 500 {
			errorClass = "server"
		}
		current.requests.WithLabelValues(operation, method, statusClass, errorClass).Inc()
		current.duration.WithLabelValues(operation, method).Observe(duration.Seconds())
		return nil
	})
}

func (a *metricsAccess) IncInFlight() error {
	return a.use(func(current *metricsResource) { current.inFlight.Inc() })
}
func (a *metricsAccess) DecInFlight() error {
	return a.use(func(current *metricsResource) { current.inFlight.Dec() })
}
func (a *metricsAccess) RecordDropped(count int) error {
	if count <= 0 {
		return nil
	}
	return a.use(func(current *metricsResource) { current.dropped.Add(float64(count)) })
}
func (a *metricsAccess) RecordExported(count int) error {
	if count <= 0 {
		return nil
	}
	return a.use(func(current *metricsResource) { current.exported.Add(float64(count)) })
}
func (a *metricsAccess) use(operation func(*metricsResource)) error {
	return a.delegate.Use(context.Background(), func(current *metricsResource) error { operation(current); return nil })
}

var _ pkgobservability.Metrics = (*metricsAccess)(nil)
var _ metricsRecorder = (*metricsAccess)(nil)
