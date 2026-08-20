// Package execution 定义由 Kernel 治理的后台任务执行能力 App 组件。
// 后端默认形态：内存 backend（幂等占用 + 执行记录），通过组件开关可在 memory 与 disabled 间切换。
// 组件承载三项通用技术能力：幂等、失败重试、执行记录，并装配外部依赖故障恢复治理
// （Degraded/Recovering/Healthy）与按模块独立声明的执行策略隔离。该层为纯技术基础设施，
// 不承载、不感知具体业务逻辑。
package execution

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	"github.com/rin721/go-scaffold-template/internal/kernel/logging"
	pkgexecution "github.com/rin721/go-scaffold-template/pkg/execution"
	"github.com/rin721/go-scaffold-template/pkg/health"
	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
	"github.com/rin721/go-scaffold-template/pkg/resilience"
)

const (
	ID         app.ID = "execution"
	ConfigPath        = "execution"
)

// Driver 表示 Execution backend 的明确选择。
type Driver string

const (
	// DriverDisabled 表示当前进程不启用后台任务执行能力。
	DriverDisabled Driver = "disabled"
	// DriverMemory 表示当前进程使用内存 backend（进程内幂等 + 记录，单实例）。
	// 内存 backend 同时充当外部依赖主存储缺位时的降级兜底。
	DriverMemory Driver = "memory"
)

// 应用层默认配置（032 集中声明）：不依赖 pkg/execution 的默认值。
const (
	defaultDriver        = DriverMemory
	defaultMaxAttempts   = 3
	defaultInitialWaitMs = 50
	defaultMaxWaitMs     = 500
)

// 恢复治理默认参数（应用层集中声明，单位毫秒；沿用既有恢复语义合理量级）。
const (
	defaultRecoveryProbeIntervalMs  = 1000
	defaultRecoveryInitialBackoffMs = 200
	defaultRecoveryMaxBackoffMs     = 30000
	defaultRecoveryVerifyAttempts   = 1
	defaultRecoveryBufferCapacity   = 1000
	defaultRecoveryOverflow         = pkgexecution.OverflowDiscard
)

// 异步执行记录持久化默认参数（应用层集中声明）。
const (
	defaultAsyncCapacity = 1000
	defaultAsyncOverflow = pkgexecution.OverflowDiscard
)

// Config 是 Execution App 的 typed 配置契约。
type Config struct {
	Driver           Driver                  `mapstructure:"driver"`
	RetryMaxAttempts int                     `mapstructure:"retryMaxAttempts"`
	RetryInitialWait int                     `mapstructure:"retryInitialWaitMs"`
	RetryMaxWait     int                     `mapstructure:"retryMaxWaitMs"`
	Policies         map[string]PolicyConfig `mapstructure:"policies"`
	Recovery         RecoveryConfig          `mapstructure:"recovery"`
	Async            AsyncConfig             `mapstructure:"async"`
}

// AsyncConfig 控制异步执行记录持久化（有界队列 + 溢出策略）。
type AsyncConfig struct {
	Capacity int                         `mapstructure:"capacity"`
	Overflow pkgexecution.OverflowPolicy `mapstructure:"overflow"`
}

// PolicyConfig 是业务模块按需独立声明的执行策略（模块之间策略隔离）。
type PolicyConfig struct {
	RetryMaxAttempts int `mapstructure:"retryMaxAttempts"`
	RetryInitialWait int `mapstructure:"retryInitialWaitMs"`
	RetryMaxWait     int `mapstructure:"retryMaxWaitMs"`
	TimeoutMs        int `mapstructure:"timeoutMs"`
}

// RecoveryConfig 控制外部依赖故障恢复治理（探测退避/最大频率、可用性验证、有界缓冲与溢出策略）。
type RecoveryConfig struct {
	ProbeIntervalMs  int                         `mapstructure:"probeIntervalMs"`
	InitialBackoffMs int                         `mapstructure:"initialBackoffMs"`
	MaxBackoffMs     int                         `mapstructure:"maxBackoffMs"`
	VerifyAttempts   int                         `mapstructure:"verifyAttempts"`
	BufferCapacity   int                         `mapstructure:"bufferCapacity"`
	Overflow         pkgexecution.OverflowPolicy `mapstructure:"overflow"`
}

// policySpec 是解析后的命名执行策略。
type policySpec struct {
	retry   resilience.RetryPolicy
	timeout time.Duration
}

