// Package kernel 负责基础能力的启动、排空和配置事务。
package kernel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

const (
	// DefaultDebounce 是文件配置事件的默认防抖时间。
	DefaultDebounce = 250 * time.Millisecond
	// DefaultReloadTimeout 是启动和单次重载事务的默认总超时。
	DefaultReloadTimeout = 30 * time.Second
)

// Options 配置 kernel 的配置监听和事务边界。
type Options struct {
	Debounce      time.Duration
	ReloadTimeout time.Duration
	Logging       *kernellogging.Manager
}

// ReloadResult 描述一次配置加载对运行实例产生的影响。
type ReloadResult struct {
	Applied        bool
	PreviousDigest string
	CurrentDigest  string
	Changed        []ID
}

type kernelState uint8

const (
	kernelCreated kernelState = iota
	kernelRunning
	kernelStopped
)

// Kernel 管理彼此独立的基础能力定义。
type Kernel struct {
	loader  *config.Loader
	options Options

	operationMu sync.Mutex
	mu          sync.Mutex
	state       kernelState
	components  []component
	registered  map[ID]struct{}
	snapshot    config.Snapshot
}

// New 创建尚未启动的 kernel。
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
	return &Kernel{
		loader:     loader,
		options:    options,
		registered: make(map[ID]struct{}),
	}, nil
}

// Name 返回进程监督参与者名称。
func (k *Kernel) Name() string {
	return "kernel"
}

// LoggingManager 返回 Kernel 与 Logger Capability 共享的稳定日志 manager。
func (k *Kernel) LoggingManager() *kernellogging.Manager {
	if k == nil {
		return nil
	}
	return k.options.Logging
}

func (k *Kernel) register(item component) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.state != kernelCreated {
		return fmt.Errorf("register component %s after kernel start", item.id())
	}
	if _, exists := k.registered[item.id()]; exists {
		return fmt.Errorf("kernel component %s already registered", item.id())
	}
	k.registered[item.id()] = struct{}{}
	k.components = append(k.components, item)
	return nil
}

// Start 加载初始配置并按注册顺序构造、启动和发布全部能力。
func (k *Kernel) Start(ctx context.Context) error {
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
	components := append([]component(nil), k.components...)
	k.mu.Unlock()

	operationCtx, cancel := context.WithTimeout(ctx, k.options.ReloadTimeout)
	defer cancel()
	snapshot, err := k.loader.Load(operationCtx)
	if err != nil {
		return fmt.Errorf("load initial kernel config: %w", err)
	}

	staged := make([]component, 0, len(components))
	for _, item := range components {
		changed, err := item.stage(snapshot)
		if err != nil {
			return err
		}
		if changed {
			staged = append(staged, item)
		}
	}
	for _, item := range staged {
		if err := item.buildStart(operationCtx); err != nil {
			cleanupErr := discardCandidatesAfterFailure(ctx, k.options.ReloadTimeout, staged)
			return errors.Join(err, cleanupErr)
		}
	}
	for _, item := range staged {
		item.publishInitial()
	}

	k.mu.Lock()
	k.snapshot = snapshot
	k.state = kernelRunning
	k.mu.Unlock()
	k.options.Logging.Info("kernel started", pkglogger.Int("capabilities", len(components)))
	return nil
}

