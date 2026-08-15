package kernel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
)

// LifecycleState 表示配置协调者拥有的 Kernel 进程状态。
type LifecycleState string

const (
	LifecycleNew            LifecycleState = "new"
	LifecycleStarting       LifecycleState = "starting"
	LifecycleRunning        LifecycleState = "running"
	LifecycleReloading      LifecycleState = "reloading"
	LifecycleDraining       LifecycleState = "draining"
	LifecycleCleanupPending LifecycleState = "cleanup-pending"
	LifecycleDegraded       LifecycleState = "degraded"
	LifecycleFailed         LifecycleState = "failed"
	LifecycleStopped        LifecycleState = "stopped"
)

// CoordinatorDiagnostics 是配置候选、代际和不可回滚清理状态的安全快照。
type CoordinatorDiagnostics struct {
	State            LifecycleState
	Ready            bool
	ConfigGeneration uint64
	ConfigDigest     string
	ConfigProvenance []string
	LastFailureType  string
	RestartRequired  bool
	CleanupRequired  bool
	Ownerships       []app.OwnershipSnapshot
	Since            time.Time
}

// Coordinator 是 Loader 的唯一进程级调用者，并把同一不可变候选交给 Kernel。
type Coordinator struct {
	runtime            *Kernel
	loader             *config.Loader
	bindings           []config.Binding
	kernelBindingCount int

	operationMu sync.Mutex
	mu          sync.Mutex
	diagnostics CoordinatorDiagnostics
	prepared    *config.Snapshot
	current     config.Snapshot
}

