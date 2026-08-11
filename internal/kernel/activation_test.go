package kernel

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

func TestActivationPublishesOnlyCommittedLoggerAndRestoresBaseline(t *testing.T) {
	baseline := pkglogger.NewTestLogger()
	manager, err := kernellogging.New(baseline)
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	source := &mutableSource{values: loggerTransactionValues("v1", "v1")}
	runtime, err := New(config.New(source), Options{Logging: manager})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	hooks := newManagedLoggerHooks(manager)
	if _, err := Register(runtime, hooks.definition()); err != nil {
		t.Fatalf("Register(logger) error = %v", err)
	}
	registerTestComponent(t, runtime, "service", &eventLog{}, func(_ context.Context, cfg testConfig) error {
		if cfg.Version == "bad" {
			return errors.New("service build failed")
		}
		return nil
	})
	if err := runtime.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	manager.Info("v1 active")

	source.set(loggerTransactionValues("v2", "bad"))
	if _, err := runtime.Reload(t.Context()); err == nil {
		t.Fatal("Reload(failed) error = nil")
	}
	manager.Info("v1 retained")
	if hooks.stopCount("v2") != 1 {
		t.Fatalf("discarded v2 stop count = %d, want 1", hooks.stopCount("v2"))
	}

	source.set(loggerTransactionValues("v2", "v2"))
	result, err := runtime.Reload(t.Context())
	if err != nil {
		t.Fatalf("Reload(success) error = %v", err)
	}
	if !result.Applied {
		t.Fatalf("Reload(success) result = %#v", result)
	}
	manager.Info("v2 active")
	if hooks.stopCount("v1") != 1 {
		t.Fatalf("old v1 stop count = %d, want 1", hooks.stopCount("v1"))
	}

	if err := runtime.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	manager.Info("baseline restored")
	if hooks.stopCount("v2") != 2 {
		t.Fatalf("all v2 stop count = %d, want candidate and current", hooks.stopCount("v2"))
	}
	if got := entryMessages(baseline.Entries()); !containsString(got, "baseline restored") {
		t.Fatalf("baseline messages = %#v", got)
	}
	if got := entryMessages(hooks.logger("v1").Entries()); !containsString(got, "v1 retained") {
		t.Fatalf("v1 messages = %#v, want failed reload to retain v1", got)
	}
	if got := entryMessages(hooks.logger("v2").Entries()); !containsString(got, "v2 active") {
		t.Fatalf("v2 messages = %#v", got)
	}
}

type loggerVersionConfig struct {
	Version string `mapstructure:"version"`
}

type managedLoggerHooks struct {
	mu      sync.Mutex
	manager *kernellogging.Manager
	loggers map[string]*pkglogger.TestLogger
	stops   map[string]int
}

func newManagedLoggerHooks(manager *kernellogging.Manager) *managedLoggerHooks {
	return &managedLoggerHooks{
		manager: manager,
		loggers: make(map[string]*pkglogger.TestLogger),
		stops:   make(map[string]int),
	}
}

func (h *managedLoggerHooks) definition() Definition[loggerVersionConfig, pkglogger.Logger] {
	return Definition[loggerVersionConfig, pkglogger.Logger]{
		ID:         "logger",
		ConfigPath: "logger",
		Decode: func(snapshot config.Snapshot) (loggerVersionConfig, error) {
			var cfg loggerVersionConfig
			if err := snapshot.DecodeSection("logger", &cfg); err != nil {
				return loggerVersionConfig{}, err
			}
			return cfg, nil
		},
		Defaults: config.DefaultContractFunc(func(context.Context) (config.Object, config.Control, error) {
			return config.Object{config.FieldOf("version", config.String("v1"))}, config.Continue, nil
		}),
		Builder:    h,
		Hooks:      h,
		Activation: h,
	}
}

func (h *managedLoggerHooks) Build(_ context.Context, cfg loggerVersionConfig) (pkglogger.Logger, error) {
	created := pkglogger.NewTestLogger()
	h.mu.Lock()
	h.loggers[cfg.Version] = created
	h.mu.Unlock()
	return created, nil
}

func (*managedLoggerHooks) Start(context.Context, pkglogger.Logger) error { return nil }

func (h *managedLoggerHooks) Stop(_ context.Context, logger pkglogger.Logger) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for version, candidate := range h.loggers {
		if candidate == logger {
			h.stops[version]++
			return nil
		}
	}
	return errors.New("unknown logger")
}

func (h *managedLoggerHooks) Activate(logger pkglogger.Logger) { h.manager.Replace(logger) }
func (h *managedLoggerHooks) Deactivate(pkglogger.Logger)      { h.manager.Restore() }

func (h *managedLoggerHooks) stopCount(version string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stops[version]
}

func (h *managedLoggerHooks) logger(version string) *pkglogger.TestLogger {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.loggers[version]
}

func loggerTransactionValues(loggerVersion, serviceVersion string) map[string]any {
	return map[string]any{
		"logger":  map[string]any{"version": loggerVersion},
		"service": map[string]any{"version": serviceVersion},
	}
}

func entryMessages(entries []pkglogger.Entry) []string {
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry.Message)
	}
	return result
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
