package observability_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rin721/go-scaffold-template/pkg/observability"
)

type metricsFixture struct{ handler http.Handler }

func (m metricsFixture) Handler() http.Handler { return m.handler }

type telemetryFixture struct{ diagnostics observability.Diagnostics }

func (t telemetryFixture) HTTP([]observability.Operation) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}
func (t telemetryFixture) Diagnostics(context.Context) (observability.Diagnostics, error) {
	return t.diagnostics, nil
}
func (t telemetryFixture) Observe(ctx context.Context, _ observability.Work, run observability.WorkFunc) error {
	return run(ctx)
}

func TestCapabilitiesCanBeConsumedWithoutTechnologyTypes(t *testing.T) {
	capabilities := observability.Capabilities{
		Metrics:   metricsFixture{handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) })},
		Telemetry: telemetryFixture{diagnostics: observability.Diagnostics{Enabled: true, Ready: true}},
	}
	recorder := httptest.NewRecorder()
	capabilities.Metrics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("metrics status = %d", recorder.Code)
	}
	diagnostics, err := capabilities.Telemetry.Diagnostics(t.Context())
	if err != nil {
		t.Fatalf("Diagnostics() error = %v", err)
	}
	if !diagnostics.Enabled || !diagnostics.Ready {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}
