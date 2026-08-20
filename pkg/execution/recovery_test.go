package execution

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/pkg/health"
)

// scriptedStore 包装 MemoryStore，可按开关模拟主存储读写失败。
type scriptedStore struct {
	*MemoryStore
	failAtomic atomic.Bool
}

func (s *scriptedStore) setFail(fail bool) { s.failAtomic.Store(fail) }

func (s *scriptedStore) Claim(ctx context.Context, key Key, ttl time.Duration, now time.Time) (bool, error) {
	if s.failAtomic.Load() {
		return false, errors.New("primary: backend down")
	}
	return s.MemoryStore.Claim(ctx, key, ttl, now)
}

func (s *scriptedStore) IsCompleted(ctx context.Context, key Key) (bool, error) {
	if s.failAtomic.Load() {
		return false, errors.New("primary: backend down")
	}
	return s.MemoryStore.IsCompleted(ctx, key)
}

func (s *scriptedStore) Complete(ctx context.Context, key Key, retentionTTL time.Duration, rec Record) error {
	if s.failAtomic.Load() {
		return errors.New("primary: backend down")
	}
	return s.MemoryStore.Complete(ctx, key, retentionTTL, rec)
}

func (s *scriptedStore) Release(ctx context.Context, key Key) error {
	if s.failAtomic.Load() {
		return errors.New("primary: backend down")
	}
	return s.MemoryStore.Release(ctx, key)
}

func (s *scriptedStore) Record(ctx context.Context, key Key, rec Record) error {
	if s.failAtomic.Load() {
		return errors.New("primary: backend down")
	}
	return s.MemoryStore.Record(ctx, key, rec)
}

func recoveryCfg() RecoveryConfig {
	return RecoveryConfig{
		ProbeInterval:  5 * time.Millisecond,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
		VerifyAttempts: 1,
		BufferCapacity: 100,
		Overflow:       OverflowDiscard,
		Jitter:         func(d time.Duration) time.Duration { return d },
	}
}

