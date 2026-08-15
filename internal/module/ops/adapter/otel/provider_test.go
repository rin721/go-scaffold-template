package oteladapter

import (
	"context"
	"errors"
	"testing"
	"time"

	prometheusadapter "github.com/rin721/go-scaffold-template/internal/module/ops/adapter/prometheus"
	configbinding "github.com/rin721/go-scaffold-template/internal/module/ops/binding/config"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type failingExporter struct{}

func (failingExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	return errors.New("secret exporter detail")
}
func (failingExporter) Shutdown(context.Context) error { return nil }

func TestBoundedProcessorCountsExporterFailureWithoutErrorText(t *testing.T) {
	metrics, err := prometheusadapter.New()
	if err != nil {
		t.Fatal(err)
	}
	config := configbinding.Default().Observability.Tracing
	config.QueueSize, config.BatchSize, config.ExportTimeout = 1, 1, time.Second
	processor := newBoundedProcessor(failingExporter{}, config, metrics)
	processor.start()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()), sdktrace.WithSpanProcessor(processor))
	_, span := provider.Tracer("test").Start(t.Context(), "operation")
	span.End()
	_ = processor.ForceFlush(t.Context())
	if err := processor.Shutdown(t.Context()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	diagnostics := processor.diagnostics(false)
	if diagnostics.DroppedSpans != 1 || diagnostics.LastErrorType == "" || diagnostics.LastErrorType == "secret exporter detail" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestBoundedProcessorDropsWithoutBlockingWhenQueueIsFull(t *testing.T) {
	metrics, err := prometheusadapter.New()
	if err != nil {
		t.Fatal(err)
	}
	config := configbinding.Default().Observability.Tracing
	config.QueueSize, config.BatchSize = 1, 1
	processor := newBoundedProcessor(failingExporter{}, config, metrics)
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()), sdktrace.WithSpanProcessor(processor))
	for range 2 {
		_, span := provider.Tracer("test").Start(t.Context(), "operation")
		span.End()
	}
	diagnostics := processor.diagnostics(false)
	if diagnostics.QueueDepth != 1 || diagnostics.DroppedSpans != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	processor.start()
	if err := processor.Shutdown(t.Context()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}
