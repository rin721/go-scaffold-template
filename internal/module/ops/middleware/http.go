// Package middleware 实现 Ops module 拥有的业务 HTTP trace 与 metrics 边界。
package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	prometheusadapter "github.com/rin721/go-scaffold-template/internal/module/ops/adapter/prometheus"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Operation 是 composition 提供给 Ops 的稳定、低基数路由事实。
type Operation struct{ ID, Method, Path string }

// HTTP 使用显式 operation inventory，绝不把 raw path、subject 或对象 ID 写入观测属性。
func HTTP(tracer trace.Tracer, metrics *prometheusadapter.Registry, operations []Operation) func(http.Handler) http.Handler {
	traceContext := propagation.TraceContext{}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			operation := resolveOperation(request.Method, request.URL.Path, operations)
			ctx := traceContext.Extract(request.Context(), propagation.HeaderCarrier(request.Header))
			ctx, span := tracer.Start(ctx, operation, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
				attribute.String("http.request.method", request.Method), attribute.String("http.route", operation),
			))
			if spanID := span.SpanContext().TraceID(); spanID.IsValid() {
				ctx = httpx.WithTraceID(ctx, spanID.String())
			}
			request = request.WithContext(ctx)
			capture := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
			startedAt := time.Now()
			metrics.IncInFlight()
			defer func() {
				metrics.DecInFlight()
				metrics.ObserveHTTP(operation, request.Method, capture.status, time.Since(startedAt))
				span.SetAttributes(attribute.Int("http.response.status_code", capture.status))
				if capture.status >= 500 {
					span.SetStatus(codes.Error, "server_error")
				}
				span.End()
			}()
			next.ServeHTTP(capture, request)
		})
	}
}

// Management 为独立管理面施加 body、并发和 application deadline 预算。
func Management(next http.Handler, requestTimeout time.Duration, maxBodyBytes int64, maxInFlight int) http.Handler {
	slots := make(chan struct{}, maxInFlight)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			writer.Header().Set("Retry-After", "1")
			http.Error(writer, "management overloaded", http.StatusServiceUnavailable)
			return
		}
		if request.ContentLength > maxBodyBytes {
			http.Error(writer, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		request.Body = http.MaxBytesReader(writer, request.Body, maxBodyBytes)
		ctx, cancel := context.WithTimeout(request.Context(), requestTimeout)
		defer cancel()
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(payload []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(payload)
}

func resolveOperation(method, requestPath string, operations []Operation) string {
	for _, operation := range operations {
		if method == operation.Method && matchPath(operation.Path, requestPath) {
			return operation.ID
		}
	}
	return "unmatched"
}

func matchPath(pattern, requestPath string) bool {
	want := strings.Split(strings.Trim(pattern, "/"), "/")
	got := strings.Split(strings.Trim(requestPath, "/"), "/")
	if len(want) != len(got) {
		return false
	}
	for index := range want {
		if strings.HasPrefix(want[index], "{") && strings.HasSuffix(want[index], "}") {
			if got[index] == "" {
				return false
			}
			continue
		}
		if want[index] != got[index] {
			return false
		}
	}
	return true
}
