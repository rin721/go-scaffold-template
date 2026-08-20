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
	"github.com/rin721/go-scaffold-template/pkg/coordination"
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
	if server.CurrentConnectionCount() == 0 {
		t.Fatal("Redis connection was not established")
	}
	if err := stop(context.Background(), current); err != nil {
		t.Fatalf("stop() error = %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for server.CurrentConnectionCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if server.CurrentConnectionCount() != 0 {
		t.Fatalf("Redis connections after stop = %d, want 0", server.CurrentConnectionCount())
	}
}

func TestRedisCoordinationEnforcesTokenOwnershipAndTTL(t *testing.T) {
	server := miniredis.RunT(t)
	cfg := defaultConfig()
	cfg.Driver = DriverRedis
	cfg.Redis.Address = server.Addr()
	current, err := build(t.Context(), cfg, struct{}{})
	if err != nil {
		t.Fatalf("build() error = %v", err)
	}
	t.Cleanup(func() { _ = stop(context.Background(), current) })
	backend, err := newAccess(fakeLease{current: current})
	if err != nil {
		t.Fatal(err)
	}
	first, err := Coordination(backend)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Coordination(backend)
	if err != nil {
		t.Fatal(err)
	}
	options := coordination.LeaseOptions{TTL: time.Second}
	lease, err := first.Acquire(t.Context(), "scheduler:test:billing.reconcile", options)
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	if _, err := second.Acquire(t.Context(), "scheduler:test:billing.reconcile", options); !errors.Is(err, coordination.ErrNotAcquired) {
		t.Fatalf("second Acquire() error = %v", err)
	}
	if err := lease.Renew(t.Context(), options); err != nil {
		t.Fatalf("Renew() error = %v", err)
	}
	server.Set("scheduler:test:billing.reconcile", "different-owner-token")
	if err := lease.Renew(t.Context(), options); !errors.Is(err, coordination.ErrLeaseLost) {
		t.Fatalf("Renew(after token change) error = %v", err)
	}
	if err := lease.Release(t.Context()); !errors.Is(err, coordination.ErrLeaseLost) {
		t.Fatalf("Release(after token change) error = %v", err)
	}
	server.Del("scheduler:test:billing.reconcile")
	lease, err = first.Acquire(t.Context(), "scheduler:test:billing.reconcile", options)
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(t.Context()); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if _, err := second.Acquire(t.Context(), "scheduler:test:billing.reconcile", options); err != nil {
		t.Fatalf("Acquire(after release) error = %v", err)
	}
}

func TestDisabledCoordinationIsExplicitlyUnavailable(t *testing.T) {
	backend, err := newAccess(fakeLease{current: &resource{driver: DriverDisabled, coordination: coordination.Unavailable()}})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := Coordination(backend)
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Acquire(t.Context(), "scheduler:test:task", coordination.LeaseOptions{TTL: time.Second})
	if !errors.Is(err, coordination.ErrUnavailable) {
		t.Fatalf("Acquire() error = %v", err)
	}
}

func TestRedisCoordinationClassifiesOutageAndRecoversWithoutRebuild(t *testing.T) {
	server := miniredis.RunT(t)
	cfg := defaultConfig()
	cfg.Driver = DriverRedis
	cfg.Redis.Address = server.Addr()
	cfg.Redis.DialTimeout = 25 * time.Millisecond
	cfg.Redis.ReadTimeout = 25 * time.Millisecond
	cfg.Redis.WriteTimeout = 25 * time.Millisecond
	current, err := build(t.Context(), cfg, struct{}{})
	if err != nil {
		t.Fatalf("build() error = %v", err)
	}
	t.Cleanup(func() { _ = stop(context.Background(), current) })
	backend, err := newAccess(fakeLease{current: current})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := Coordination(backend)
	if err != nil {
		t.Fatal(err)
	}
	server.Close()
	outageCtx, cancelOutage := context.WithTimeout(t.Context(), 2*time.Second)
	_, err = manager.Acquire(outageCtx, "scheduler:test:recover", coordination.LeaseOptions{TTL: time.Second})
	cancelOutage()
	if !errors.Is(err, coordination.ErrUnavailable) {
		t.Fatalf("Acquire(during outage) error = %v, want ErrUnavailable", err)
	}
	if err := server.Restart(); err != nil {
		t.Fatalf("Restart() error = %v", err)
	}
	recoveryCtx, cancelRecovery := context.WithTimeout(t.Context(), time.Second)
	defer cancelRecovery()
	lease, err := manager.Acquire(recoveryCtx, "scheduler:test:recover", coordination.LeaseOptions{TTL: time.Second})
	if err != nil {
		t.Fatalf("Acquire(after recovery) error = %v", err)
	}
	if err := lease.Release(recoveryCtx); err != nil {
		t.Fatalf("Release(after recovery) error = %v", err)
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
