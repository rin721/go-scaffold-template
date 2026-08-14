package cache

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	kernelconfig "github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgcache "github.com/rin721/go-scaffold-template/pkg/cache"
)

func TestDefinitionContributesRestartRequiredCacheAccess(t *testing.T) {
	definition, err := Definition()
	if err != nil {
		t.Fatalf("Definition() error = %v", err)
	}
	plan := app.NewPlan()
	added, err := app.Add(plan, definition)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if added.Output == nil {
		t.Fatal("Cache Access is nil")
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	bindings := frozen.Configurations()
	if len(bindings) != 1 || bindings[0].CapabilityID != string(ID) || bindings[0].ConfigPath != ConfigPath {
		t.Fatalf("Configurations() = %#v", bindings)
	}
	components := frozen.Components()
	if len(components) != 1 || components[0].Policy() != app.RestartRequired {
		t.Fatalf("Components() = %#v", components)
	}
	accessType := reflect.TypeOf((*Access)(nil)).Elem()
	if _, exposed := accessType.MethodByName("Close"); exposed {
		t.Fatal("Cache Access exposes shared Close ownership")
	}
}

func TestDisabledAccessReturnsTypedError(t *testing.T) {
	backend, err := newAccess(fakeLease{current: &resource{driver: DriverDisabled}})
	if err != nil {
		t.Fatalf("newAccess() error = %v", err)
	}
	client, err := NewClient[string](backend, &pkgcache.Config{DefaultTTL: time.Minute})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Set(t.Context(), "key", "value"); !errors.Is(err, pkgcache.ErrDisabled) {
		t.Fatalf("Set() error = %v, want ErrDisabled", err)
	}
	if err := backend.Ping(t.Context()); !errors.Is(err, pkgcache.ErrDisabled) {
		t.Fatalf("Ping() error = %v, want ErrDisabled", err)
	}
}

func TestRedisResourceSupportsTypedClient(t *testing.T) {
	server := miniredis.RunT(t)
	cfg := defaultConfig()
	cfg.Driver = DriverRedis
	cfg.Redis.Address = server.Addr()
	current, err := build(t.Context(), cfg, struct{}{})
	if err != nil {
		t.Fatalf("build() error = %v", err)
	}
	t.Cleanup(func() { _ = stop(context.Background(), current) })
	if err := ready(t.Context(), current); err != nil {
		t.Fatalf("ready() error = %v", err)
	}
	backend, err := newAccess(fakeLease{current: current})
	if err != nil {
		t.Fatalf("newAccess() error = %v", err)
	}
	client, err := NewClient[string](backend, &pkgcache.Config{DefaultTTL: time.Minute})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := client.Set(t.Context(), "key", "value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value, err := client.Get(t.Context(), "key")
	if err != nil || value != "value" {
		t.Fatalf("Get() value=%q error=%v", value, err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestDecodeRejectsInvalidDriverWithoutLeakingPassword(t *testing.T) {
	loader := kernelconfig.New(kernelconfig.MapSource("test", map[string]any{
		"cache": map[string]any{
			"driver": "unknown",
			"redis":  map[string]any{"password": "do-not-print"},
		},
	}))
	snapshot, err := loader.Load(t.Context())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	_, err = decode(snapshot)
	if err == nil {
		t.Fatal("decode() error = nil")
	}
	if strings.Contains(err.Error(), "do-not-print") {
		t.Fatalf("decode() leaked password: %v", err)
	}
}

type fakeLease struct{ current *resource }

func (f fakeLease) Use(ctx context.Context, use func(*resource) error) error {
	if ctx == nil {
		return app.ErrNilContext
	}
	return use(f.current)
}