// NewCoordinator 为一个 Kernel 认领唯一配置协调者。
func NewCoordinator(runtime *Kernel, applicationBindings ...config.Binding) (*Coordinator, error) {
	if runtime == nil {
		return nil, fmt.Errorf("kernel coordinator runtime is nil")
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.coordinated {
		return nil, fmt.Errorf("kernel coordinator is already created")
	}
	bindings := append([]config.Binding(nil), runtime.configurations...)
	bindings = append(bindings, applicationBindings...)
	if err := config.ValidateRegistrations(bindings...); err != nil {
		return nil, fmt.Errorf("validate coordinator config bindings: %w", err)
	}
	runtime.coordinated = true
	return &Coordinator{
		runtime:            runtime,
		loader:             runtime.loader,
		bindings:           bindings,
		kernelBindingCount: len(runtime.configurations),
		diagnostics:        CoordinatorDiagnostics{State: LifecycleNew, Since: time.Now()},
	}, nil
}

// Prepare 加载并严格校验唯一初始候选，供纯内存 application composition 使用。
func (c *Coordinator) Prepare(ctx context.Context) (config.Snapshot, error) {
	if ctx == nil {
		return config.Snapshot{}, ErrNilContext
	}
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	c.mu.Lock()
	if c.prepared != nil || c.diagnostics.State != LifecycleNew {
		c.mu.Unlock()
		return config.Snapshot{}, fmt.Errorf("kernel coordinator is already prepared")
	}
	c.mu.Unlock()
	snapshot, err := c.loader.Load(ctx)
	if err == nil {
		err = config.ValidateCandidate(snapshot, c.bindings...)
	}
	if err != nil {
		return config.Snapshot{}, err
	}
	c.mu.Lock()
	copied := snapshot
	c.prepared = &copied
	c.mu.Unlock()
	return snapshot, nil
}

// Name 返回进程监督使用的稳定 owner ID。
func (*Coordinator) Name() string { return "kernel" }

// Start 只加载一次初始候选并启动 Kernel。
func (c *Coordinator) Start(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	c.mu.Lock()
	state := c.diagnostics.State
	c.mu.Unlock()
	if state == LifecycleFailed || state == LifecycleStopped {
		return ErrStopped
	}
	if state != LifecycleNew {
		return fmt.Errorf("kernel coordinator cannot start from state %s", state)
	}
	c.update(LifecycleStarting, false, nil, false, config.Snapshot{}, false)
	c.mu.Lock()
	prepared := c.prepared
	c.prepared = nil
	c.mu.Unlock()
	var snapshot config.Snapshot
	var err error
	if prepared != nil {
		snapshot = *prepared
	} else {
		snapshot, err = c.loader.Load(ctx)
		if err == nil {
			err = config.ValidateCandidate(snapshot, c.bindings...)
		}
	}
	if err == nil {
		err = c.runtime.startCandidate(ctx, snapshot)
	}
	if err != nil {
		c.update(LifecycleFailed, false, err, false, config.Snapshot{}, false)
		ownerships := c.runtime.ownerships()
		if hasIncompleteOwnership(ownerships) {
			c.mu.Lock()
			c.diagnostics.CleanupRequired = true
			c.mu.Unlock()
		}
		return err
	}
	c.update(LifecycleRunning, true, nil, true, snapshot, false)
	c.mu.Lock()
	c.current = snapshot
	c.mu.Unlock()
	return nil
}

// Reload 加载一次候选并让 Kernel 从同一候选完成预检和提交。
func (c *Coordinator) Reload(ctx context.Context) (ReloadResult, error) {
	if ctx == nil {
		return ReloadResult{}, ErrNilContext
	}
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	c.mu.Lock()
	state := c.diagnostics.State
	if state == LifecycleStopped {
		c.mu.Unlock()
		return ReloadResult{}, ErrStopped
	}
	if state != LifecycleRunning && state != LifecycleDegraded {
		c.mu.Unlock()
		return ReloadResult{}, ErrNotRunning
	}
	if c.diagnostics.State == LifecycleDegraded || c.diagnostics.RestartRequired {
		c.mu.Unlock()
		return ReloadResult{}, fmt.Errorf("kernel reload blocked until process restart")
	}
	c.mu.Unlock()
	c.update(LifecycleReloading, true, nil, false, config.Snapshot{}, false)
	snapshot, err := c.loader.Load(ctx)
	if err != nil {
		c.update(LifecycleRunning, true, err, false, config.Snapshot{}, false)
		return ReloadResult{}, err
	}
	if err := config.ValidateCandidate(snapshot, c.bindings...); err != nil {
		c.update(LifecycleRunning, true, err, false, config.Snapshot{}, false)
		return ReloadResult{}, err
	}
	c.mu.Lock()
	current := c.current
	external := append([]config.Binding(nil), c.bindings[c.kernelBindingCount:]...)
	c.mu.Unlock()
	restartIDs := make([]app.ID, 0, len(external))
	for _, binding := range external {
		previousDigest, previousErr := current.SectionDigest(binding.ConfigPath)
		candidateDigest, candidateErr := snapshot.SectionDigest(binding.ConfigPath)
		if previousErr != nil || candidateErr != nil {
			return ReloadResult{}, errors.Join(previousErr, candidateErr)
		}
		if previousDigest != candidateDigest {
			restartIDs = append(restartIDs, app.ID(binding.CapabilityID))
		}
	}
	if len(restartIDs) > 0 {
		err := &app.RestartRequiredError{Components: restartIDs}
		c.update(LifecycleRunning, true, err, false, config.Snapshot{}, true)
		return ReloadResult{PreviousDigest: current.Digest(), CurrentDigest: current.Digest(), RestartRequired: restartIDs}, err
	}
	result, err := c.runtime.reloadCandidate(ctx, snapshot)
	if err != nil {
		var committed *CommittedCleanupError
		switch {
		case errors.As(err, &committed):
			c.update(LifecycleDegraded, false, err, result.Applied, snapshot, true)
			c.mu.Lock()
			c.diagnostics.CleanupRequired = true
			c.mu.Unlock()
		case errors.Is(err, app.ErrRestartRequired):
			c.update(LifecycleRunning, true, err, false, config.Snapshot{}, true)
		default:
			ownerships := c.runtime.ownerships()
			if !hasIncompleteOwnership(ownerships) {
				c.update(LifecycleRunning, true, err, false, config.Snapshot{}, false)
			} else {
				c.update(LifecycleDegraded, false, err, false, config.Snapshot{}, true)
				c.mu.Lock()
				c.diagnostics.CleanupRequired = true
				c.mu.Unlock()
			}
		}
		return result, err
	}
	c.update(LifecycleRunning, true, nil, true, snapshot, false)
	c.mu.Lock()
	c.current = snapshot
	c.mu.Unlock()
	return result, nil
}

// Stop 进入不可回滚的终止排空，并释放 Kernel 当前资源。
func (c *Coordinator) Stop(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	c.update(LifecycleDraining, false, nil, false, config.Snapshot{}, false)
	err := c.runtime.Stop(ctx)
	if err != nil {
		var pending *DrainIncompleteError
		if errors.As(err, &pending) {
			c.update(LifecycleCleanupPending, false, err, false, config.Snapshot{}, false)
		} else {
			c.update(LifecycleFailed, false, err, false, config.Snapshot{}, false)
		}
		c.mu.Lock()
		c.diagnostics.CleanupRequired = true
		c.mu.Unlock()
		return err
	}
	c.update(LifecycleStopped, false, nil, false, config.Snapshot{}, false)
	c.mu.Lock()
	c.diagnostics.CleanupRequired = false
	c.mu.Unlock()
	return nil
}

// Diagnostics 返回不包含原始配置值的独立诊断快照。
func (c *Coordinator) Diagnostics() CoordinatorDiagnostics {
	if c == nil {
		return CoordinatorDiagnostics{State: LifecycleFailed}
	}
	c.mu.Lock()
	result := c.diagnostics
	result.ConfigProvenance = append([]string(nil), result.ConfigProvenance...)
	c.mu.Unlock()
	result.Ownerships = c.runtime.ownerships()
	return result
}

func (c *Coordinator) update(state LifecycleState, ready bool, failure error, committed bool, snapshot config.Snapshot, restart bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.diagnostics.State = state
	c.diagnostics.Ready = ready
	c.diagnostics.Since = time.Now()
	if failure != nil {
		c.diagnostics.LastFailureType = fmt.Sprintf("%T", failure)
	}
	if committed {
		if snapshot.Digest() != c.diagnostics.ConfigDigest {
			c.diagnostics.ConfigGeneration++
			c.diagnostics.ConfigDigest = snapshot.Digest()
			c.diagnostics.ConfigProvenance = snapshot.Provenance()
		}
	}
	if restart {
		c.diagnostics.RestartRequired = true
	}
}

func hasIncompleteOwnership(snapshots []app.OwnershipSnapshot) bool {
	for _, snapshot := range snapshots {
		switch snapshot.State {
		case app.OwnershipWaitingForDrain, app.OwnershipFinalizationPending, app.OwnershipFinalizing, app.OwnershipTerminalFailed:
			return true
		}
	}
	return false
}

var _ interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
} = (*Coordinator)(nil)
