package kernel

import (
	"context"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

type testLoggingAccess struct{ logger pkglogger.Logger }

func (a testLoggingAccess) Use(ctx context.Context, use func(pkglogger.Logger) error) error {
	if ctx == nil {
		return ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if use == nil {
		return nil
	}
	return use(a.logger)
}

func newTestLoggingAccess(t *testing.T) pkglogger.Access {
	t.Helper()
	return testLoggingAccess{logger: pkglogger.Noop()}
}

func TestNewRequiresLoggingAccess(t *testing.T) {
	if runtime, err := New(config.New(config.MapSource("empty", map[string]any{})), Options{}); err == nil || runtime != nil {
		t.Fatalf("New(without logging) = %#v, %v; want nil, error", runtime, err)
	}
}
