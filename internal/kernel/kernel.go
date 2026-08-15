// Package kernel 负责底层 App 组件计划的启动、排空和配置事务。
package kernel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold-template/internal/kernel/logging"
	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
)

const (
	// DefaultDebounce 是文件配置变化的默认防抖时间。
	DefaultDebounce = 250 * time.Millisecond
	// DefaultReloadTimeout 是启动和单次重载事务的默认超时。
	DefaultReloadTimeout = 30 * time.Second
)

// BuiltinLoggerID 是 Kernel 内置 Logger target 在 App Plan 中的稳定身份。
const BuiltinLoggerID app.ID = "kernel.logger"

// Options 配置 Kernel 的监听和事务边界。
type Options struct {
	Debounce      time.Duration
	ReloadTimeout time.Duration
	Logging       *kernellogging.Manager
}

// ReloadResult 描述一轮配置候选对当前有效状态产生的结果。
type ReloadResult struct {
	Applied         bool
	PreviousDigest  string
	CurrentDigest   string
	Changed         []app.ID
	RestartRequired []app.ID
}

type kernelState uint8

const (
	kernelCreated kernelState = iota
	kernelRunning
	kernelDraining
	kernelFailed
	kernelStopped
)

// Kernel 执行一份显式安装的底层 App 组件计划。
type Kernel struct {
	loader  *config.Loader
	options Options

	operationMu    sync.Mutex
	mu             sync.Mutex
	state          kernelState
	installed      bool
	coordinated    bool
	components     []app.RuntimeComponent
	configurations []config.Binding
	snapshot       config.Snapshot
}

// New 创建尚未安装组件计划的空 Kernel。
func New(loader *config.Loader, options Options) (*Kernel, error) {
	if loader == nil {
		return nil, fmt.Errorf("kernel config loader is nil")
	}
	if options.Logging == nil {
		return nil, fmt.Errorf("kernel logging manager is nil")
	}
	if options.Debounce < 0 {
		return nil, fmt.Errorf("kernel debounce must be non-negative")
	}
	if options.ReloadTimeout < 0 {
		return nil, fmt.Errorf("kernel reload timeout must be non-negative")
	}
	if options.Debounce == 0 {
		options.Debounce = DefaultDebounce
	}
	if options.ReloadTimeout == 0 {
		options.ReloadTimeout = DefaultReloadTimeout
	}
	return &Kernel{loader: loader, options: options}, nil
}

// Name 返回进程监督参与者名称。
func (k *Kernel) Name() string { return "kernel" }

// Logger 返回配置加载前也始终可用、且不暴露替换权的稳定日志入口。
func (k *Kernel) Logger() pkglogger.Logger {
	if k == nil {
		return nil
	}
	return k.options.Logging.Logger()
}

// LoggerTarget 返回 composition 建立内置 Logger typed Binding 所需的替换 target。
// 该控制入口不得进入普通 Capabilities 或业务调用方。
func (k *Kernel) LoggerTarget() kernellogging.Target {
	if k == nil {
		return nil
	}
	return k.options.Logging
}

// Install 把完整冻结计划一次性安装到尚未启动的空 Kernel。
func (k *Kernel) Install(plan app.FrozenPlan) error {
	if k == nil {
		return fmt.Errorf("kernel is nil")
	}
	if err := plan.Validate(); err != nil {
		return fmt.Errorf("validate component plan: %w", err)
	}
	components := plan.Components()
	k.operationMu.Lock()
	defer k.operationMu.Unlock()
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.state != kernelCreated {
		return fmt.Errorf("install component plan after kernel start")
	}
	if k.installed {
		return fmt.Errorf("kernel component plan is already installed")
	}
	k.components = components
	k.configurations = plan.Configurations()
	k.installed = true
	return nil
}

