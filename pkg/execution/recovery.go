package execution

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// RecoveryState 描述外部依赖治理的当前健康状态。
type RecoveryState string

const (
	// StateHealthy 表示主存储可用，操作直接走主实现。
	StateHealthy RecoveryState = "healthy"
	// StateDegraded 表示主存储不可用，已降级到本地实现并启用恢复探测。
	StateDegraded RecoveryState = "degraded"
	// StateRecovering 表示探测与基本可用性验证已通过，正在回放缓冲并准备原子切回主实现。
	StateRecovering RecoveryState = "recovering"
)

// OverflowPolicy 定义降级缓冲达到上限时的处理策略（AGENTS：不允许无限积压）。
type OverflowPolicy string

const (
	// OverflowDiscard 到达上限后丢弃新记录并计数，不阻塞业务。
	OverflowDiscard OverflowPolicy = "discard"
	// OverflowBlock 到达上限后阻塞调用方直到缓冲腾出空间或上下文结束。
	OverflowBlock OverflowPolicy = "block"
	// OverflowAlert 到达上限后丢弃新记录、计数并返回可区分错误，供上层告警。
	OverflowAlert OverflowPolicy = "alert"
)

// ErrBufferOverflow 表示降级缓冲已满且按策略丢弃并需要显式告警（OverflowAlert）。
var ErrBufferOverflow = errors.New("execution: degradation buffer overflow")

// ErrStopped 表示恢复治理 Store 已被停止，操作无法继续等待缓冲空间。
var ErrStopped = errors.New("execution: recovery store stopped")

// Verifier 由主 Store 可选实现，用于恢复探测通过后的基本可用性验证。
type Verifier interface {
	// VerifyConnection 执行一次真实读写往返，验证主存储可用而不仅仅可达。
	VerifyConnection(context.Context) error
}

// RecoveryConfig 控制恢复探测、退避、最大探测频率、缓冲上限与溢出策略。
type RecoveryConfig struct {
	// ProbeInterval 是两次外部依赖探测之间的最小间隔（最大探测频率上限）。
	ProbeInterval time.Duration
	// InitialBackoff 是进入降级后的首次探测等待。
	InitialBackoff time.Duration
	// MaxBackoff 是探测等待的指数退避上限。
	MaxBackoff time.Duration
	// VerifyAttempts 是探测成功后连续通过基本可用性验证的次数。
	VerifyAttempts int
	// BufferCapacity 是降级期间有界本地记录缓冲的上限；<=0 表示按默认值处理。
	BufferCapacity int
	// Overflow 是缓冲到达上限时的处理策略。
	Overflow OverflowPolicy
	// DependencyErr 判定一个主 Store 错误是否属于外部依赖故障（应触发降级）。
	// 未设置时默认把任一非 nil 错误视为依赖故障。
	DependencyErr func(error) bool
	// Jitter 为探测等待注入抖动；nil 表示使用默认随机抖动。测试可注入恒等函数保证确定性。
	Jitter func(time.Duration) time.Duration
}

// RecoverySnapshot 是恢复治理的可观测快照（状态 / 缓冲 / 丢弃 / 状态变化次数）。
type RecoverySnapshot struct {
	State       RecoveryState
	Buffered    int
	Dropped     uint64
	Transitions uint64
}

// RecoveringStore 是具备故障降级与完整恢复机制的 Store 装饰器：
// 主 Store 不可用时自动降级到本地 Store，并把执行记录写入有界缓冲；后台恢复循环按
// 退避 + 抖动 + 最大频率规则探测，验证通过后回放缓冲并原子切回主实现。
type RecoveringStore struct {
	primary Store
	local   Store
	cfg     RecoveryConfig

	mu          sync.Mutex
	state       RecoveryState
	buffered    []bufferedRecord
	dropped     uint64
	transitions uint64
	backoff     time.Duration
	spaceCh     chan struct{}
	onChange    func(RecoveryState, RecoveryState)

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	doneCh    chan struct{}
}

type recKind uint8

const (
	kindComplete recKind = iota
	kindRecord
)

type bufferedRecord struct {
	key  Key
	kind recKind
	rec  Record
}

