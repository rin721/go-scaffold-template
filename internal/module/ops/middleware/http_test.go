package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	prometheusadapter "github.com/rin721/go-scaffold-template/internal/module/ops/adapter/prometheus"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestHTTPUsesStableOperationAndPropagatesTraceContext(t *testing.T) {
	metrics, err := prometheusadapter.New()
	if err != nil {
		t.Fatal(err)
	}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	var traceID string
	handler := HTTP(provider.Tracer("test"), metrics, []Operation{{ID: "getTodo", Method: http.MethodGet, Path: "/api/v1/todos/{id}"}})(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		traceID, _ = httpx.TraceIDFromContext(request.Context())
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/todos/secret-object-id", nil)
	request.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if traceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace id = %q", traceID)
	}
	recorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	payload, _ := io.ReadAll(recorder.Result().Body)
	text := string(payload)
	if !strings.Contains(text, `operation="getTodo"`) || strings.Contains(text, "secret-object-id") {
		t.Fatalf("metrics payload leaked or missed operation:\n%s", text)
	}
}
