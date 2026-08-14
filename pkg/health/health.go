package health

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Status 表示组件健康状态。
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Check 是单个健康检查函数。
type Check func(context.Context) Result

// Result 是健康检查结果。
type Result struct {
	Name      string
	Kind      Kind
	Status    Status
	Message   string
	Error     error
	CheckedAt time.Time
	Duration  time.Duration
}

// Registry 保存组件健康检查。
type Registry struct {
	timeout time.Duration
	mu      sync.RWMutex
	checks  map[string]Check
	order   []string
}

// New 创建健康检查注册表。
func New(timeout time.Duration) *Registry {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &Registry{timeout: timeout, checks: make(map[string]Check)}
}

// Register 注册命名健康检查。
func (r *Registry) Register(name string, check Check) error {
	if name == "" {
		return fmt.Errorf("health check name is required")
	}
	if check == nil {
		return fmt.Errorf("health check %s is nil", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.checks[name]; exists {
		return fmt.Errorf("health check %s already registered", name)
	}
	r.checks[name] = check
	r.order = append(r.order, name)
	return nil
}

// Snapshot 执行所有健康检查并返回汇总。
func (r *Registry) Snapshot(ctx context.Context) Snapshot {
	if ctx == nil {
		ctx = context.Background()
	}
	r.mu.RLock()
	checks := make([]struct {
		name  string
		check Check
	}, 0, len(r.order))
	for _, name := range r.order {
		checks = append(checks, struct {
			name  string
			check Check
		}{name: name, check: r.checks[name]})
	}
	r.mu.RUnlock()

	results := make([]Result, 0, len(checks))
	overall := StatusPass
	for _, registered := range checks {
		startedAt := time.Now()
		checkCtx, cancel := context.WithTimeout(ctx, r.timeout)
		result := registered.check(checkCtx)
		cancel()
		result.Name = registered.name
		result.CheckedAt = startedAt
		result.Duration = time.Since(startedAt)
		if result.Status == "" {
			if result.Error != nil {
				result.Status = StatusFail
			} else {
				result.Status = StatusPass
			}
		}
		if result.Status == StatusFail {
			overall = StatusFail
		} else if result.Status == StatusWarn && overall != StatusFail {
			overall = StatusWarn
		}
		results = append(results, result)
	}
	return Snapshot{Status: overall, Results: results}
}

// Snapshot 是一次健康检查汇总。
type Snapshot struct {
	Status  Status
	Results []Result
}

// Error 返回失败检查的聚合错误。
func (s Snapshot) Error() error {
	var joined error
	for _, result := range s.Results {
		if result.Error != nil {
			joined = errors.Join(joined, fmt.Errorf("%s health: %w", result.Name, result.Error))
		}
	}
	return joined
}
