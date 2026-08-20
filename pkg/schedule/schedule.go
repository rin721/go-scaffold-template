// Package schedule 定义业务模块声明定时任务所需的项目自有契约。
//
// 本包不暴露底层 scheduler、Redis、Generation 或生命周期控制权。业务模块只构造
// 不可变 Binding；统一 composition 负责把 Binding 连接到调度、协调与 Execution。
package schedule

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	maxTaskIDLength       = 128
	maxCronExpressionSize = 256
	maxTriggerDuration    = 365 * 24 * time.Hour
	maxTaskConcurrency    = 10_000
	maxTaskQueue          = 100_000
)

// TaskID 是应用内稳定唯一的任务标识。
type TaskID string

// Task 是业务模块声明的任务逻辑。实现必须尊重 context 取消。
type Task func(context.Context) error

// TriggerKind 是受支持的触发方式。
type TriggerKind string

const (
	TriggerCron       TriggerKind = "cron"
	TriggerFixedDelay TriggerKind = "fixedDelay"
)

// Trigger 是 cron/fixedDelay 的封闭联合值。
type Trigger struct {
	kind       TriggerKind
	expression string
	withSecond bool
	timezone   string
	delay      time.Duration
	initial    time.Duration
}

// Cron 创建 cron 触发器。timezone 为空时由 Scheduler 默认时区补齐。
func Cron(expression, timezone string, withSeconds bool) (Trigger, error) {
	trigger := Trigger{
		kind: TriggerCron, expression: strings.TrimSpace(expression),
		timezone: strings.TrimSpace(timezone), withSecond: withSeconds,
	}
	if err := trigger.Validate(); err != nil {
		return Trigger{}, err
	}
	return trigger, nil
}

// FixedDelay 创建从上一次完成时刻开始等待的固定间隔触发器。
func FixedDelay(delay, initialDelay time.Duration) (Trigger, error) {
	trigger := Trigger{kind: TriggerFixedDelay, delay: delay, initial: initialDelay}
	if err := trigger.Validate(); err != nil {
		return Trigger{}, err
	}
	return trigger, nil
}

// Kind 返回触发器种类。
func (t Trigger) Kind() TriggerKind { return t.kind }

// CronValues 返回 cron 参数；非 cron 触发器返回 ok=false。
func (t Trigger) CronValues() (expression, timezone string, withSeconds, ok bool) {
	if t.kind != TriggerCron {
		return "", "", false, false
	}
	return t.expression, t.timezone, t.withSecond, true
}

// FixedDelayValues 返回 fixedDelay 参数；非 fixedDelay 触发器返回 ok=false。
func (t Trigger) FixedDelayValues() (delay, initialDelay time.Duration, ok bool) {
	if t.kind != TriggerFixedDelay {
		return 0, 0, false
	}
	return t.delay, t.initial, true
}

