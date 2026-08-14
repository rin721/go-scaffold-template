package testkit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/pkg/clock"
	"github.com/rin721/go-scaffold-template/pkg/health"
)

// Clock 返回固定测试时钟。
func Clock(t *testing.T, now time.Time) clock.Clock {
	t.Helper()
	return clock.Fixed(now)
}

// TempConfigFile 写入临时 YAML 配置并返回路径。
func TempConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

// HealthyRegistry 创建带单个通过项的健康检查注册表。
func HealthyRegistry(t *testing.T, name string) *health.Registry {
	t.Helper()
	registry := health.New(time.Second)
	if err := registry.Register(name, func(context.Context) health.Result {
		return health.Result{Status: health.StatusPass}
	}); err != nil {
		t.Fatalf("register health check: %v", err)
	}
	return registry
}
