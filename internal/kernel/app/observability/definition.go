// Package observability 封装进程级 Prometheus 与 generation-owned OpenTelemetry 实现。
package observability

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	pkgobservability "github.com/rin721/go-scaffold-template/pkg/observability"
)

const (
	MetricsID   app.ID = "observability.metrics"
	TelemetryID app.ID = "observability.telemetry"
)

// MetricsDefinition 返回进程期稳定且不暴露 registry 的 Metrics 声明。
func MetricsDefinition() (app.Definition[pkgobservability.Metrics], error) {
	return app.ManagedFixed(MetricsID, struct{}{}, app.FixedDependencies(struct{}{}), buildMetrics, app.Leased(newMetricsAccess))
}

// TelemetryDefinition 返回由 Application Generation 持有的 Telemetry 声明。
func TelemetryDefinition(metrics pkgobservability.Metrics) (app.Definition[pkgobservability.Telemetry], error) {
	recorder, ok := metrics.(metricsRecorder)
	if !ok || recorder == nil {
		return app.Definition[pkgobservability.Telemetry]{}, fmt.Errorf("observability metrics capability is incompatible")
	}
	source, err := app.Configured(ConfigPath, decode, defaults{})
	if err != nil {
		return app.Definition[pkgobservability.Telemetry]{}, err
	}
	return app.ManagedConfigured(
		TelemetryID, source, app.FixedDependencies(recorder), buildTelemetry,
		app.Leased(newTelemetryAccess), app.RestartRequired,
		app.WithStart(startTelemetry), app.WithTerminalFinalizer(stopTelemetry),
	)
}
