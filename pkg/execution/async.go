// 异步执行记录持久化：让业务链路不被执行记录（诊断/审计）的写入阻塞。
package execution

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AsyncConfig 控制异步执行记录持久化：有界队列、溢出策略与工作线程数。
type AsyncConfig struct {
	// Capacity 是异步记录队列上限；<=0 表示按默认值处理。
	Capacity int
	// Overflow 是队列满时的处理策略（复用 OverflowPolicy：丢弃/阻断/告警）。
	Overflow OverflowPolicy
	// Workers 是持久化工作线程数；<=0 表示默认 1。
	Workers int
	// OnError 是异步写入失败的观测回调（可为 nil，用于日志/指标/告警）。
	OnError func(error)
}

// defaultAsyncCapacity 是异步记录队列默认上限。
const defaultAsyncCapacity = 1000

// AsyncRecorder 是 Store 装饰器：幂等占用与完成状态（Claim/IsCompleted/Complete）仍同步
// 交给底层 Store（保证去重不重跑），仅"过程/失败执行记录"（Record）进入有界后台队列异步持久化，
// 避免等待记录落盘而阻塞正常业务链路。Shutdown 会排空剩余队列再退出工作线程。
type AsyncRecorder struct {
	inner Store
	cfg   AsyncConfig

	ch     chan queuedRecord
	stop   chan struct{}
	mu     sync.Mutex
	closed bool

	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
}

type queuedRecord struct {
	key Key
	rec Record
}

var _ Store = (*AsyncRecorder)(nil)

// NewAsyncRecorder 创建基于 inner 的异步记录持久化装饰器；需先 Start 再供业务使用，最终 Shutdown。
func NewAsyncRecorder(inner Store, cfg AsyncConfig) *AsyncRecorder {
	if inner == nil {
		inner = NewMemoryStore()
	}
	if cfg.Capacity <= 0 {
		cfg.Capacity = defaultAsyncCapacity
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.Overflow == "" {
		cfg.Overflow = OverflowDiscard
	}
	return &AsyncRecorder{
		inner: inner,
		cfg:   cfg,
		ch:    make(chan queuedRecord, cfg.Capacity),
		stop:  make(chan struct{}),
	}
}

// Start 启动固定数量的持久化工作线程（幂等）。
func (a *AsyncRecorder) Start() *AsyncRecorder {
	a.startOnce.Do(func() {
		for i := 0; i < a.cfg.Workers; i++ {
			a.wg.Add(1)
			go a.worker()
		}
	})
	return a
}

// Shutdown 停止接收新记录并排空剩余队列后退出工作线程（幂等）。
func (a *AsyncRecorder) Shutdown() {
	a.stopOnce.Do(func() {
		a.mu.Lock()
		a.closed = true
		a.mu.Unlock()
		close(a.stop)
		a.wg.Wait()
	})
}

func (a *AsyncRecorder) worker() {
	defer a.wg.Done()
	ctx := context.Background()
	for {
		select {
		case item := <-a.ch:
			a.write(ctx, item)
		case <-a.stop:
			// 停止信号：排空剩余队列后退出，保证不丢已接收记录。
			for {
				select {
				case item := <-a.ch:
					a.write(ctx, item)
				default:
					return
				}
			}
		}
	}
}

func (a *AsyncRecorder) write(ctx context.Context, item queuedRecord) {
	if err := a.inner.Record(ctx, item.key, item.rec); err != nil && a.cfg.OnError != nil {
		a.cfg.OnError(err)
	}
}

// Claim 同步转发（幂等语义必须同步）。
func (a *AsyncRecorder) Claim(ctx context.Context, key Key, ttl time.Duration, now time.Time) (bool, error) {
	return a.inner.Claim(ctx, key, ttl, now)
}

// IsCompleted 同步转发。
func (a *AsyncRecorder) IsCompleted(ctx context.Context, key Key) (bool, error) {
	return a.inner.IsCompleted(ctx, key)
}

// Complete 同步转发（完成状态决定去重，必须同步）。
func (a *AsyncRecorder) Complete(ctx context.Context, key Key, rec Record) error {
	return a.inner.Complete(ctx, key, rec)
}

// Record 异步持久化执行记录：入有界队列后立即返回；队列满按溢出策略处理。
// Shutdown 后回退为同步写入底层 Store，避免静默丢数据。
func (a *AsyncRecorder) Record(ctx context.Context, key Key, rec Record) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	return a.enqueue(ctx, queuedRecord{key: key, rec: rec})
}

// enqueue 快速路径在锁内与 closed 判定原子执行；阻断等待路径释放锁，避免 Shutdown 被长期闭锁。
// a.ch 永不关闭（关闭的是 a.stop），因此不会出现向已关闭 channel 发送的竞态。
func (a *AsyncRecorder) enqueue(ctx context.Context, item queuedRecord) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return a.inner.Record(ctx, item.key, item.rec)
	}
	select {
	case a.ch <- item:
		a.mu.Unlock()
		return nil
	default:
	}
	a.mu.Unlock()

	switch a.cfg.Overflow {
	case OverflowDiscard:
		return nil
	case OverflowAlert:
		return fmt.Errorf("%w: async record queue full", ErrBufferOverflow)
	default: // OverflowBlock
		select {
		case a.ch <- item:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-a.stop:
			return a.inner.Record(context.Background(), item.key, item.rec)
		}
	}
}
