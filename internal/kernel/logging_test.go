package kernel

import (
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

func newTestLoggingManager(t *testing.T) *kernellogging.Manager {
	t.Helper()
	manager, err := kernellogging.New(pkglogger.Noop())
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	return manager
}

func TestNewRequiresLoggingManager(t *testing.T) {
	if runtime, err := New(config.New(config.MapSource("empty", map[string]any{})), Options{}); err == nil || runtime != nil {
		t.Fatalf("New(without logging) = %#v, %v; want nil, error", runtime, err)
	}
}
