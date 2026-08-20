package execution

import (
	"context"
	"sync"
	"time"
)

// MemoryStore 是 Store 的内存实现，按 key 维护幂等占用与执行记录。仅进程内可见，
// 用于单实例运行与测试；跨进程严格一次不在此范围。
type MemoryStore struct {
	mu      sync.Mutex
	claims  map[Key]claimState
	records map[Key][]Record
	nowFunc func() time.Time
}

type claimState struct {
	status Status
	until  time.Time // 过期时间；zero 表示不设过期（完成仅当外部重建）
}

var _ Store = (*MemoryStore)(nil)

// NewMemoryStore 返回一个空的内存 backend。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		claims:  make(map[Key]claimState),
		records: make(map[Key][]Record),
		nowFunc: time.Now,
	}
}

// WithClock 注入当前时间函数（测试用），返回自身。
func (s *MemoryStore) WithClock(now func() time.Time) *MemoryStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nowFunc = now
	return s
}

func (s *MemoryStore) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
}

// Claim 建立/续期 key 的 running 占用。已有活动占用（未过期 running 或未过期 completed）时返回 false。
func (s *MemoryStore) Claim(ctx context.Context, key Key, ttl time.Duration, now time.Time) (bool, error) {
	if err := contextErr(ctx); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if now.IsZero() {
		now = s.now()
	}
	if st, ok := s.claims[key]; ok {
		active := st.until.IsZero() || now.Before(st.until)
		if active && (st.status == StatusRunning || st.status == StatusCompleted) {
			// 活动占用：running（进行中）或未过期 completed（已完成）都不允许再执行。
			return false, nil
		}
	}
	until := time.Time{}
	if ttl > 0 {
		until = now.Add(ttl)
	}
	s.claims[key] = claimState{status: StatusRunning, until: until}
	return true, nil
}

// IsCompleted 判断 key 是否已完成（幂等重复提交判定）。完成以 until 过期为准；无 until 视为不设过期。
func (s *MemoryStore) IsCompleted(ctx context.Context, key Key) (bool, error) {
	if err := contextErr(ctx); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.claims[key]
	if !ok || st.status != StatusCompleted {
		return false, nil
	}
	if st.until.IsZero() {
		return true, nil
	}
	return s.now().Before(st.until), nil
}

// Complete 记录成功完成：写入成功记录并把占用标记为 completed，保留窗口沿用 Claim 的 TTL。
func (s *MemoryStore) Complete(ctx context.Context, key Key, rec Record) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = s.now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// 保留窗口沿用原占用 TTL；调用方声明 0 时保持不过期。
	old := s.claims[key]
	until := old.until
	if until.IsZero() {
		until = time.Time{} // 原占用无 TTL 时，完成保留也不设过期（quasi-permanent）
	}
	s.claims[key] = claimState{status: StatusCompleted, until: until}
	s.records[key] = append(s.records[key], rec)
	return nil
}

// Record 写入一次过程/失败记录。
func (s *MemoryStore) Record(ctx context.Context, key Key, rec Record) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = s.now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[key] = append(s.records[key], rec)
	return nil
}

// Records 返回某 key 的执行记录副本（诊断用）。
func (s *MemoryStore) Records(ctx context.Context, key Key) ([]Record, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, 0, len(s.records[key]))
	out = append(out, s.records[key]...)
	return out, nil
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