// Access 是业务模块消费的稳定执行入口。
type Access interface {
	Execute(context.Context, pkgexecution.Execution) (pkgexecution.Result, error)
	// Recovery 返回恢复治理的可观测快照（状态 / 缓冲 / 丢弃 / 状态变化次数）。
	Recovery() (pkgexecution.RecoverySnapshot, error)
	// Health 返回按恢复治理状态映射的健康结果，供健康检查 / 告警消费。
	Health() (health.Result, error)
}

// componentDeps 是 Execution 组件的注入依赖（当前为结构化 Logger）。
type componentDeps struct {
	logger pkglogger.Logger
}

type resource struct {
	driver        Driver
	executor      pkgexecution.OperationExecutor
	defaultPolicy resilience.RetryPolicy
	policies      map[string]policySpec
	recovering    *pkgexecution.RecoveringStore
	recorder      *pkgexecution.AsyncRecorder
}

type access struct {
	delegate app.Lease[*resource]
}

// Definition 返回无安装副作用的 Execution 组件声明；logger 为组件的结构化日志依赖输入。
func Definition(logger app.Input[logging.Target]) (app.Definition[Access], error) {
	source, err := app.Configured(ConfigPath, decode, defaults{})
	if err != nil {
		return app.Definition[Access]{}, err
	}
	dependencies, err := app.DependencySet(func(values app.Values) (componentDeps, error) {
		target, err := app.Resolve(values, logger)
		if err != nil {
			return componentDeps{}, err
		}
		return componentDeps{logger: target.Logger()}, nil
	}, logger)
	if err != nil {
		return app.Definition[Access]{}, err
	}
	return app.ManagedConfigured(
		ID,
		source,
		dependencies,
		build,
		app.Leased(newAccess),
		app.KernelInstanceSwap,
		app.WithReady(ready),
		app.WithTerminalFinalizer(stop),
	)
}

func newAccess(delegate app.Lease[*resource]) (Access, error) {
	if delegate == nil {
		return nil, fmt.Errorf("execution lease is nil")
	}
	return &access{delegate: delegate}, nil
}

// Configuration 返回 execution 组件的配置节契约（CapabilityID=execution, path=execution）。
// 该绑定只承载配置校验元数据，不依赖 Logger 输入、不创建任何资源；各入口（bootstrap / CLI / migrate / config）
// 用它识别 `execution` 配置节，无需先装配组件或注入 Logger。
func Configuration() config.Binding {
	return config.Binding{
		CapabilityID: string(ID),
		ConfigPath:   ConfigPath,
		Contract:     defaults{},
		Validate: func(snapshot config.Snapshot) error {
			_, err := decode(snapshot)
			return err
		},
	}
}

// Recovery 返回当前恢复治理快照；后端关闭或未装配恢复治理时返回错误。
func (a *access) Recovery() (pkgexecution.RecoverySnapshot, error) {
	var snap pkgexecution.RecoverySnapshot
	err := a.delegate.Use(context.Background(), func(current *resource) error {
		if current == nil {
			return fmt.Errorf("execution instance is nil")
		}
		if current.recovering == nil {
			return fmt.Errorf("execution recovery is not active")
		}
		snap = current.recovering.Snapshot()
		return nil
	})
	return snap, err
}

// Health 返回按恢复治理状态映射的健康结果。
func (a *access) Health() (health.Result, error) {
	var result health.Result
	err := a.delegate.Use(context.Background(), func(current *resource) error {
		if current == nil {
			return fmt.Errorf("execution instance is nil")
		}
		if current.recovering == nil {
			result = health.Result{Status: health.StatusFail, Message: "execution recovery is not active"}
			return nil
		}
		result = current.recovering.Health()
		return nil
	})
	return result, err
}

func (a *access) Execute(ctx context.Context, exec pkgexecution.Execution) (pkgexecution.Result, error) {
	if ctx == nil {
		return pkgexecution.Result{}, pkgexecution.ErrNilContext
	}
	var result pkgexecution.Result
	err := a.delegate.Use(ctx, func(current *resource) error {
		if current == nil {
			return fmt.Errorf("execution instance is nil")
		}
		if current.driver == DriverDisabled || current.executor == nil {
			return fmt.Errorf("execution backend is disabled")
		}
		// 按模块声明的命名策略解析；未声明时回退到组件集中声明的默认策略。
		if err := current.applyPolicy(&exec); err != nil {
			return err
		}
		var err error
		result, err = current.executor.Execute(ctx, exec)
		return err
	})
	return result, err
}