// startCandidate 从进程级 coordinator 提供的同一候选启动全部 Kernel 组件。
func (k *Kernel) startCandidate(ctx context.Context, snapshot config.Snapshot) error {
	if ctx == nil {
		return ErrNilContext
	}
	k.operationMu.Lock()
	defer k.operationMu.Unlock()

	k.mu.Lock()
	if k.state == kernelStopped {
		k.mu.Unlock()
		return ErrStopped
	}
	if k.state == kernelRunning {
		k.mu.Unlock()
		return fmt.Errorf("kernel already started")
	}
	components := append([]app.RuntimeComponent(nil), k.components...)
	k.mu.Unlock()

	operationCtx, cancel := context.WithTimeout(ctx, k.options.ReloadTimeout)
	defer cancel()

	started := make([]app.RuntimeComponent, 0, len(components))
	for _, component := range components {
		changed, stageErr := component.Stage(snapshot)
		if stageErr != nil {
			return k.failStart(ctx, components, started, stageErr)
		}
		if !changed {
			continue
		}
		if buildErr := prepareComponent(operationCtx, component); buildErr != nil {
			cleanupErr := component.DiscardCandidate(context.WithoutCancel(ctx))
			return k.failStart(ctx, components, started, errors.Join(buildErr, cleanupErr))
		}
		component.PublishInitial()
		started = append(started, component)
	}

	k.mu.Lock()
	k.snapshot = snapshot
	k.state = kernelRunning
	k.mu.Unlock()
	k.options.Logging.Debug("kernel started", pkglogger.Int("components", len(components)))
	return nil
}

func (k *Kernel) failStart(ctx context.Context, components, started []app.RuntimeComponent, cause error) error {
	cleanupErr := stopStartedAfterFailure(ctx, k.options.ReloadTimeout, started)
	for _, component := range components {
		component.StopPending()
	}
	k.mu.Lock()
	k.state = kernelStopped
	k.mu.Unlock()
	return errors.Join(cause, cleanupErr)
}

func prepareComponent(ctx context.Context, component app.RuntimeComponent) error {
	if err := component.Build(ctx); err != nil {
		return err
	}
	if err := component.Start(ctx); err != nil {
		return err
	}
	return component.Ready(ctx)
}

// reloadCandidate 从进程级 coordinator 提供的同一候选执行配置事务。
func (k *Kernel) reloadCandidate(ctx context.Context, candidateSnapshot config.Snapshot) (ReloadResult, error) {
	if ctx == nil {
		return ReloadResult{}, ErrNilContext
	}
	k.operationMu.Lock()
	defer k.operationMu.Unlock()

	k.mu.Lock()
	if k.state != kernelRunning {
		state := k.state
		k.mu.Unlock()
		if state == kernelStopped {
			return ReloadResult{}, ErrStopped
		}
		return ReloadResult{}, ErrNotRunning
	}
	components := append([]app.RuntimeComponent(nil), k.components...)
	previousSnapshot := k.snapshot
	k.mu.Unlock()

	operationCtx, cancel := context.WithTimeout(ctx, k.options.ReloadTimeout)
	defer cancel()
	result := ReloadResult{PreviousDigest: previousSnapshot.Digest(), CurrentDigest: previousSnapshot.Digest()}

	changed := make([]app.RuntimeComponent, 0, len(components))
	for _, component := range components {
		componentChanged, stageErr := component.Stage(candidateSnapshot)
		if stageErr != nil {
			return result, stageErr
		}
		if !componentChanged {
			continue
		}
		changed = append(changed, component)
		result.Changed = append(result.Changed, component.ID())
		if component.Policy() == app.RestartRequired {
			result.RestartRequired = append(result.RestartRequired, component.ID())
		}
	}
	if len(result.RestartRequired) > 0 {
		return result, &app.RestartRequiredError{Components: append([]app.ID(nil), result.RestartRequired...)}
	}
	if len(changed) == 0 {
		k.mu.Lock()
		k.snapshot = candidateSnapshot
		k.mu.Unlock()
		result.CurrentDigest = candidateSnapshot.Digest()
		k.options.Logging.Debug("kernel reload unchanged")
		return result, nil
	}

	prepared := make([]app.RuntimeComponent, 0, len(changed))
	for _, component := range changed {
		if prepareErr := prepareComponent(operationCtx, component); prepareErr != nil {
			cleanupErr := discardCandidatesAfterFailure(ctx, k.options.ReloadTimeout, append(prepared, component))
			return result, errors.Join(prepareErr, cleanupErr)
		}
		prepared = append(prepared, component)
	}

	drained, drainErr := drainReverse(operationCtx, changed)
	if drainErr != nil {
		for index := len(drained) - 1; index >= 0; index-- {
			drained[index].Rollback()
		}
		cleanupErr := discardCandidatesAfterFailure(ctx, k.options.ReloadTimeout, changed)
		return result, errors.Join(drainErr, cleanupErr)
	}
	for _, component := range changed {
		component.Commit()
	}
	k.mu.Lock()
	k.snapshot = candidateSnapshot
	k.mu.Unlock()
	result.CurrentDigest = candidateSnapshot.Digest()
	for _, component := range changed {
		component.Resume()
	}
	result.Applied = true

	cleanupErr := stopPreviousReverse(operationCtx, changed)
	if cleanupErr != nil {
		return result, &CommittedCleanupError{Err: cleanupErr}
	}
	k.options.Logging.Info("kernel reload completed", pkglogger.Any("changed", result.Changed))
	return result, nil
}