// waitFor 轮询直到 cond 为真或超时。
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func TestRecoveringHealthyRoutesPrimary(t *testing.T) {
	primary := &scriptedStore{MemoryStore: NewMemoryStore()}
	local := NewMemoryStore()
	rec := NewRecoveringStore(primary, local, recoveryCfg())

	if err := rec.Complete(context.Background(), "k", 0, Record{Key: "k", Status: StatusCompleted}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if snap := rec.Snapshot(); snap.State != StateHealthy || snap.Buffered != 0 {
		t.Fatalf("snapshot=%+v want healthy with empty buffer", snap)
	}
	// 主存储确实收到记录；本地为空。
	if got, _ := primary.Records(context.Background(), "k"); len(got) != 1 {
		t.Fatalf("primary records=%d want 1", len(got))
	}
	if got, _ := local.Records(context.Background(), "k"); len(got) != 0 {
		t.Fatalf("local records=%d want 0", len(got))
	}
}

func TestRecoveringDegradesAndServesLocal(t *testing.T) {
	primary := &scriptedStore{MemoryStore: NewMemoryStore()}
	primary.setFail(true)
	local := NewMemoryStore()
	rec := NewRecoveringStore(primary, local, recoveryCfg())

	if err := rec.Complete(context.Background(), "k", 0, Record{Key: "k", Status: StatusCompleted}); err != nil {
		t.Fatalf("complete under degradation: %v", err)
	}
	snap := rec.Snapshot()
	if snap.State != StateDegraded || snap.Buffered != 1 {
		t.Fatalf("snapshot=%+v want degraded with 1 buffered", snap)
	}
	// 本地已写入记录（有限能力）。
	if got, _ := local.Records(context.Background(), "k"); len(got) != 1 {
		t.Fatalf("local records=%d want 1", len(got))
	}
}

func TestRecoveringReplaysBufferOnRecovery(t *testing.T) {
	primary := &scriptedStore{MemoryStore: NewMemoryStore()}
	primary.setFail(true)
	rec := NewRecoveringStore(primary, NewMemoryStore(), recoveryCfg()).Start()
	defer rec.Stop()

	for i := 0; i < 3; i++ {
		key := Key("order:1")
		if err := rec.Record(context.Background(), key, Record{Key: key, Status: StatusFailed}); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}
	if snap := rec.Snapshot(); snap.State != StateDegraded || snap.Buffered != 3 {
		t.Fatalf("snapshot=%+v want degraded 3 buffered", snap)
	}

	// 主存储恢复：等待循环探测、验证、回放并原子切回 Healthy。
	primary.setFail(false)
	waitFor(t, 2*time.Second, func() bool {
		return rec.Snapshot().State == StateHealthy
	})
	snap := rec.Snapshot()
	if snap.Buffered != 0 || snap.Dropped != 0 || snap.Transitions < 2 {
		t.Fatalf("snapshot=%+v want drained with >=2 transitions", snap)
	}
	if got, _ := primary.Records(context.Background(), "order:1"); len(got) != 3 {
		t.Fatalf("primary records=%d want 3 replayed", len(got))
	}
}

func TestRecoveringOverflowDiscard(t *testing.T) {
	primary := &scriptedStore{MemoryStore: NewMemoryStore()}
	primary.setFail(true)
	cfg := recoveryCfg()
	cfg.BufferCapacity = 2
	cfg.Overflow = OverflowDiscard
	rec := NewRecoveringStore(primary, NewMemoryStore(), cfg)

	for i := 0; i < 5; i++ {
		if err := rec.Record(context.Background(), "k", Record{Key: "k"}); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}
	snap := rec.Snapshot()
	if snap.Buffered != 2 || snap.Dropped != 3 {
		t.Fatalf("snapshot=%+v want buffered=2 dropped=3", snap)
	}
}

func TestRecoveringOverflowAlertReturnsError(t *testing.T) {
	primary := &scriptedStore{MemoryStore: NewMemoryStore()}
	primary.setFail(true)
	cfg := recoveryCfg()
	cfg.BufferCapacity = 1
	cfg.Overflow = OverflowAlert
	rec := NewRecoveringStore(primary, NewMemoryStore(), cfg)

	if err := rec.Record(context.Background(), "k", Record{Key: "k"}); err != nil {
		t.Fatalf("first record: %v", err)
	}
	err := rec.Record(context.Background(), "k2", Record{Key: "k2"})
	if !errors.Is(err, ErrBufferOverflow) {
		t.Fatalf("want ErrBufferOverflow, got %v", err)
	}
	if snap := rec.Snapshot(); snap.Dropped != 1 {
		t.Fatalf("snapshot=%+v want dropped=1", snap)
	}
}

func TestRecoveringOverflowBlockUnblocksAfterDrain(t *testing.T) {
	primary := &scriptedStore{MemoryStore: NewMemoryStore()}
	primary.setFail(true)
	cfg := recoveryCfg()
	cfg.BufferCapacity = 1
	cfg.Overflow = OverflowBlock
	rec := NewRecoveringStore(primary, NewMemoryStore(), cfg).Start()
	defer rec.Stop()

	// 填满缓冲。
	if err := rec.Record(context.Background(), "k", Record{Key: "k"}); err != nil {
		t.Fatalf("first record: %v", err)
	}
	// 第二写入在缓冲未腾出前应阻塞；恢复后腾出空间完成。
	done := make(chan error, 1)
	go func() {
		done <- rec.Record(context.Background(), "k2", Record{Key: "k2"})
	}()
	select {
	case err := <-done:
		t.Fatalf("second record should block while degraded, got err=%v", err)
	case <-time.After(20 * time.Millisecond):
	}
	primary.setFail(false)
	waitFor(t, 2*time.Second, func() bool { return rec.Snapshot().State == StateHealthy })
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("blocked record after recovery: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("blocked record did not unblock after recovery")
	}
}

func TestRecoveringBlockRespectsContext(t *testing.T) {
	primary := &scriptedStore{MemoryStore: NewMemoryStore()}
	primary.setFail(true)
	cfg := recoveryCfg()
	cfg.BufferCapacity = 1
	cfg.Overflow = OverflowBlock
	rec := NewRecoveringStore(primary, NewMemoryStore(), cfg)

	if err := rec.Record(context.Background(), "k", Record{Key: "k"}); err != nil {
		t.Fatalf("first record: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := rec.Record(ctx, "k2", Record{Key: "k2"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want context deadline, got %v", err)
	}
}

func TestRecoveringNonDependencyErrorPropagates(t *testing.T) {
	cfg := recoveryCfg()
	cfg.DependencyErr = func(err error) bool { return err == nil } // 永不判定为依赖故障
	// 用普通 MemoryStore 作为"主"，本地故意失败以证明错误来自主存储且未降级。
	primary := NewMemoryStore()
	local := &failingLocal{}
	rec := NewRecoveringStore(primary, local, cfg)
	// 主存储正常时应返回 nil（主成功）。
	if err := rec.Complete(context.Background(), "k", 0, Record{Key: "k"}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if snap := rec.Snapshot(); snap.State != StateHealthy {
		t.Fatalf("snapshot=%+v want healthy", snap)
	}
}

// failingLocal 是在未被调用时应保持 failing 的本地存根。
type failingLocal struct{}

func (f *failingLocal) Claim(context.Context, Key, time.Duration, time.Time) (bool, error) {
	return false, errors.New("local should not be used")
}
func (f *failingLocal) IsCompleted(context.Context, Key) (bool, error) {
	return false, errors.New("local should not be used")
}
func (f *failingLocal) Complete(context.Context, Key, time.Duration, Record) error {
	return errors.New("local should not be used")
}
func (f *failingLocal) Release(context.Context, Key) error {
	return errors.New("local should not be used")
}
func (f *failingLocal) Record(context.Context, Key, Record) error {
	return errors.New("local should not be used")
}

func TestRecoveringStateObserverFires(t *testing.T) {
	primary := &scriptedStore{MemoryStore: NewMemoryStore()}
	primary.setFail(true)
	var transitions int32
	rec := NewRecoveringStore(primary, NewMemoryStore(), recoveryCfg())
	rec.OnStateChange(func(from, to RecoveryState) {
		atomic.AddInt32(&transitions, 1)
	})
	if err := rec.Complete(context.Background(), "k", 0, Record{Key: "k"}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if rec.Snapshot().State != StateDegraded {
		t.Fatalf("want degraded")
	}
	if atomic.LoadInt32(&transitions) < 1 {
		t.Fatalf("state observer fired %d times, want >=1", transitions)
	}
}

func TestRecoveringStopIsIdempotent(t *testing.T) {
	rec := NewRecoveringStore(&scriptedStore{MemoryStore: NewMemoryStore()}, NewMemoryStore(), recoveryCfg()).Start()
	if err := rec.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := rec.Stop(); err != nil {
		t.Fatalf("second stop should be idempotent: %v", err)
	}
}

func TestRecoveringHealthReflectsState(t *testing.T) {
	primary := &scriptedStore{MemoryStore: NewMemoryStore()}
	rec := NewRecoveringStore(primary, NewMemoryStore(), recoveryCfg())
	if result := rec.Health(); result.Status != health.StatusPass {
		t.Fatalf("healthy status=%q want pass", result.Status)
	}
	primary.setFail(true)
	if err := rec.Complete(context.Background(), "k", 0, Record{Key: "k", Status: StatusCompleted}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if rec.Snapshot().State != StateDegraded {
		t.Fatalf("want degraded")
	}
	if result := rec.Health(); result.Status != health.StatusWarn {
		t.Fatalf("degraded status=%q want warn", result.Status)
	}
}

// TestExecutorWithRecoveringStoreReplaysRecordsAndDedupes 验证业务链路在主存储故障时降级可用，
// 恢复后执行记录被回放，且幂等去重回到主存储完成。
func TestExecutorWithRecoveringStoreReplaysRecordsAndDedupes(t *testing.T) {
	primary := &scriptedStore{MemoryStore: NewMemoryStore()}
	primary.setFail(true)
	rec := NewRecoveringStore(primary, NewMemoryStore(), recoveryCfg()).Start()
	defer rec.Stop()
	executor := NewExecutor(rec)

	// 主存储故障期间，一次失败记录进入缓冲。
	if _, err := executor.Execute(context.Background(), Execution{
		Key: "pay:fail", Operation: func(context.Context) (any, error) {
			return nil, errors.New("boom")
		},
	}); err == nil {
		t.Fatalf("want failure")
	}
	// 一次成功记录也进入缓冲。
	okKey := Key("pay:ok")
	if _, err := executor.Execute(context.Background(), Execution{
		Key: okKey, Operation: func(context.Context) (any, error) { return "ok", nil },
	}); err != nil {
		t.Fatalf("execute ok under degradation: %v", err)
	}
	if snap := rec.Snapshot(); snap.State != StateDegraded || snap.Buffered != 3 {
		t.Fatalf("snapshot=%+v want degraded 3 buffered (release + failed record + completion)", snap)
	}

	// 主存储恢复：等待回放并切回 Healthy。
	primary.setFail(false)
	waitFor(t, 2*time.Second, func() bool { return rec.Snapshot().State == StateHealthy })
	if got, _ := primary.Records(context.Background(), "pay:fail"); len(got) != 1 {
		t.Fatalf("primary failed records=%d want 1", len(got))
	}
	if got, _ := primary.Records(context.Background(), "pay:ok"); len(got) != 1 {
		t.Fatalf("primary ok records=%d want 1", len(got))
	}

	// 恢复后幂等去重回到主存储：重复提交相同 key 返回 Duplicate，不重跑操作。
	var calls int32
	res, err := executor.Execute(context.Background(), Execution{
		Key: okKey, Operation: func(context.Context) (any, error) {
			atomic.AddInt32(&calls, 1)
			return "ok", nil
		},
	})
	if err != nil {
		t.Fatalf("duplicate execute: %v", err)
	}
	if !res.Duplicate || res.Status != StatusCompleted {
		t.Fatalf("duplicate result=%+v want Duplicate completed", res)
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Fatalf("operation should not rerun on duplicate, calls=%d", calls)
	}
}