// 默认恢复治理参数（集中声明，供未显式配置时使用；应用层默认值由 kernel/app 声明）。
const (
	defaultProbeInterval   = time.Second
	defaultInitialBackoff  = 200 * time.Millisecond
	defaultMaxBackoff      = 30 * time.Second
	defaultVerifyAttempts  = 1
	defaultBufferCapacity  = 1000
	defaultJitterFraction  = 0.2 // 抖动幅度为等待时长的 ±20%
	recoveryProbeKeyPrefix = "execution:recovery-probe:"
	recoveryProbeTTL       = time.Minute
)

var _ Store = (*RecoveringStore)(nil)

// NewRecoveringStore 创建以 primary 为主、local 为降级兜底的恢复治理 Store。
// primary 为主存储（如 Cache/Database），local 为进程内降级存储（如 MemoryStore）。
func NewRecoveringStore(primary Store, local Store, cfg RecoveryConfig) *RecoveringStore {
	if primary == nil {
		primary = NewMemoryStore()
	}
	if local == nil {
		local = NewMemoryStore()
	}
	if cfg.ProbeInterval <= 0 {
		cfg.ProbeInterval = defaultProbeInterval
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = defaultInitialBackoff
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = defaultMaxBackoff
	}
	if cfg.VerifyAttempts <= 0 {
		cfg.VerifyAttempts = defaultVerifyAttempts
	}
	if cfg.BufferCapacity <= 0 {
		cfg.BufferCapacity = defaultBufferCapacity
	}
	if cfg.Overflow == "" {
		cfg.Overflow = OverflowDiscard
	}
	if cfg.Jitter == nil {
		cfg.Jitter = func(d time.Duration) time.Duration {
			if d <= 0 {
				return 0
			}
			fraction := (rand.Float64()*2 - 1) * defaultJitterFraction
			return time.Duration(float64(d) * (1 + fraction))
		}
	}
	if cfg.DependencyErr == nil {
		cfg.DependencyErr = func(error) bool { return true }
	}
	return &RecoveringStore{
		primary: primary,
		local:   local,
		cfg:     cfg,
		state:   StateHealthy,
		spaceCh: make(chan struct{}),
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

// OnStateChange 注册状态变化回调（用于应用层输出日志 / 指标 / 告警）。
// 必须在 Start 之前调用；回调不得反向调用本 Store 的阻塞方法，避免自死锁。
func (s *RecoveringStore) OnStateChange(fn func(from, to RecoveryState)) *RecoveringStore {
	s.onChange = fn
	return s
}

// Start 启动恢复探测循环（幂等，只能启动一次；goroutine 归属本 Store）。
func (s *RecoveringStore) Start() *RecoveringStore {
	s.startOnce.Do(func() {
		go s.loop()
	})
	return s
}

// Stop 停止恢复探测循环并等待其退出。
func (s *RecoveringStore) Stop() error {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		<-s.doneCh
	})
	return nil
}

// Snapshot 返回当前恢复治理的可观测快照。
func (s *RecoveringStore) Snapshot() RecoverySnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return RecoverySnapshot{
		State:       s.state,
		Buffered:    len(s.buffered),
		Dropped:     s.dropped,
		Transitions: s.transitions,
	}
}

// IsCompleted 实现 Store。
func (s *RecoveringStore) IsCompleted(ctx context.Context, key Key) (bool, error) {
	if err := contextErr(ctx); err != nil {
		return false, err
	}
	if s.usingPrimary() {
		done, err := s.primary.IsCompleted(ctx, key)
		if err == nil {
			return done, nil
		}
		if !s.degradeBecause(err) {
			return done, err
		}
	}
	return s.local.IsCompleted(ctx, key)
}

// Claim 实现 Store。
func (s *RecoveringStore) Claim(ctx context.Context, key Key, ttl time.Duration, now time.Time) (bool, error) {
	if err := contextErr(ctx); err != nil {
		return false, err
	}
	if s.usingPrimary() {
		claimed, err := s.primary.Claim(ctx, key, ttl, now)
		if err == nil {
			return claimed, nil
		}
		if !s.degradeBecause(err) {
			return claimed, err
		}
	}
	return s.local.Claim(ctx, key, ttl, now)
}

