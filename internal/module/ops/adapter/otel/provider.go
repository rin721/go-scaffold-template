// Package oteladapter 封装 generation-owned OpenTelemetry trace provider 与有界 exporter 队列。
package oteladapter

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	prometheusadapter "github.com/rin721/go-scaffold-template/internal/module/ops/adapter/prometheus"
	configbinding "github.com/rin721/go-scaffold-template/internal/module/ops/binding/config"
	"github.com/rin721/go-scaffold-template/internal/module/ops/model"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Provider 是 Ops module 向 composition 暴露的 trace 完成品。
type Provider struct {
	enabled   bool
	provider  *sdktrace.TracerProvider
	processor *boundedProcessor
	tracer    trace.Tracer
	started   atomic.Bool
	stopOnce  sync.Once
	stopErr   error
}

// New 创建尚未启动 exporter worker 的 trace provider。
func New(ctx context.Context, config configbinding.Observability, metrics *prometheusadapter.Registry) (*Provider, error) {
	if ctx == nil || metrics == nil {
		return nil, fmt.Errorf("OpenTelemetry dependencies are incomplete")
	}
	if !config.Tracing.Enabled {
		provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()))
		return &Provider{provider: provider, tracer: provider.Tracer(config.ServiceName)}, nil
	}
	options := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(config.Tracing.Endpoint), otlptracehttp.WithTimeout(config.Tracing.ExportTimeout)}
	if config.Tracing.Insecure {
		options = append(options, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("create OTLP HTTP exporter: %w", sanitize(err))
	}
	processor := newBoundedProcessor(exporter, config.Tracing, metrics)
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.Tracing.SampleRatio)),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", config.ServiceName))),
		sdktrace.WithSpanProcessor(processor),
	)
	return &Provider{enabled: true, provider: provider, processor: processor, tracer: provider.Tracer(config.ServiceName)}, nil
}

// Name 返回 generation participant 的稳定 owner ID。
func (*Provider) Name() string { return "ops-telemetry" }

// Start 启动有界 exporter worker；未启用 trace 时保持无副作用 Ready。
func (p *Provider) Start(ctx context.Context) error {
	if p == nil || p.provider == nil || ctx == nil {
		return fmt.Errorf("OpenTelemetry provider is invalid")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !p.enabled {
		p.started.Store(true)
		return nil
	}
	if !p.started.CompareAndSwap(false, true) {
		return fmt.Errorf("OpenTelemetry provider is already started")
	}
	p.processor.start()
	return nil
}

// Stop 在调用方总预算内 flush 并关闭 exporter。
func (p *Provider) Stop(ctx context.Context) error {
	if p == nil || p.provider == nil {
		return nil
	}
	p.stopOnce.Do(func() {
		if ctx == nil {
			p.stopErr = fmt.Errorf("OpenTelemetry stop context is nil")
			return
		}
		p.stopErr = errors.Join(p.provider.ForceFlush(ctx), p.provider.Shutdown(ctx))
		p.started.Store(false)
	})
	return p.stopErr
}

func (p *Provider) Tracer() trace.Tracer { return p.tracer }

// Diagnostics 返回队列、丢弃和 exporter failure 的低敏快照。
func (p *Provider) Diagnostics() model.TelemetryDiagnostics {
	if p == nil {
		return model.TelemetryDiagnostics{}
	}
	if !p.enabled {
		return model.TelemetryDiagnostics{Ready: true}
	}
	return p.processor.diagnostics(p.started.Load())
}

type boundedProcessor struct {
	exporter  sdktrace.SpanExporter
	config    configbinding.Tracing
	metrics   *prometheusadapter.Registry
	queue     chan sdktrace.ReadOnlySpan
	flush     chan chan error
	stop      chan struct{}
	done      chan struct{}
	startOnce sync.Once
	dropped   atomic.Uint64
	exported  atomic.Uint64
	lastError atomic.Value
}

func newBoundedProcessor(exporter sdktrace.SpanExporter, config configbinding.Tracing, metrics *prometheusadapter.Registry) *boundedProcessor {
	processor := &boundedProcessor{exporter: exporter, config: config, metrics: metrics, queue: make(chan sdktrace.ReadOnlySpan, config.QueueSize), flush: make(chan chan error), stop: make(chan struct{}), done: make(chan struct{})}
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
		p.metrics.SetQueueDepth(int64(len(p.queue)))
	default:
		p.recordDropped(1)
	}
}

func (p *boundedProcessor) ForceFlush(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("OpenTelemetry flush context is nil")
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
		return fmt.Errorf("OpenTelemetry shutdown context is nil")
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
			p.metrics.SetQueueDepth(int64(len(p.queue)))
			if len(batch) >= p.config.BatchSize {
				_ = p.export(batch)
				batch = batch[:0]
			}
		case response := <-p.flush:
			batch = p.drain(batch)
			response <- p.export(batch)
			batch = batch[:0]
		case <-ticker.C:
			_ = p.export(batch)
			batch = batch[:0]
		case <-p.stop:
			batch = p.drain(batch)
			exportErr := p.export(batch)
			shutdownCtx, cancel := context.WithTimeout(context.Background(), p.config.ShutdownTimeout)
			shutdownErr := p.exporter.Shutdown(shutdownCtx)
			cancel()
			if exportErr != nil || shutdownErr != nil {
				p.lastError.Store(errorType(errors.Join(exportErr, shutdownErr)))
			}
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
			p.metrics.SetQueueDepth(0)
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
		p.lastError.Store(errorType(err))
		p.recordDropped(len(batch))
		return sanitize(err)
	}
	p.exported.Add(uint64(len(batch)))
	p.metrics.RecordExported(len(batch))
	return nil
}

func (p *boundedProcessor) recordDropped(count int) {
	if count <= 0 {
		return
	}
	// #nosec G115 -- count 来自内存队列长度，已证明为正 int，转换不会截断。
	p.dropped.Add(uint64(count))
	p.metrics.RecordDropped(count)
}
func (p *boundedProcessor) diagnostics(ready bool) model.TelemetryDiagnostics {
	return model.TelemetryDiagnostics{Enabled: true, Ready: ready, QueueDepth: int64(len(p.queue)), DroppedSpans: p.dropped.Load(), ExportedSpans: p.exported.Load(), LastErrorType: p.lastError.Load().(string)}
}
func errorType(err error) string {
	if err == nil {
		return ""
	}
	return reflect.TypeOf(err).String()
}
func sanitize(error) error { return errors.New("telemetry exporter operation failed") }

var _ sdktrace.SpanProcessor = (*boundedProcessor)(nil)
