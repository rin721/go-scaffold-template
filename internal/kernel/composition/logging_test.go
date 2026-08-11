package composition

import (
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

func newTestRuntime(t *testing.T, source config.Source) *kernel.Kernel {
	t.Helper()
	manager, err := kernellogging.New(pkglogger.Noop())
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}
	runtime, err := kernel.New(config.New(source), kernel.Options{Logging: manager})
	if err != nil {
		t.Fatalf("kernel.New() error = %v", err)
	}
	return runtime
}
