package execution

import (
	"context"
	"errors"
	"testing"
	"time"
)

// blockRecordStore 在 Record 上阻塞直到 trigger 关闭，用于稳定制造队列满的场景。
type blockRecordStore struct {
	*MemoryStore
	trigger chan struct{}
}

func (s *blockRecordStore) Record(ctx context.Context, key Key, rec Record) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	select {
	case <-s.trigger:
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.MemoryStore.Record(ctx, key, rec)
}

func TestAsyncRecordPersistsAfterShutdown(t *testing.T) {
	inner := NewMemoryStore()
	rec := NewAsyncRecorder(inner, AsyncConfig{Capacity: 16, Workers: 1}).Start()
	if err := rec.Record(context.Background(), "k", Record{Key: "k", Status: StatusFailed}); err != nil {
		t.Fatalf("record: %v", err)
	}
	// 异步写入：此时不保证立即可见，但 Shutdown 排空后必须落盘。
	rec.Shutdown()
	if got, _ := inner.Records(context.Background(), "k"); len(got) != 1 {
		t.Fatalf("records=%d want 1 after shutdown", len(got))
	}
}

func TestAsyncShutdownFlushesMany(t *testing.T) {
	inner := NewMemoryStore()
	rec := NewAsyncRecorder(inner, AsyncConfig{Capacity: 8, Workers: 2, Overflow: OverflowDiscard}).Start()
	const n = 50
	for i := 0; i < n; i++ {
		_ = rec.Record(context.Background(), "k", Record{Key: "k"})
	}
	rec.Shutdown()
	got, _ := inner.Records(context.Background(), "k")
	if got == nil {
		t.Fatalf("records should be non-nil after flush")
	}
	if len(got) > n {
		t.Fatalf("records=%d should not exceed submitted=%d", len(got), n)
	}
}

func TestAsyncCompleteIsSynchronous(t *testing.T) {
	inner := NewMemoryStore()
	rec := NewAsyncRecorder(inner, AsyncConfig{Capacity: 8}).Start()
	defer rec.Shutdown()
	// Complete 同步落盘：返回后立即可见，且完成状态用于幂等去重。
	if err := rec.Complete(context.Background(), "k", Record{Key: "k", Status: StatusCompleted}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	done, err := rec.IsCompleted(context.Background(), "k")
	if err != nil || !done {
		t.Fatalf("IsCompleted=%v err=%v want completed synchronously", done, err)
	}
	claimed, err := rec.Claim(context.Background(), "k", 0, time.Now())
	if err != nil || claimed {
		t.Fatalf("claim after complete should be rejected: claimed=%v err=%v", claimed, err)
	}
}

func TestAsyncOverflowDiscardNoError(t *testing.T) {
	inner := &blockRecordStore{MemoryStore: NewMemoryStore(), trigger: make(chan struct{})}
	rec := NewAsyncRecorder(inner, AsyncConfig{Capacity: 1, Workers: 1, Overflow: OverflowDiscard}).Start()
	for i := 0; i < 20; i++ {
		if err := rec.Record(context.Background(), "k", Record{Key: "k"}); err != nil {
			t.Fatalf("discard record %d should not error: %v", i, err)
		}
	}
	close(inner.trigger)
	rec.Shutdown()
}

func TestAsyncOverflowAlertReturnsError(t *testing.T) {
	inner := &blockRecordStore{MemoryStore: NewMemoryStore(), trigger: make(chan struct{})}
	rec := NewAsyncRecorder(inner, AsyncConfig{Capacity: 1, Workers: 1, Overflow: OverflowAlert}).Start()
	var sawOverflow bool
	for i := 0; i < 20; i++ {
		if err := rec.Record(context.Background(), "k", Record{Key: "k"}); errors.Is(err, ErrBufferOverflow) {
			sawOverflow = true
			break
		}
	}
	close(inner.trigger)
	rec.Shutdown()
	if !sawOverflow {
		t.Fatal("expected ErrBufferOverflow under alert overflow")
	}
}

func TestAsyncOverflowBlockRespectsContext(t *testing.T) {
	inner := &blockRecordStore{MemoryStore: NewMemoryStore(), trigger: make(chan struct{})}
	rec := NewAsyncRecorder(inner, AsyncConfig{Capacity: 1, Workers: 1, Overflow: OverflowBlock}).Start()
	defer func() {
		close(inner.trigger)
		rec.Shutdown()
	}()
	// 填满队列：worker 阻塞在第一条记录上，队列再容纳一条后即满。
	_ = rec.Record(context.Background(), "k1", Record{Key: "k1"})
	// 让 worker 至少处理到阻塞态：略等，再填满队列。
	time.Sleep(10 * time.Millisecond)
	_ = rec.Record(context.Background(), "k2", Record{Key: "k2"})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := rec.Record(ctx, "k3", Record{Key: "k3"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocking record should respect context, got %v", err)
	}
}

func TestAsyncSyncFallbackAfterShutdown(t *testing.T) {
	inner := NewMemoryStore()
	rec := NewAsyncRecorder(inner, AsyncConfig{Capacity: 8}).Start()
	rec.Shutdown()
	// Shutdown 后 Record 回退为同步写。
	if err := rec.Record(context.Background(), "k", Record{Key: "k"}); err != nil {
		t.Fatalf("record after shutdown: %v", err)
	}
	if got, _ := inner.Records(context.Background(), "k"); len(got) != 1 {
		t.Fatalf("records=%d want 1 after sync fallback", len(got))
	}
}
