package schedule

import (
	"context"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgschedule "github.com/rin721/go-scaffold-template/pkg/schedule"
)

func TestDecodeSchedulerConfiguration(t *testing.T) {
	enabled := true
	value, err := decodeSchedulerConfig(t, map[string]any{
		"enabled":             true,
		"timezone":            "Asia/Shanghai",
		"maxConcurrency":      7,
		"shutdownTimeout":     "4s",
		"occurrenceRetention": "12h",
		"coordination": map[string]any{
			"namespace": " company:scheduler ", "leaseTTL": "30s", "renewInterval": "8s",
			"acquireTimeout": "2s", "retryMin": "100ms", "retryMax": "2s",
		},
		"tasks": map[string]any{
			"billing.reconcile": map[string]any{"enabled": enabled, "unavailablePolicy": "pause"},
		},
	})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !value.Enabled || value.Timezone != "Asia/Shanghai" || value.MaxConcurrency != 7 {
		t.Fatalf("scheduler config=%+v", value)
	}
	if value.Coordination.Namespace != "company:scheduler" {
		t.Fatalf("namespace=%q want company:scheduler", value.Coordination.Namespace)
	}
	override := value.Tasks["billing.reconcile"]
	if override.Enabled == nil || !*override.Enabled || override.UnavailablePolicy != pkgschedule.UnavailablePause {
		t.Fatalf("task override=%+v", override)
	}
}

func TestDecodeRejectsUnsafeLeaseWindow(t *testing.T) {
	_, err := decodeSchedulerConfig(t, map[string]any{
		"coordination": map[string]any{"leaseTTL": "9s", "renewInterval": "4s"},
	})
	if err == nil {
		t.Fatal("renew interval without two safety windows should fail")
	}
}

func TestNormalizeRejectsUnknownUnavailablePolicy(t *testing.T) {
	cfg := defaultConfig()
	cfg.Tasks = map[string]TaskConfig{"billing.reconcile": {UnavailablePolicy: "fallback"}}
	if _, err := normalizeConfig(cfg); err == nil {
		t.Fatal("unknown unavailable policy should fail")
	}
}

func TestNormalizeRejectsUnboundedOperationalValues(t *testing.T) {
	cfg := defaultConfig()
	cfg.MaxConcurrency = maxConfiguredConcurrency + 1
	if _, err := normalizeConfig(cfg); err == nil {
		t.Fatal("unbounded max concurrency should fail")
	}
	cfg = defaultConfig()
	cfg.Coordination.LeaseTTL = maxLeaseTTL + time.Second
	if _, err := normalizeConfig(cfg); err == nil {
		t.Fatal("unbounded lease ttl should fail")
	}
}

func TestDefaultsKeepSchedulerDisabled(t *testing.T) {
	cfg := defaultConfig()
	if cfg.Enabled {
		t.Fatal("scheduler must be disabled unless deployment explicitly enables it")
	}
	if cfg.OccurrenceRetention != 24*time.Hour || cfg.Timezone != "UTC" {
		t.Fatalf("defaults=%+v", cfg)
	}
}

func decodeSchedulerConfig(t *testing.T, section map[string]any) (Config, error) {
	t.Helper()
	snapshot, err := config.New(config.MapSource("test", map[string]any{"scheduler": section})).Load(context.Background())
	if err != nil {
		t.Fatalf("load config snapshot: %v", err)
	}
	return decode(snapshot)
}