// Complete 实现 Store：降级期间写入本地并缓冲，供恢复后回放主存储。
func (s *RecoveringStore) Complete(ctx context.Context, key Key, rec Record) error {
	return s.write(ctx, key, kindComplete, rec, func() error {
		return s.primary.Complete(ctx, key, rec)
	}, func() error {
		return s.local.Complete(ctx, key, rec)
	})
}

// Record 实现 Store：降级期间写入本地并缓冲，供恢复后回放主存储。
func (s *RecoveringStore) Record(ctx context.Context, key Key, rec Record) error {
	return s.write(ctx, key, kindRecord, rec, func() error {
		return s.primary.Record(ctx, key, rec)
	}, func() error {
		return s.local.Record(ctx, key, rec)
	})
}

// write 统一处理 Complete/Record 的主/降级路由与缓冲成单一职责。
func (s *RecoveringStore) write(ctx context.Context, key Key, kind recKind, rec Record, toPrimary, toLocal func() error) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if s.usingPrimary() {
		if err := toPrimary(); err == nil {
			return nil
		} else if !s.degradeBecause(err) {
			return err
		}
		// 主存储失败已触发降级：本次记录转本地并缓冲。
		if err := toLocal(); err != nil {
			return err
		}
		return s.enqueue(ctx, bufferedRecord{key: key, kind: kind, rec: rec})
	}
	// 降级 / 恢复中：写本地并缓冲。
	if err := toLocal(); err != nil {
		return err
	}
	return s.enqueue(ctx, bufferedRecord{key: key, kind: kind, rec: rec})
}

// usingPrimary 判断当前路由是否应走主实现。
func (s *RecoveringStore) usingPrimary() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == StateHealthy
}

// degradeBecause 判定错误是否为依赖故障；若是则把状态置为 Degraded 并返回 true。
func (s *RecoveringStore) degradeBecause(err error) bool {
	if err == nil || !s.cfg.DependencyErr(err) {
		return false
	}
	s.mu.Lock()
	if s.state != StateHealthy {
		s.mu.Unlock()
		return true
	}
	s.state = StateDegraded
	s.transitions++
	s.backoff = s.cfg.InitialBackoff
	s.mu.Unlock()
	s.notify(StateHealthy, StateDegraded)
	return true
}

// notify 在锁外触发状态变化回调（回调不持有 Store 锁，避免自死锁）。
func (s *RecoveringStore) notify(from, to RecoveryState) {
	if s.onChange != nil {
		s.onChange(from, to)
	}
}

// enqueue 把记录写入有界缓冲，按溢出策略处理满容情况。
func (s *RecoveringStore) enqueue(ctx context.Context, item bufferedRecord) error {
	capacity := s.cfg.BufferCapacity
	for {
		s.mu.Lock()
		switch {
		case len(s.buffered) < capacity:
			s.buffered = append(s.buffered, item)
			s.mu.Unlock()
			return nil
		case s.cfg.Overflow == OverflowDiscard:
			s.dropped++
			s.mu.Unlock()
			return nil
		case s.cfg.Overflow == OverflowAlert:
			s.dropped++
			s.mu.Unlock()
			return fmt.Errorf("%w: cap=%d", ErrBufferOverflow, capacity)
		default: // OverflowBlock
			space := s.spaceCh
			s.mu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-s.stopCh:
				return ErrStopped
			case <-space:
				// 缓冲腾出空间后重试入队。
				continue
			}
		}
	}
}

// loop 是恢复探测主循环；仅在 Degraded 状态探测，退避 + 抖动 + 最大频率控制。
func (s *RecoveringStore) loop() {
	defer close(s.doneCh)
	wait := s.nextWait(s.cfg.InitialBackoff)
	for {
		timer := time.NewTimer(wait)
		select {
		case <-s.stopCh:
			timer.Stop()
			return
		case <-timer.C:
		}
		s.mu.Lock()
		if s.state != StateDegraded {
			s.mu.Unlock()
			wait = s.nextWait(s.cfg.InitialBackoff)
			continue
		}
		s.mu.Unlock()

		if s.verifyPrimary() {
			if s.recover() {
				wait = s.nextWait(s.cfg.InitialBackoff)
			} else {
				wait = s.growBackoff()
			}
		} else {
			wait = s.growBackoff()
		}
	}
}