// applyPolicy 依据 PolicyName 或默认策略填充执行策略，实现模块之间策略隔离。
// 命名的策略必须存在；未知策略名返回可识别错误，不静默回退到默认策略。
func (r *resource) applyPolicy(exec *pkgexecution.Execution) error {
	if exec.PolicyName != "" {
		spec, ok := r.policies[exec.PolicyName]
		if !ok {
			return fmt.Errorf("unknown execution policy %q", exec.PolicyName)
		}
		exec.Policy = spec.retry
		if spec.timeout > 0 {
			exec.Timeout = spec.timeout
		}
		return nil
	}
	if exec.Policy.MaxAttempts == 0 {
		exec.Policy = r.defaultPolicy
	}
	return nil
}

func build(ctx context.Context, cfg Config, deps componentDeps) (*resource, error) {
	if ctx == nil {
		return nil, app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch cfg.Driver {
	case DriverDisabled:
		return &resource{driver: DriverDisabled}, nil
	case DriverMemory:
		// 当前 memory backend 同时兼作外部主存储缺位时的降级兜底；外部主存储接入为后续增量。
		return assemble(cfg, deps, pkgexecution.NewMemoryStore(), pkgexecution.NewMemoryStore())
	default:
		return nil, fmt.Errorf("unsupported execution driver %q", cfg.Driver)
	}
}

// assemble 用给定主存储与本地降级兜底装配执行资源并提供恢复治理。
// primary/local 通常为进程内 memory；测试可用故障注入主存储以覆盖降级与恢复语义。
func assemble(cfg Config, deps componentDeps, primary, local pkgexecution.Store) (*resource, error) {
	policy := retryPolicy(cfg.RetryMaxAttempts, cfg.RetryInitialWait, cfg.RetryMaxWait)
	policies := make(map[string]policySpec, len(cfg.Policies))
	for name, p := range cfg.Policies {
		policies[name] = policySpec{
			retry:   retryPolicy(p.RetryMaxAttempts, p.RetryInitialWait, p.RetryMaxWait),
			timeout: time.Duration(p.TimeoutMs) * time.Millisecond,
		}
	}
	recovering := pkgexecution.NewRecoveringStore(primary, local, recoveryConfig(cfg.Recovery))
	recovering.OnStateChange(logTransition(deps.logger))
	recovering.Start()
	// 执行记录采用异步持久化：幂等占用/完成仍同步（保证去重），过程/失败记录异步落盘避免阻塞链路。
	asyncCfg := asyncConfig(cfg.Async)
	asyncCfg.OnError = func(err error) {
		if deps.logger != nil {
			deps.logger.Warn("execution record persistence failed", pkglogger.String("error", err.Error()))
		}
	}
	recorder := pkgexecution.NewAsyncRecorder(recovering, asyncCfg)
	recorder.Start()
	return &resource{
		driver:        DriverMemory,
		executor:      pkgexecution.NewExecutor(recorder),
		defaultPolicy: policy,
		policies:      policies,
		recovering:    recovering,
		recorder:      recorder,
	}, nil
}

func asyncConfig(cfg AsyncConfig) pkgexecution.AsyncConfig {
	if cfg.Capacity <= 0 {
		cfg.Capacity = defaultAsyncCapacity
	}
	if cfg.Overflow == "" {
		cfg.Overflow = defaultAsyncOverflow
	}
	return pkgexecution.AsyncConfig{Capacity: cfg.Capacity, Overflow: cfg.Overflow}
}

// logTransition 返回恢复治理状态变化的结构化日志回调（Degraded 告警，Recovering/Healthy 信息）。
func logTransition(logger pkglogger.Logger) func(pkgexecution.RecoveryState, pkgexecution.RecoveryState) {
	if logger == nil {
		return nil
	}
	return func(from, to pkgexecution.RecoveryState) {
		switch to {
		case pkgexecution.StateDegraded:
			logger.Warn("execution external dependency degraded",
				pkglogger.String("from", string(from)), pkglogger.String("state", string(to)))
		case pkgexecution.StateRecovering, pkgexecution.StateHealthy:
			logger.Info("execution external dependency recovered",
				pkglogger.String("from", string(from)), pkglogger.String("state", string(to)))
		}
	}
}

func ready(ctx context.Context, current *resource) error {
	if ctx == nil {
		return app.ErrNilContext
	}
	if current == nil {
		return fmt.Errorf("execution instance is nil")
	}
	// 内存 backend 无外部就绪依赖；disabled 也视为就绪（组件存在但关闭）。
	// 外部主存储接入后，此处不得因主存储未就绪而阻止应用启动（降级 + 恢复探测接管）。
	return nil
}

func stop(_ context.Context, current *resource) error {
	if current == nil {
		return nil
	}
	// 先排空异步记录队列，再停止恢复探测循环，避免关闭顺序问题。
	if current.recorder != nil {
		current.recorder.Shutdown()
	}
	if current.recovering != nil {
		return current.recovering.Stop()
	}
	return nil
}

func recoveryConfig(cfg RecoveryConfig) pkgexecution.RecoveryConfig {
	return pkgexecution.RecoveryConfig{
		ProbeInterval:  time.Duration(cfg.ProbeIntervalMs) * time.Millisecond,
		InitialBackoff: time.Duration(cfg.InitialBackoffMs) * time.Millisecond,
		MaxBackoff:     time.Duration(cfg.MaxBackoffMs) * time.Millisecond,
		VerifyAttempts: cfg.VerifyAttempts,
		BufferCapacity: cfg.BufferCapacity,
		Overflow:       cfg.Overflow,
	}
}

func retryPolicy(maxAttempts, initialWaitMs, maxWaitMs int) resilience.RetryPolicy {
	return resilience.RetryPolicy{
		MaxAttempts: maxAttempts,
		InitialWait: time.Duration(initialWaitMs) * time.Millisecond,
		MaxWait:     time.Duration(maxWaitMs) * time.Millisecond,
	}
}

func decode(snapshot config.Snapshot) (Config, error) {
	cfg := defaultConfig()
	if err := snapshot.DecodeSection(ConfigPath, &cfg); err != nil {
		return Config{}, err
	}
	cfg.Driver = Driver(strings.ToLower(strings.TrimSpace(string(cfg.Driver))))
	switch cfg.Driver {
	case DriverDisabled, DriverMemory:
	default:
		return Config{}, fmt.Errorf("unsupported execution driver %q", cfg.Driver)
	}
	if cfg.RetryMaxAttempts < 0 || cfg.RetryInitialWait < 0 || cfg.RetryMaxWait < 0 {
		return Config{}, fmt.Errorf("execution retry policy values must be non-negative")
	}
	if cfg.Recovery.Overflow == "" {
		cfg.Recovery.Overflow = defaultRecoveryOverflow
	}
	switch cfg.Recovery.Overflow {
	case pkgexecution.OverflowDiscard, pkgexecution.OverflowBlock, pkgexecution.OverflowAlert:
	default:
		return Config{}, fmt.Errorf("unsupported execution recovery overflow policy %q", cfg.Recovery.Overflow)
	}
	if cfg.Recovery.ProbeIntervalMs < 0 || cfg.Recovery.InitialBackoffMs < 0 ||
		cfg.Recovery.MaxBackoffMs < 0 || cfg.Recovery.VerifyAttempts < 0 ||
		cfg.Recovery.BufferCapacity < 0 {
		return Config{}, fmt.Errorf("execution recovery values must be non-negative")
	}
	if cfg.Async.Overflow == "" {
		cfg.Async.Overflow = defaultAsyncOverflow
	}
	switch cfg.Async.Overflow {
	case pkgexecution.OverflowDiscard, pkgexecution.OverflowBlock, pkgexecution.OverflowAlert:
	default:
		return Config{}, fmt.Errorf("unsupported execution async overflow policy %q", cfg.Async.Overflow)
	}
	if cfg.Async.Capacity < 0 {
		return Config{}, fmt.Errorf("execution async capacity must be non-negative")
	}
	for name, p := range cfg.Policies {
		name = strings.TrimSpace(name)
		if name == "" {
			return Config{}, fmt.Errorf("execution policy name must be non-empty")
		}
		if p.RetryMaxAttempts < 0 || p.RetryInitialWait < 0 || p.RetryMaxWait < 0 || p.TimeoutMs < 0 {
			return Config{}, fmt.Errorf("execution policy %q values must be non-negative", name)
		}
	}
	return cfg, nil
}

type defaults struct{}

func (defaults) Defaults(ctx context.Context) (config.Object, config.Control, error) {
	if ctx == nil {
		return nil, config.Continue, app.ErrNilContext
	}
	cfg := defaultConfig()
	attempts, err := config.Number(fmt.Sprint(cfg.RetryMaxAttempts))
	if err != nil {
		return nil, config.Continue, err
	}
	initial, err := config.Number(fmt.Sprint(cfg.RetryInitialWait))
	if err != nil {
		return nil, config.Continue, err
	}
	max, err := config.Number(fmt.Sprint(cfg.RetryMaxWait))
	if err != nil {
		return nil, config.Continue, err
	}
	probe, err := config.Number(fmt.Sprint(cfg.Recovery.ProbeIntervalMs))
	if err != nil {
		return nil, config.Continue, err
	}
	initialBackoff, err := config.Number(fmt.Sprint(cfg.Recovery.InitialBackoffMs))
	if err != nil {
		return nil, config.Continue, err
	}
	maxBackoff, err := config.Number(fmt.Sprint(cfg.Recovery.MaxBackoffMs))
	if err != nil {
		return nil, config.Continue, err
	}
	verify, err := config.Number(fmt.Sprint(cfg.Recovery.VerifyAttempts))
	if err != nil {
		return nil, config.Continue, err
	}
	capacity, err := config.Number(fmt.Sprint(cfg.Recovery.BufferCapacity))
	if err != nil {
		return nil, config.Continue, err
	}
	asyncCap, err := config.Number(fmt.Sprint(cfg.Async.Capacity))
	if err != nil {
		return nil, config.Continue, err
	}
	fields := []config.Field{
		config.FieldOf("driver", config.String(string(cfg.Driver))),
		config.FieldOf("retryMaxAttempts", attempts),
		config.FieldOf("retryInitialWaitMs", initial),
		config.FieldOf("retryMaxWaitMs", max),
		config.FieldOf("recovery", config.ObjectValue(config.Object{
			config.FieldOf("probeIntervalMs", probe),
			config.FieldOf("initialBackoffMs", initialBackoff),
			config.FieldOf("maxBackoffMs", maxBackoff),
			config.FieldOf("verifyAttempts", verify),
			config.FieldOf("bufferCapacity", capacity),
			config.FieldOf("overflow", config.String(string(cfg.Recovery.Overflow))),
		})),
		config.FieldOf("async", config.ObjectValue(config.Object{
			config.FieldOf("capacity", asyncCap),
			config.FieldOf("overflow", config.String(string(cfg.Async.Overflow))),
		})),
	}
	// 按模块的策略隔离：逐个声明命名策略，键排序保证默认对象确定性。
	if len(cfg.Policies) > 0 {
		names := make([]string, 0, len(cfg.Policies))
		for name := range cfg.Policies {
			names = append(names, name)
		}
		sort.Strings(names)
		policyFields := make([]config.Field, 0, len(names))
		for _, name := range names {
			p := cfg.Policies[name]
			pFields := make([]config.Field, 0, 4)
			for _, item := range []struct {
				key   string
				value int
			}{
				{"retryMaxAttempts", p.RetryMaxAttempts},
				{"retryInitialWaitMs", p.RetryInitialWait},
				{"retryMaxWaitMs", p.RetryMaxWait},
				{"timeoutMs", p.TimeoutMs},
			} {
				number, err := config.Number(fmt.Sprint(item.value))
				if err != nil {
					return nil, config.Continue, err
				}
				pFields = append(pFields, config.FieldOf(item.key, number))
			}
			policyFields = append(policyFields, config.FieldOf(name, config.ObjectValue(config.Object(pFields))))
		}
		fields = append(fields, config.FieldOf("policies", config.ObjectValue(config.Object(policyFields))))
	}
	return config.Object(fields), config.Continue, nil
}

func defaultConfig() Config {
	return Config{
		Driver:           defaultDriver,
		RetryMaxAttempts: defaultMaxAttempts,
		RetryInitialWait: defaultInitialWaitMs,
		RetryMaxWait:     defaultMaxWaitMs,
		Recovery: RecoveryConfig{
			ProbeIntervalMs:  defaultRecoveryProbeIntervalMs,
			InitialBackoffMs: defaultRecoveryInitialBackoffMs,
			MaxBackoffMs:     defaultRecoveryMaxBackoffMs,
			VerifyAttempts:   defaultRecoveryVerifyAttempts,
			BufferCapacity:   defaultRecoveryBufferCapacity,
			Overflow:         defaultRecoveryOverflow,
		},
		Async: AsyncConfig{
			Capacity: defaultAsyncCapacity,
			Overflow: defaultAsyncOverflow,
		},
	}
}

var _ Access = (*access)(nil)
var _ config.DefaultContract = defaults{}
