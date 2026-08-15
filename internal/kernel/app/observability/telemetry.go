package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	pkgobservability "github.com/rin721/go-scaffold-template/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type telemetryResource struct {
	enabled   bool
	provider  *sdktrace.TracerProvider
	processor *boundedProcessor
	tracer    trace.Tracer
	metrics   metricsRecorder
	started   atomic.Bool
	lastError atomic.Value
}

type telemetryAccess struct{ delegate app.Lease[*telemetryResource] }

func buildTelemetry(ctx context.Context, cfg Config, metrics metricsRecorder) (*telemetryResource, error) {
	if ctx == nil || metrics == nil {
		return nil, fmt.Errorf("telemetry dependencies are incomplete")
	}
	if !cfg.Tracing.Enabled {
		provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()))
		result := &telemetryResource{provider: provider, tracer: provider.Tracer(cfg.ServiceName), metrics: metrics}
		result.lastError.Store("")
		return result, nil
	}
	options := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(cfg.Tracing.Endpoint), otlptracehttp.WithTimeout(cfg.Tracing.ExportTimeout)}
	if cfg.Tracing.Insecure {
		options = append(options, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("create OTLP HTTP exporter: %w", sanitize(err))
	}
	processor := newBoundedProcessor(exporter, cfg.Tracing, metrics)
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.Tracing.SampleRatio)),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", cfg.ServiceName))),
		sdktrace.WithSpanProcessor(processor),
	)
	result := &telemetryResource{enabled: true, provider: provider, processor: processor, tracer: provider.Tracer(cfg.ServiceName), metrics: metrics}
	result.lastError.Store("")
	return result, nil
}

func startTelemetry(ctx context.Context, current *telemetryResource) error {
	if ctx == nil || current == nil || current.provider == nil {
		return fmt.Errorf("telemetry resource is invalid")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if current.started.Swap(true) {
		return fmt.Errorf("telemetry resource is already started")
	}
	if current.processor != nil {
		current.processor.start()
	}
	return nil
}

func stopTelemetry(ctx context.Context, current *telemetryResource) error {
	if current == nil || current.provider == nil {
		return nil
	}
	if ctx == nil {
		return fmt.Errorf("telemetry stop context is nil")
	}
	current.started.Store(false)
	return errors.Join(current.provider.ForceFlush(ctx), current.provider.Shutdown(ctx))
}

func newTelemetryAccess(delegate app.Lease[*telemetryResource]) (pkgobservability.Telemetry, error) {
	if delegate == nil {
		return nil, fmt.Errorf("telemetry lease is nil")
	}
	return &telemetryAccess{delegate: delegate}, nil
}

func (a *telemetryAccess) HTTP(operations []pkgobservability.Operation) func(http.Handler) http.Handler {
	inventory := append([]pkgobservability.Operation(nil), operations...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			err := a.delegate.Use(request.Context(), func(current *telemetryResource) error {
				current.serveHTTP(writer, request, next, inventory)
				return nil
			})
			if err != nil {
				http.Error(writer, "telemetry unavailable", http.StatusServiceUnavailable)
			}
		})
	}
}

func (a *telemetryAccess) Diagnostics(ctx context.Context) (pkgobservability.Diagnostics, error) {
	var result pkgobservability.Diagnostics
	err := a.delegate.Use(ctx, func(current *telemetryResource) error {
		result = current.diagnostics()
		return nil
	})
	return result, err
}

func (r *telemetryResource) serveHTTP(writer http.ResponseWriter, request *http.Request, next http.Handler, operations []pkgobservability.Operation) {
	operation := resolveOperation(request.Method, request.URL.Path, operations)
	traceContext := propagation.TraceContext{}
	ctx := traceContext.Extract(request.Context(), propagation.HeaderCarrier(request.Header))
	ctx, span := r.tracer.Start(ctx, operation, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
		attribute.String("http.request.method", request.Method), attribute.String("http.route", operation),
	))
	if spanID := span.SpanContext().TraceID(); spanID.IsValid() {
		ctx = httpx.WithTraceID(ctx, spanID.String())
	}
	request = request.WithContext(ctx)
	capture := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
	startedAt := time.Now()
	r.captureMetricsError(r.metrics.IncInFlight())
	defer func() {
		r.captureMetricsError(r.metrics.DecInFlight())
		r.captureMetricsError(r.metrics.ObserveHTTP(operation, request.Method, capture.status, time.Since(startedAt)))
		span.SetAttributes(attribute.Int("http.response.status_code", capture.status))
		if capture.status >= 500 {
			span.SetStatus(codes.Error, "server_error")
		}
		span.End()
	}()
	next.ServeHTTP(capture, request)
}

func (r *telemetryResource) diagnostics() pkgobservability.Diagnostics {
	if r == nil {
		return pkgobservability.Diagnostics{}
	}
	if !r.enabled {
		return pkgobservability.Diagnostics{Ready: r.started.Load(), LastErrorType: r.lastError.Load().(string)}
	}
	return r.processor.diagnostics(r.started.Load())
}