// Validate 校验触发器的项目级基本约束；具体 cron parser 校验由内部 Adapter 在 Prepare 完成。
func (t Trigger) Validate() error {
	switch t.kind {
	case TriggerCron:
		if t.expression == "" {
			return fmt.Errorf("%w: cron expression is required", ErrInvalidTrigger)
		}
		if len(t.expression) > maxCronExpressionSize {
			return fmt.Errorf("%w: cron expression is too long", ErrInvalidTrigger)
		}
		fields := strings.Fields(t.expression)
		want := 5
		if t.withSecond {
			want = 6
		}
		if len(fields) != want {
			return fmt.Errorf("%w: cron expression requires %d fields", ErrInvalidTrigger, want)
		}
		if t.timezone != "" {
			if _, err := time.LoadLocation(t.timezone); err != nil {
				return fmt.Errorf("%w: invalid cron timezone: %w", ErrInvalidTrigger, err)
			}
		}
		return nil
	case TriggerFixedDelay:
		if t.delay <= 0 {
			return fmt.Errorf("%w: fixed delay must be positive", ErrInvalidTrigger)
		}
		if t.delay > maxTriggerDuration || t.initial > maxTriggerDuration {
			return fmt.Errorf("%w: fixed delay exceeds the supported limit", ErrInvalidTrigger)
		}
		if t.initial < 0 {
			return fmt.Errorf("%w: initial delay must be non-negative", ErrInvalidTrigger)
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported trigger kind %q", ErrInvalidTrigger, t.kind)
	}
}

// CongestionPolicy 表示任务达到本地并发上限后的处理方式。
type CongestionPolicy string

const (
	// CongestionSkip 跳过无法立即取得并发槽位的触发。
	CongestionSkip CongestionPolicy = "skip"
	// CongestionWait 只允许在显式有界队列中等待。
	CongestionWait CongestionPolicy = "wait"
)

// ConcurrencyPolicy 声明单 Task ID 的本地并发与有界积压。
type ConcurrencyPolicy struct {
	maxConcurrent int
	congestion    CongestionPolicy
	queueLimit    int
}

// Concurrency 创建任务并发策略。
func Concurrency(maxConcurrent int, congestion CongestionPolicy, queueLimit int) (ConcurrencyPolicy, error) {
	policy := ConcurrencyPolicy{maxConcurrent: maxConcurrent, congestion: congestion, queueLimit: queueLimit}
	if err := policy.Validate(); err != nil {
		return ConcurrencyPolicy{}, err
	}
	return policy, nil
}

// SerialSkip 返回不允许重叠且拥塞时跳过的默认策略。
func SerialSkip() ConcurrencyPolicy {
	return ConcurrencyPolicy{maxConcurrent: 1, congestion: CongestionSkip}
}

// MaxConcurrent 返回单任务最大并发数。
func (p ConcurrencyPolicy) MaxConcurrent() int { return p.maxConcurrent }

// Congestion 返回达到上限时的策略。
func (p ConcurrencyPolicy) Congestion() CongestionPolicy { return p.congestion }

// QueueLimit 返回等待策略的最大排队数。
func (p ConcurrencyPolicy) QueueLimit() int { return p.queueLimit }

// Validate 校验并发和积压必须有界。
func (p ConcurrencyPolicy) Validate() error {
	if p.maxConcurrent <= 0 {
		return fmt.Errorf("%w: max concurrency must be positive", ErrInvalidConcurrency)
	}
	if p.maxConcurrent > maxTaskConcurrency || p.queueLimit > maxTaskQueue {
		return fmt.Errorf("%w: concurrency or queue exceeds the supported limit", ErrInvalidConcurrency)
	}
	if p.queueLimit < 0 {
		return fmt.Errorf("%w: queue limit must be non-negative", ErrInvalidConcurrency)
	}
	switch p.congestion {
	case CongestionSkip:
		if p.queueLimit != 0 {
			return fmt.Errorf("%w: skip policy cannot declare a queue", ErrInvalidConcurrency)
		}
	case CongestionWait:
		if p.queueLimit == 0 {
			return fmt.Errorf("%w: wait policy requires a bounded queue", ErrInvalidConcurrency)
		}
	default:
		return fmt.Errorf("%w: unsupported congestion policy %q", ErrInvalidConcurrency, p.congestion)
	}
	return nil
}

// CoordinationMode 表示任务的跨实例执行权要求。
type CoordinationMode string

const (
	CoordinationLocal                 CoordinationMode = "local"
	CoordinationDistributedStrict     CoordinationMode = "distributedSingletonStrict"
	CoordinationDistributedBestEffort CoordinationMode = "distributedSingletonBestEffort"
)

// UnavailablePolicy 表示协调依赖不可用时的任务级策略。
type UnavailablePolicy string

const (
	UnavailableSkip  UnavailablePolicy = "skip"
	UnavailablePause UnavailablePolicy = "pause"
	UnavailableFail  UnavailablePolicy = "fail"
	UnavailableLocal UnavailablePolicy = "local"
)

// CoordinationPolicy 是任务级执行权与不可用策略。
type CoordinationPolicy struct {
	mode        CoordinationMode
	unavailable UnavailablePolicy
}

// Local 返回每个实例独立运行的本地策略。
func Local() CoordinationPolicy { return CoordinationPolicy{mode: CoordinationLocal} }

// DistributedSingleton 创建分布式单实例策略。
func DistributedSingleton(strict bool, unavailable UnavailablePolicy) (CoordinationPolicy, error) {
	mode := CoordinationDistributedBestEffort
	if strict {
		mode = CoordinationDistributedStrict
	}
	policy := CoordinationPolicy{mode: mode, unavailable: unavailable}
	if err := policy.Validate(); err != nil {
		return CoordinationPolicy{}, err
	}
	return policy, nil
}

// Mode 返回协调模式。
func (p CoordinationPolicy) Mode() CoordinationMode { return p.mode }

// OnUnavailable 返回协调依赖不可用策略。
func (p CoordinationPolicy) OnUnavailable() UnavailablePolicy { return p.unavailable }

// Distributed 表示任务是否需要跨实例执行权。
func (p CoordinationPolicy) Distributed() bool { return p.mode != CoordinationLocal }

// Validate 校验严格任务不会隐式或显式降级为本地执行。
func (p CoordinationPolicy) Validate() error {
	switch p.mode {
	case CoordinationLocal:
		if p.unavailable != "" {
			return fmt.Errorf("%w: local task cannot declare unavailable policy", ErrInvalidCoordination)
		}
		return nil
	case CoordinationDistributedStrict, CoordinationDistributedBestEffort:
	default:
		return fmt.Errorf("%w: unsupported coordination mode %q", ErrInvalidCoordination, p.mode)
	}
	switch p.unavailable {
	case UnavailableSkip, UnavailablePause, UnavailableFail:
		return nil
	case UnavailableLocal:
		if p.mode == CoordinationDistributedStrict {
			return fmt.Errorf("%w: strict task cannot degrade to local execution", ErrInvalidCoordination)
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported unavailable policy %q", ErrInvalidCoordination, p.unavailable)
	}
}

// Spec 是构造不可变 Binding 的声明输入。
type Spec struct {
	ID              TaskID
	Trigger         Trigger
	Concurrency     ConcurrencyPolicy
	Coordination    CoordinationPolicy
	ExecutionPolicy string
}

// Binding 是业务模块交给 composition 的不可变任务完成品。
type Binding struct {
	id              TaskID
	trigger         Trigger
	concurrency     ConcurrencyPolicy
	coordination    CoordinationPolicy
	executionPolicy string
	run             Task
}

var taskIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$`)

// Bind 构造并完整校验任务 Binding，不启动 goroutine 或访问外部资源。
func Bind(spec Spec, run Task) (Binding, error) {
	binding := Binding{
		id: spec.ID, trigger: spec.Trigger, concurrency: spec.Concurrency,
		coordination: spec.Coordination, executionPolicy: strings.TrimSpace(spec.ExecutionPolicy), run: run,
	}
	if err := binding.Validate(); err != nil {
		return Binding{}, err
	}
	return binding, nil
}

// ID 返回稳定任务标识。
func (b Binding) ID() TaskID { return b.id }

// Trigger 返回不可变触发器值。
func (b Binding) Trigger() Trigger { return b.trigger }

// Concurrency 返回任务级并发策略。
func (b Binding) Concurrency() ConcurrencyPolicy { return b.concurrency }

// Coordination 返回任务级分布式执行权策略。
func (b Binding) Coordination() CoordinationPolicy { return b.coordination }

// ExecutionPolicy 返回现有 Execution App 中的命名策略。
func (b Binding) ExecutionPolicy() string { return b.executionPolicy }

// Run 执行业务任务。只有统一调度运行器可以调用此方法。
func (b Binding) Run(ctx context.Context) error { return b.run(ctx) }

// Validate 验证 Binding 的全部跨字段不变量。
func (b Binding) Validate() error {
	if len(b.id) > maxTaskIDLength || !taskIDPattern.MatchString(string(b.id)) {
		return fmt.Errorf("%w: %q", ErrInvalidTaskID, b.id)
	}
	if b.run == nil {
		return ErrNilTask
	}
	if err := b.trigger.Validate(); err != nil {
		return err
	}
	if err := b.concurrency.Validate(); err != nil {
		return err
	}
	if err := b.coordination.Validate(); err != nil {
		return err
	}
	if b.trigger.Kind() == TriggerFixedDelay && (b.concurrency.MaxConcurrent() != 1 || b.concurrency.Congestion() != CongestionSkip) {
		return fmt.Errorf("%w: fixedDelay task must use serial skip concurrency", ErrInvalidConcurrency)
	}
	return nil
}

var (
	ErrInvalidTaskID       = errors.New("schedule: invalid task id")
	ErrNilTask             = errors.New("schedule: nil task")
	ErrInvalidTrigger      = errors.New("schedule: invalid trigger")
	ErrInvalidConcurrency  = errors.New("schedule: invalid concurrency policy")
	ErrInvalidCoordination = errors.New("schedule: invalid coordination policy")
)