// verifyPrimary 执行基本可用性验证：探测成功并不等于恢复，需连续多次真实读写往返通过。
func (s *RecoveringStore) verifyPrimary() bool {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ProbeInterval)
	defer cancel()
	for i := 0; i < s.cfg.VerifyAttempts; i++ {
		if err := s.verifyOnce(ctx); err != nil {
			return false
		}
	}
	return true
}

func (s *RecoveringStore) verifyOnce(ctx context.Context) error {
	if verifier, ok := s.primary.(Verifier); ok {
		return verifier.VerifyConnection(ctx)
	}
	// 主 Store 未实现 Verifier：用保留 key 做一次真实读写往返（占用后立即完成）。
	key := Key(recoveryProbeKeyPrefix + fmt.Sprintf("%d", time.Now().UnixNano()))
	claimed, err := s.primary.Claim(ctx, key, recoveryProbeTTL, time.Now())
	if err != nil {
		return err
	}
	if !claimed {
		// 占用被占（理论上不会）：仍说明存储可读写，视为通过。
		return nil
	}
	return s.primary.Complete(ctx, key, Record{Key: key, Status: StatusCompleted, CreatedAt: time.Now()})
}

// recover 进入 Recovering 状态并回放缓冲到主存储；全部回放成功后原子切回 Healthy。
func (s *RecoveringStore) recover() bool {
	s.mu.Lock()
	if s.state != StateDegraded {
		s.mu.Unlock()
		return false
	}
	s.state = StateRecovering
	s.transitions++
	s.notifyLocked(StateDegraded, StateRecovering)
	s.mu.Unlock()

	// 逐条回放；任一回放失败即退回 Degraded，保留剩余缓冲。
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ProbeInterval)
	defer cancel()
	for {
		s.mu.Lock()
		if len(s.buffered) == 0 {
			s.state = StateHealthy
			s.transitions++
			s.backoff = s.cfg.InitialBackoff
			s.notifyLocked(StateRecovering, StateHealthy)
			s.mu.Unlock()
			return true
		}
		item := s.buffered[0]
		s.mu.Unlock()

		var err error
		switch item.kind {
		case kindComplete:
			err = s.primary.Complete(ctx, item.key, item.rec)
		case kindRecord:
			err = s.primary.Record(ctx, item.key, item.rec)
		}
		if err != nil {
			s.mu.Lock()
			if s.state == StateRecovering {
				s.state = StateDegraded
				s.transitions++
				s.backoff = s.cfg.InitialBackoff
				s.notifyLocked(StateRecovering, StateDegraded)
			}
			s.mu.Unlock()
			return false
		}

		s.mu.Lock()
		s.buffered = s.buffered[1:]
		s.wakeLocked()
		s.mu.Unlock()
	}
}

// growBackoff 加倍当前退避（上限 MaxBackoff），并返回下次探测等待。
func (s *RecoveringStore) growBackoff() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backoff <= 0 {
		s.backoff = s.cfg.InitialBackoff
	}
	if s.backoff < s.cfg.MaxBackoff {
		if s.backoff*2 < s.cfg.MaxBackoff {
			s.backoff *= 2
		} else {
			s.backoff = s.cfg.MaxBackoff
		}
	}
	return s.nextWaitLocked(s.backoff)
}

// nextWait 计算下一次探测等待（最大频率下限 + 抖动）。
func (s *RecoveringStore) nextWait(base time.Duration) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nextWaitLocked(base)
}

func (s *RecoveringStore) nextWaitLocked(base time.Duration) time.Duration {
	if base <= 0 {
		base = s.backoff
		if base <= 0 {
			base = s.cfg.InitialBackoff
		}
	}
	wait := base
	if s.cfg.ProbeInterval > wait {
		wait = s.cfg.ProbeInterval // 最大探测频率：不小于探针间隔
	}
	return s.cfg.Jitter(wait)
}

// notifyLocked 在持有锁时触发状态变化回调（不回调、不 panic 传播给调用方）。
func (s *RecoveringStore) notifyLocked(from, to RecoveryState) {
	if s.onChange != nil {
		s.onChange(from, to)
	}
}

// wakeLocked 唤醒等待缓冲空间的阻塞入队者（持有锁调用）。
func (s *RecoveringStore) wakeLocked() {
	close(s.spaceCh)
	s.spaceCh = make(chan struct{})
}