// Reload 执行一轮全体受影响能力的原子配置事务。
func (k *Kernel) Reload(ctx context.Context) (ReloadResult, error) {
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
	components := append([]component(nil), k.components...)
	previousSnapshot := k.snapshot
	k.mu.Unlock()

	operationCtx, cancel := context.WithTimeout(ctx, k.options.ReloadTimeout)
	defer cancel()
	candidateSnapshot, err := k.loader.Load(operationCtx)
	if err != nil {
		return ReloadResult{}, fmt.Errorf("load candidate kernel config: %w", err)
	}
	result := ReloadResult{
		PreviousDigest: previousSnapshot.Digest(),
		CurrentDigest:  candidateSnapshot.Digest(),
	}

	changed := make([]component, 0, len(components))
	for _, item := range components {
		componentChanged, err := item.stage(candidateSnapshot)
		if err != nil {
			return result, err
		}
		if componentChanged {
			changed = append(changed, item)
			result.Changed = append(result.Changed, item.id())
		}
	}
	if len(changed) == 0 {
		k.mu.Lock()
		k.snapshot = candidateSnapshot
		k.mu.Unlock()
		k.options.Logging.Debug("kernel reload unchanged")
		return result, nil
	}

	drained := make([]<-chan struct{}, 0, len(changed))
	for _, item := range changed {
		ready, err := item.beginDrain()
		if err != nil {
			for _, begun := range changed[:len(drained)] {
				begun.rollback()
			}
			return result, fmt.Errorf("drain component %s: %w", item.id(), err)
		}
		drained = append(drained, ready)
	}

	group, groupCtx := errgroup.WithContext(operationCtx)
	group.Go(func() error {
		for _, item := range changed {
			if err := item.buildStart(groupCtx); err != nil {
				return err
			}
		}
		return nil
	})
	group.Go(func() error {
		for index, ready := range drained {
			select {
			case <-groupCtx.Done():
				return fmt.Errorf("wait component %s drain: %w", changed[index].id(), groupCtx.Err())
			case <-ready:
			}
		}
		return nil
	})
	if err := group.Wait(); err != nil {
		cleanupErr := discardCandidatesAfterFailure(ctx, k.options.ReloadTimeout, changed)
		for _, item := range changed {
			item.rollback()
		}
		return result, errors.Join(err, cleanupErr)
	}

	for _, item := range changed {
		item.prepareCommit()
	}
	k.mu.Lock()
	k.snapshot = candidateSnapshot
	k.mu.Unlock()
	for _, item := range changed {
		item.publish()
	}
	result.Applied = true

	var cleanupErr error
	for index := len(changed) - 1; index >= 0; index-- {
		cleanupErr = errors.Join(cleanupErr, changed[index].stopPrevious(operationCtx))
	}
	if cleanupErr != nil {
		return result, &CommittedCleanupError{Err: cleanupErr}
	}
	k.options.Logging.Info("kernel reload completed", pkglogger.Any("changed", result.Changed))
	return result, nil
}

// Stop 排空所有能力并按反向注册顺序停止当前实例。
func (k *Kernel) Stop(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	k.operationMu.Lock()
	defer k.operationMu.Unlock()

	k.mu.Lock()
	if k.state == kernelStopped {
		k.mu.Unlock()
		return nil
	}
	components := append([]component(nil), k.components...)
	if k.state == kernelCreated {
		k.state = kernelStopped
		k.mu.Unlock()
		for _, item := range components {
			item.stopPending()
		}
		return nil
	}
	k.mu.Unlock()

	drained := make([]<-chan struct{}, 0, len(components))
	for _, item := range components {
		ready, err := item.beginDrain()
		if err != nil {
			for _, begun := range components[:len(drained)] {
				begun.rollback()
			}
			return fmt.Errorf("drain component %s for stop: %w", item.id(), err)
		}
		drained = append(drained, ready)
	}
	for index, ready := range drained {
		select {
		case <-ctx.Done():
			for _, item := range components {
				item.rollback()
			}
			return fmt.Errorf("wait component %s drain for stop: %w", components[index].id(), ctx.Err())
		case <-ready:
		}
	}
	for _, item := range components {
		item.prepareStop()
	}
	k.mu.Lock()
	k.state = kernelStopped
	k.mu.Unlock()

	var joined error
	for index := len(components) - 1; index >= 0; index-- {
		joined = errors.Join(joined, components[index].stopCurrent(ctx))
	}
	if joined == nil {
		k.options.Logging.Info("kernel stopped")
	}
	return joined
}

func discardCandidates(ctx context.Context, components []component) error {
	var joined error
	for index := len(components) - 1; index >= 0; index-- {
		joined = errors.Join(joined, components[index].discardCandidate(ctx))
	}
	return joined
}

func discardCandidatesAfterFailure(parent context.Context, timeout time.Duration, components []component) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
	defer cancel()
	return discardCandidates(ctx, components)
}