func drainReverse(ctx context.Context, components []app.RuntimeComponent) ([]app.RuntimeComponent, error) {
	drained := make([]app.RuntimeComponent, 0, len(components))
	for index := len(components) - 1; index >= 0; index-- {
		component := components[index]
		ready, err := component.BeginDrain()
		if err != nil {
			return drained, fmt.Errorf("drain component %s: %w", component.ID(), err)
		}
		drained = append(drained, component)
		select {
		case <-ctx.Done():
			return drained, fmt.Errorf("wait component %s drain: %w", component.ID(), ctx.Err())
		case <-ready:
		}
	}
	return drained, nil
}

// Stop 排空所有运行组件，并按计划反序释放 Kernel 拥有的资源。
func (k *Kernel) Stop(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	k.operationMu.Lock()
	defer k.operationMu.Unlock()

	k.mu.Lock()
	state := k.state
	if state == kernelStopped {
		k.mu.Unlock()
		return nil
	}
	components := append([]app.RuntimeComponent(nil), k.components...)
	if state == kernelCreated {
		k.state = kernelStopped
		k.mu.Unlock()
		for _, component := range components {
			component.StopPending()
		}
		return nil
	}
	if state == kernelRunning {
		k.state = kernelDraining
	}
	k.mu.Unlock()

	if state != kernelFailed {
		drainErr := terminalDrainReverse(ctx, components)
		if drainErr != nil {
			return &DrainIncompleteError{Err: drainErr}
		}
		for index := len(components) - 1; index >= 0; index-- {
			components[index].PrepareStop()
		}
	}
	var joined error
	for index := len(components) - 1; index >= 0; index-- {
		component := components[index]
		joined = errors.Join(joined,
			component.DiscardCandidate(ctx),
			component.StopPrevious(ctx),
			component.StopCurrent(ctx),
		)
	}
	k.mu.Lock()
	if joined != nil {
		k.state = kernelFailed
	} else {
		k.state = kernelStopped
	}
	k.mu.Unlock()
	if joined != nil {
		return joined
	}
	k.options.Logging.Debug("kernel stopped")
	return nil
}

func terminalDrainReverse(ctx context.Context, components []app.RuntimeComponent) error {
	var joined error
	for index := len(components) - 1; index >= 0; index-- {
		component := components[index]
		ready, err := component.BeginTerminalDrain()
		if err != nil {
			joined = errors.Join(joined, fmt.Errorf("drain component %s: %w", component.ID(), err))
			continue
		}
		select {
		case <-ctx.Done():
			return errors.Join(joined, fmt.Errorf("wait component %s terminal drain: %w", component.ID(), ctx.Err()))
		case <-ready:
		}
	}
	return joined
}

// ownerships 返回所有由 Kernel 管理的实例责任安全快照。
func (k *Kernel) ownerships() []app.OwnershipSnapshot {
	if k == nil {
		return nil
	}
	k.mu.Lock()
	components := append([]app.RuntimeComponent(nil), k.components...)
	k.mu.Unlock()
	var snapshots []app.OwnershipSnapshot
	for _, component := range components {
		snapshots = append(snapshots, component.Ownerships()...)
	}
	return snapshots
}

func discardCandidatesAfterFailure(parent context.Context, timeout time.Duration, components []app.RuntimeComponent) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
	defer cancel()
	var joined error
	for index := len(components) - 1; index >= 0; index-- {
		joined = errors.Join(joined, components[index].DiscardCandidate(ctx))
	}
	return joined
}

func stopPreviousReverse(ctx context.Context, components []app.RuntimeComponent) error {
	var joined error
	for index := len(components) - 1; index >= 0; index-- {
		joined = errors.Join(joined, components[index].StopPrevious(ctx))
	}
	return joined
}

func stopStartedAfterFailure(parent context.Context, timeout time.Duration, components []app.RuntimeComponent) error {
	if len(components) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
	defer cancel()
	drained, err := drainReverse(ctx, components)
	if err != nil {
		return err
	}
	for index := len(components) - 1; index >= 0; index-- {
		components[index].PrepareStop()
	}
	var joined error
	for index := len(components) - 1; index >= 0; index-- {
		joined = errors.Join(joined, components[index].StopCurrent(ctx))
	}
	_ = drained
	return joined
}