func (r *telemetryResource) captureMetricsError(err error) {
	if err != nil {
		r.lastError.Store(errorType(err))
	}
}

type boundedProcessor struct {
	exporter  sdktrace.SpanExporter
	config    Tracing
	metrics   metricsRecorder
	queue     chan sdktrace.ReadOnlySpan
	flush     chan chan error
	stop      chan struct{}
	done      chan struct{}
	startOnce sync.Once
	dropped   atomic.Uint64
	exported  atomic.Uint64
	lastError atomic.Value
}

func newBoundedProcessor(exporter sdktrace.SpanExporter, cfg Tracing, metrics metricsRecorder) *boundedProcessor {
	processor := &boundedProcessor{exporter: exporter, config: cfg, metrics: metrics, queue: make(chan sdktrace.ReadOnlySpan, cfg.QueueSize), flush: make(chan chan error), stop: make(chan struct{}), done: make(chan struct{})}
	processor.lastError.Store("")
	return processor
}

func (p *boundedProcessor) start()                                        { p.startOnce.Do(func() { go p.run() }) }
func (*boundedProcessor) OnStart(context.Context, sdktrace.ReadWriteSpan) {}
func (p *boundedProcessor) OnEnd(span sdktrace.ReadOnlySpan) {
	if span == nil || !span.SpanContext().IsSampled() {
		return
	}
	select {
	case p.queue <- span:
	default:
		p.recordDropped(1)
	}
}

func (p *boundedProcessor) ForceFlush(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("telemetry flush context is nil")
	}
	response := make(chan error, 1)
	select {
	case p.flush <- response:
	case <-ctx.Done():
		return ctx.Err()
	case <-p.done:
		return nil
	}
	select {
	case err := <-response:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *boundedProcessor) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("telemetry shutdown context is nil")
	}
	select {
	case <-p.stop:
	default:
		close(p.stop)
	}
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *boundedProcessor) run() {
	defer close(p.done)
	ticker := time.NewTicker(p.config.BatchTimeout)
	defer ticker.Stop()
	batch := make([]sdktrace.ReadOnlySpan, 0, p.config.BatchSize)
	for {
		select {
		case span := <-p.queue:
			batch = append(batch, span)
			if len(batch) >= p.config.BatchSize {
				p.captureExportError(p.export(batch))
				batch = batch[:0]
			}
		case response := <-p.flush:
			batch = p.drain(batch)
			response <- p.export(batch)
			batch = batch[:0]
		case <-ticker.C:
			p.captureExportError(p.export(batch))
			batch = batch[:0]
		case <-p.stop:
			batch = p.drain(batch)
			exportErr := p.export(batch)
			shutdownCtx, cancel := context.WithTimeout(context.Background(), p.config.ShutdownTimeout)
			shutdownErr := p.exporter.Shutdown(shutdownCtx)
			cancel()
			p.captureExportError(errors.Join(exportErr, shutdownErr))
			return
		}
	}
}

func (p *boundedProcessor) drain(batch []sdktrace.ReadOnlySpan) []sdktrace.ReadOnlySpan {
	for {
		select {
		case span := <-p.queue:
			batch = append(batch, span)
		default:
			return batch
		}
	}
}

func (p *boundedProcessor) export(batch []sdktrace.ReadOnlySpan) error {
	if len(batch) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.config.ExportTimeout)
	defer cancel()
	if err := p.exporter.ExportSpans(ctx, batch); err != nil {
		p.recordDropped(len(batch))
		return sanitize(err)
	}
	p.exported.Add(uint64(len(batch)))
	p.captureMetricsError(p.metrics.RecordExported(len(batch)))
	return nil
}

func (p *boundedProcessor) recordDropped(count int) {
	if count <= 0 {
		return
	}
	// #nosec G115 -- count 来自内存队列长度，已证明为正 int，转换不会截断。
	p.dropped.Add(uint64(count))
	p.captureMetricsError(p.metrics.RecordDropped(count))
}

func (p *boundedProcessor) captureMetricsError(err error) {
	if err != nil {
		p.lastError.Store(errorType(err))
	}
}

func (p *boundedProcessor) captureExportError(err error) {
	if err != nil {
		p.lastError.Store(errorType(err))
	}
}

func (p *boundedProcessor) diagnostics(ready bool) pkgobservability.Diagnostics {
	return pkgobservability.Diagnostics{Enabled: true, Ready: ready, QueueDepth: int64(len(p.queue)), DroppedSpans: p.dropped.Load(), ExportedSpans: p.exported.Load(), LastErrorType: p.lastError.Load().(string)}
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

func resolveOperation(method, requestPath string, operations []pkgobservability.Operation) string {
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

func errorType(err error) string {
	if err == nil {
		return ""
	}
	return reflect.TypeOf(err).String()
}

func sanitize(_ error) error { return errors.New("telemetry exporter operation failed") }

var _ pkgobservability.Telemetry = (*telemetryAccess)(nil)
var _ sdktrace.SpanProcessor = (*boundedProcessor)(nil)
