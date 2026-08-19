package execution

import (
	"context"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgexecution "github.com/rin721/go-scaffold-template/pkg/execution"
	"github.com/rin721/go-scaffold-template/pkg/resilience"
)

func TestBuildMemoryReturnsExecutor(t *testing.T) {
	resource, err := build(context.Background(), Config{
		Driver:           DriverMemory,
		RetryMaxAttempts: 2,
		RetryInitialWait: 10,
		RetryMaxWait:     50,
	}, struct{}{})
	if err != nil {
		t.Fatalf("build memory: %v", err)
	}
	defer func() { _ = stop(context.Background(), resource) }()
	if resource == nil || resource.driver != DriverMemory || resource.executor == nil {
		t.Fatalf("memory resource unexpected: %+v", resource)
	}
	if resource.recovering == nil {
		t.Fatalf("memory resource should carry a RecoveringStore")
	}
	if snap := resource.recovering.Snapshot(); snap.State != pkgexecution.StateHealthy {
		t.Fatalf("recovering snapshot=%+v want healthy", snap)
	}
}

func TestBuildDisabledReturnsNilExecutor(t *testing.T) {
	resource, err := build(context.Background(), Config{Driver: DriverDisabled}, struct{}{})
	if err != nil {
		t.Fatalf("build disabled: %v", err)
	}
	if resource == nil || resource.executor != nil {
		t.Fatalf("disabled resource should have nil executor: %+v", resource)
	}
	if resource.recovering != nil {
		t.Fatalf("disabled resource should not carry recovering store")
	}
}

func TestBuildUnsupportedDriver(t *testing.T) {
	if _, err := build(context.Background(), Config{Driver: "bogus"}, struct{}{}); err == nil {
		t.Fatal("unsupported driver should error")
	}
}

func TestStopIsIdempotent(t *testing.T) {
	resource, err := build(context.Background(), Config{Driver: DriverMemory}, struct{}{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := stop(context.Background(), resource); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := stop(context.Background(), resource); err != nil {
		t.Fatalf("second stop should be idempotent: %v", err)
	}
}

func TestDefaultConfigDriverIsMemory(t *testing.T) {
	cfg := defaultConfig()
	if cfg.Driver != DriverMemory {
		t.Fatalf("default driver=%q want memory", cfg.Driver)
	}
	if cfg.Recovery.Overflow != pkgexecution.OverflowDiscard {
		t.Fatalf("default overflow=%q want discard", cfg.Recovery.Overflow)
	}
}

func TestApplyPolicyNamed(t *testing.T) {
	r := &resource{
		defaultPolicy: retryPolicy(3, 50, 500),
		policies: map[string]policySpec{
			"payment": {retry: retryPolicy(5, 100, 1000), timeout: 2 * time.Second},
		},
	}
	exec := pkgexecution.Execution{PolicyName: "payment"}
	if err := r.applyPolicy(&exec); err != nil {
		t.Fatalf("applyPolicy: %v", err)
	}
	if exec.Policy.MaxAttempts != 5 || exec.Policy.InitialWait != 100*time.Millisecond || exec.Timeout != 2*time.Second {
		t.Fatalf("policy resolved %+v timeout=%v", exec.Policy, exec.Timeout)
	}
}

func TestApplyPolicyUnknownNameErrors(t *testing.T) {
	r := &resource{policies: map[string]policySpec{}}
	exec := pkgexecution.Execution{PolicyName: "missing"}
	if err := r.applyPolicy(&exec); err == nil {
		t.Fatal("unknown policy name should error")
	}
}

func TestApplyPolicyFallsBackToDefault(t *testing.T) {
	r := &resource{defaultPolicy: retryPolicy(3, 50, 500)}
	exec := pkgexecution.Execution{}
	if err := r.applyPolicy(&exec); err != nil {
		t.Fatalf("applyPolicy: %v", err)
	}
	if exec.Policy.MaxAttempts != 3 {
		t.Fatalf("policy=%+v want default max attempts 3", exec.Policy)
	}
}

func TestApplyPolicyKeepsExplicitPolicy(t *testing.T) {
	r := &resource{defaultPolicy: retryPolicy(3, 50, 500)}
	exec := pkgexecution.Execution{Policy: resilience.RetryPolicy{MaxAttempts: 7, InitialWait: time.Second, MaxWait: time.Second}}
	if err := r.applyPolicy(&exec); err != nil {
		t.Fatalf("applyPolicy: %v", err)
	}
	if exec.Policy.MaxAttempts != 7 {
		t.Fatalf("explicit policy overwritten: %+v", exec.Policy)
	}
}

func TestDecodeAppliesOverflow(t *testing.T) {
	cfg, err := decodeCfg(t, map[string]any{
		"driver":   "memory",
		"recovery": map[string]any{"overflow": "block"},
	})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Recovery.Overflow != pkgexecution.OverflowBlock {
		t.Fatalf("overflow=%q want block", cfg.Recovery.Overflow)
	}
}

func TestDecodeRejectsBadOverflow(t *testing.T) {
	if _, err := decodeCfg(t, map[string]any{
		"driver":   "memory",
		"recovery": map[string]any{"overflow": "bogus"},
	}); err == nil {
		t.Fatal("bad overflow should error")
	}
}

func TestDecodeRejectsNegativeRecovery(t *testing.T) {
	if _, err := decodeCfg(t, map[string]any{
		"driver":   "memory",
		"recovery": map[string]any{"bufferCapacity": -1},
	}); err == nil {
		t.Fatal("negative recovery value should error")
	}
}

func TestDecodeRejectsBadPolicy(t *testing.T) {
	if _, err := decodeCfg(t, map[string]any{
		"driver":   "memory",
		"policies": map[string]any{"pay": map[string]any{"retryMaxAttempts": -1}},
	}); err == nil {
		t.Fatal("negative policy value should error")
	}
}

func TestDecodeNormalizesDriver(t *testing.T) {
	cfg, err := decodeCfg(t, map[string]any{"driver": "  MEMORY  "})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Driver != DriverMemory {
		t.Fatalf("driver=%q want memory", cfg.Driver)
	}
}

// decodeCfg 用给定 execution 配置段构建真实快照并交给 decode，覆盖其归一化与校验逻辑。
func decodeCfg(t *testing.T, section map[string]any) (Config, error) {
	t.Helper()
	snapshot, err := config.New(config.MapSource("test", map[string]any{"execution": section})).Load(context.Background())
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	return decode(snapshot)
}
