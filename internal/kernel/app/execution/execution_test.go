package execution

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgexecution "github.com/rin721/go-scaffold-template/pkg/execution"
	"github.com/rin721/go-scaffold-template/pkg/health"
	"github.com/rin721/go-scaffold-template/pkg/logger"
	"github.com/rin721/go-scaffold-template/pkg/resilience"
)

func TestBuildMemoryReturnsExecutor(t *testing.T) {
	resource, err := build(context.Background(), Config{
		Driver:           DriverMemory,
		RetryMaxAttempts: 2,
		RetryInitialWait: 10,
		RetryMaxWait:     50,
	}, componentDeps{logger: logger.Noop()})
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
	resource, err := build(context.Background(), Config{Driver: DriverDisabled}, componentDeps{})
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
	if _, err := build(context.Background(), Config{Driver: "bogus"}, componentDeps{}); err == nil {
		t.Fatal("unsupported driver should error")
	}
}

func TestStopIsIdempotent(t *testing.T) {
	resource, err := build(context.Background(), Config{Driver: DriverMemory}, componentDeps{logger: logger.Noop()})
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

func TestAccessRecoveryAndHealth(t *testing.T) {
	resource, err := build(context.Background(), Config{Driver: DriverMemory}, componentDeps{logger: logger.Noop()})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer func() { _ = stop(context.Background(), resource) }()
	acc := &access{delegate: &fakeLease{current: resource}}
	snap, err := acc.Recovery()
	if err != nil {
		t.Fatalf("Recovery: %v", err)
	}
	if snap.State != pkgexecution.StateHealthy {
		t.Fatalf("recovery state=%q want healthy", snap.State)
	}
	result, err := acc.Health()
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if result.Status != health.StatusPass {
		t.Fatalf("health status=%q want pass", result.Status)
	}
}

func TestAccessHealthDisabledFails(t *testing.T) {
	resource, err := build(context.Background(), Config{Driver: DriverDisabled}, componentDeps{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	acc := &access{delegate: &fakeLease{current: resource}}
	result, err := acc.Health()
	if err != nil {
		t.Fatalf("Health disabled: %v", err)
	}
	if result.Status != health.StatusFail {
		t.Fatalf("disabled health status=%q want fail", result.Status)
	}
}

func TestLogTransitionEmitsWarnOnDegraded(t *testing.T) {
	rec := logger.NewTestLogger()
	cb := logTransition(rec)
	cb(pkgexecution.StateHealthy, pkgexecution.StateDegraded)
	cb(pkgexecution.StateDegraded, pkgexecution.StateHealthy)
	var warn, info int
	for _, entry := range rec.Entries() {
		switch entry.Level {
		case "warn":
			warn++
		case "info":
			info++
		}
	}
	if warn < 1 || info < 1 {
		t.Fatalf("warn=%d info=%d want >=1 each", warn, info)
	}
}

func TestLogTransitionNilLoggerNoop(t *testing.T) {
	if fn := logTransition(nil); fn != nil {
		t.Fatal("nil logger should produce nil transition callback")
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

// fakeLease 以固定实例返回给 Use 测试用。
type fakeLease struct{ current *resource }

func (l *fakeLease) Use(ctx context.Context, fn func(*resource) error) error {
	return fn(l.current)
}

// faultyStore 是可按开关注入主存储故障的测试 Store，内存作为真实持久化承载，便于断言回放。
type faultyStore struct {
	inner *pkgexecution.MemoryStore
	fail  atomic.Bool
}

func (f *faultyStore) setFail(v bool) { f.fail.Store(v) }

func (f *faultyStore) Claim(ctx context.Context, key pkgexecution.Key, ttl time.Duration, now time.Time) (bool, error) {
	if f.fail.Load() {
		return false, errors.New("primary: backend down")
	}
	return f.inner.Claim(ctx, key, ttl, now)
}

func (f *faultyStore) IsCompleted(ctx context.Context, key pkgexecution.Key) (bool, error) {
	if f.fail.Load() {
		return false, errors.New("primary: backend down")
	}
	return f.inner.IsCompleted(ctx, key)
}

func (f *faultyStore) Complete(ctx context.Context, key pkgexecution.Key, rec pkgexecution.Record) error {
	if f.fail.Load() {
		return errors.New("primary: backend down")
	}
	return f.inner.Complete(ctx, key, rec)
}

func (f *faultyStore) Record(ctx context.Context, key pkgexecution.Key, rec pkgexecution.Record) error {
	if f.fail.Load() {
		return errors.New("primary: backend down")
	}
	return f.inner.Record(ctx, key, rec)
}

func (f *faultyStore) Records(ctx context.Context, key pkgexecution.Key) ([]pkgexecution.Record, error) {
	return f.inner.Records(ctx, key)
}

// TestAccessDegradeRecoverEndToEnd 验证经 Access 装配的完整链路：
// 主存储故障→降级本地有限能力→后台自动恢复探测→回放→原子切回→幂等去重回到主存储，
// 并验证 Recovery() 状态与 Health()/日志随状态变化输出。
func TestAccessDegradeRecoverEndToEnd(t *testing.T) {
	primary := &faultyStore{inner: pkgexecution.NewMemoryStore()}
	primary.setFail(true)
	logs := logger.NewTestLogger()
	cfg := Config{
		Driver:           DriverMemory,
		RetryMaxAttempts: 3,
		RetryInitialWait: 1,
		RetryMaxWait:     5,
		Recovery: RecoveryConfig{
			ProbeIntervalMs:  5,
			InitialBackoffMs: 1,
			MaxBackoffMs:     50,
			VerifyAttempts:   1,
			BufferCapacity:   100,
			Overflow:         pkgexecution.OverflowDiscard,
		},
	}
	resource, err := assemble(cfg, componentDeps{logger: logs}, primary, pkgexecution.NewMemoryStore())
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	defer func() { _ = stop(context.Background(), resource) }()
	acc := &access{delegate: &fakeLease{current: resource}}

	// 主存储故障：成功操作经本地有限能力完成，并升入 Degraded。
	okKey := pkgexecution.Key("pay:ok")
	res, err := acc.Execute(context.Background(), pkgexecution.Execution{
		Key: okKey, Operation: func(context.Context) (any, error) { return "ok", nil },
	})
	if err != nil {
		t.Fatalf("execute under degradation: %v", err)
	}
	if res.Status != pkgexecution.StatusCompleted {
		t.Fatalf("result=%+v want completed", res)
	}
	if snap, _ := acc.Recovery(); snap.State != pkgexecution.StateDegraded {
		t.Fatalf("recovery=%+v want degraded", snap)
	}
	if result, _ := acc.Health(); result.Status != health.StatusWarn {
		t.Fatalf("degraded health=%q want warn", result.Status)
	}

	// 主存储恢复：等待后台探测自动切回 Healthy 并回放缓冲。
	primary.setFail(false)
	waitState(t, acc, pkgexecution.StateHealthy, 2*time.Second)
	if got, _ := primary.Records(context.Background(), "pay:ok"); len(got) != 1 {
		t.Fatalf("primary recovered records=%d want 1 replayed", len(got))
	}
	if result, _ := acc.Health(); result.Status != health.StatusPass {
		t.Fatalf("recovered health=%q want pass", result.Status)
	}

	// 恢复后幂等去重回到主存储：重复提交不重跑。
	var calls int32
	res, err = acc.Execute(context.Background(), pkgexecution.Execution{
		Key: okKey, Operation: func(context.Context) (any, error) {
			atomic.AddInt32(&calls, 1)
			return "ok", nil
		},
	})
	if err != nil {
		t.Fatalf("duplicate execute: %v", err)
	}
	if !res.Duplicate || res.Status != pkgexecution.StatusCompleted {
		t.Fatalf("duplicate result=%+v want Duplicate completed", res)
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Fatalf("operation should not rerun on duplicate, calls=%d", calls)
	}

	// 状态变化应已输出 Warn（degraded）与 Info（recovered）日志。
	var warn, info int
	for _, entry := range logs.Entries() {
		switch entry.Level {
		case "warn":
			warn++
		case "info":
			info++
		}
	}
	if warn < 1 || info < 1 {
		t.Fatalf("logs warn=%d info=%d want >=1 each", warn, info)
	}
}

func waitState(t *testing.T, acc *access, want pkgexecution.RecoveryState, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snap, err := acc.Recovery()
		if err == nil && snap.State == want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("recovery state not %q within %s", want, timeout)
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
