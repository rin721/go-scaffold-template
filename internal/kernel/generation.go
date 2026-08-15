package kernel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
)

// ActiveGeneration 是已经获得生产连接 admission 的不可变应用对象图。
type ActiveGeneration interface {
	ID() uint64
	Snapshot() config.Snapshot
	ConfiguredAddress() string
	BoundAddress() string
	ActiveConnections() int64
	ActiveRequests() int64
	ResourceStats() GenerationResourceStats
	Retire(context.Context) error
	Stop(context.Context) error
	ForceStop(context.Context) error
}

// GenerationResourceStats 描述当前代际显式复用和新建的 typed owner。
type GenerationResourceStats struct {
	Reused []string
	Built  []string
}

// PreparedGeneration 是全部 Build/Ready 完成、但尚未发布的应用候选。
type PreparedGeneration interface {
	ID() uint64
	Snapshot() config.Snapshot
	Commit(ActiveGeneration) (ActiveGeneration, error)
	Abort(context.Context) error
}

// GenerationFactory 从单一 Snapshot 构造完整 Application Generation。
type GenerationFactory interface {
	Prepare(context.Context, config.Snapshot, ActiveGeneration) (PreparedGeneration, error)
}

type generationFactoryStopper interface {
	Stop(context.Context) error
}

type generationFailureSource interface {
	Failures() <-chan error
}

// GenerationDiagnostics 是完整应用代际的脱敏运行状态。
type GenerationDiagnostics struct {
	State               LifecycleState
	Ready               bool
	Attempt             uint64
	CurrentGeneration   uint64
	CandidateGeneration uint64
	RetiringGeneration  uint64
	ConfigDigest        string
	ConfigProvenance    []string
	ChangedSections     []string
	Phase               string
	ConfiguredAddress   string
	BoundAddress        string
	RetiringAddress     string
	ActiveConnections   int64
	RetiringConnections int64
	ActiveRequests      int64
	RetiringRequests    int64
	ResourceReused      []string
	ResourceBuilt       []string
	RestartPolicy       string
	CleanupRequired     bool
	LastFailurePhase    string
	LastFailureOwner    string
	LastFailureType     string
	Since               time.Time
}

// GenerationReloadResult 描述一次完整应用候选的提交结果。
type GenerationReloadResult struct {
	Applied            bool
	PreviousGeneration uint64
	CurrentGeneration  uint64
	PreviousDigest     string
	CurrentDigest      string
	ChangedSections    []string
}

// GenerationCoordinator 是 Loader 的唯一 Service 调用者和 Application Generation 提交点。
type GenerationCoordinator struct {
	loader   *config.Loader
	bindings []config.Binding
	factory  GenerationFactory
	options  Options

	operationMu sync.Mutex
	mu          sync.Mutex
	current     ActiveGeneration
	retiring    ActiveGeneration
	diagnostics GenerationDiagnostics
}

// NewGenerationCoordinator 创建完整应用代际协调者。
func NewGenerationCoordinator(
	loader *config.Loader,
	bindings []config.Binding,
	factory GenerationFactory,
	options Options,
) (*GenerationCoordinator, error) {
	if loader == nil {
		return nil, fmt.Errorf("generation config loader is nil")
	}
	if factory == nil {
		return nil, fmt.Errorf("generation factory is nil")
	}
	if options.Logging == nil {
		return nil, fmt.Errorf("generation logging manager is nil")
	}
	if options.Debounce < 0 || options.ReloadTimeout < 0 {
		return nil, fmt.Errorf("generation timing options must be non-negative")
	}
	if options.Debounce == 0 {
		options.Debounce = DefaultDebounce
	}
	if options.ReloadTimeout == 0 {
		options.ReloadTimeout = DefaultReloadTimeout
	}
	bindings = append([]config.Binding(nil), bindings...)
	if err := config.ValidateRegistrations(bindings...); err != nil {
		return nil, fmt.Errorf("validate generation config bindings: %w", err)
	}
	return &GenerationCoordinator{
		loader: loader, bindings: bindings, factory: factory, options: options,
		diagnostics: GenerationDiagnostics{State: LifecycleNew, Phase: "idle", Since: time.Now()},
	}, nil
}

// Name 返回 Supervisor 使用的稳定 owner ID。
func (*GenerationCoordinator) Name() string { return "application-generation" }

// Start 构造并提交第一代应用对象图。
func (c *GenerationCoordinator) Start(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	c.mu.Lock()
	if c.diagnostics.State != LifecycleNew {
		state := c.diagnostics.State
		c.mu.Unlock()
		return fmt.Errorf("generation coordinator cannot start from state %s", state)
	}
	c.beginAttemptLocked(LifecycleStarting, "load")
	c.mu.Unlock()

	snapshot, err := c.loadCandidate(ctx)
	if err != nil {
		wrapped := generationOperationError("load", 0, err)
		c.fail(LifecycleFailed, "load", wrapped)
		return wrapped
	}
	prepared, err := c.prepare(ctx, snapshot, nil)
	if err != nil {
		wrapped := generationOperationError("prepare", 0, err)
		c.fail(LifecycleFailed, "prepare", wrapped)
		return wrapped
	}
	c.setCandidate(prepared.ID())
	active, err := prepared.Commit(nil)
	if err != nil {
		abortErr := c.abortPrepared(ctx, prepared)
		joined := generationOperationError("commit", prepared.ID(), errors.Join(err, abortErr))
		c.fail(LifecycleFailed, "commit", joined)
		return joined
	}
	c.mu.Lock()
	c.current = active
	c.publishLocked(active, nil)
	c.mu.Unlock()
	c.options.Logging.Info("application generation started",
		pkglogger.Any("generation", active.ID()),
		pkglogger.String("bound_address", active.BoundAddress()),
	)
	return nil
}

// Reload 加载、准备并提交一份完整应用候选。
func (c *GenerationCoordinator) Reload(ctx context.Context) (GenerationReloadResult, error) {
	if ctx == nil {
		return GenerationReloadResult{}, ErrNilContext
	}
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	c.mu.Lock()
	if c.diagnostics.State == LifecycleStopped {
		c.mu.Unlock()
		return GenerationReloadResult{}, ErrStopped
	}
	if c.diagnostics.State != LifecycleRunning {
		c.mu.Unlock()
		return GenerationReloadResult{}, ErrNotRunning
	}
	if c.diagnostics.CleanupRequired {
		c.mu.Unlock()
		return GenerationReloadResult{}, fmt.Errorf("application generation reload blocked by cleanup debt")
	}
	previous := c.current
	c.beginAttemptLocked(LifecycleReloading, "load")
	c.mu.Unlock()

	snapshot, err := c.loadCandidate(ctx)
	if err != nil {
		wrapped := generationOperationError("load", 0, err)
		c.reject("load", wrapped)
		return GenerationReloadResult{}, wrapped
	}
	previousSnapshot := previous.Snapshot()
	changed, err := changedConfigSections(previousSnapshot, snapshot, c.bindings)
	if err != nil {
		wrapped := generationOperationError("diff", previous.ID(), err)
		c.reject("diff", wrapped)
		return GenerationReloadResult{}, wrapped
	}
	result := GenerationReloadResult{
		PreviousGeneration: previous.ID(), CurrentGeneration: previous.ID(),
		PreviousDigest: previousSnapshot.Digest(), CurrentDigest: previousSnapshot.Digest(),
		ChangedSections: append([]string(nil), changed...),
	}
	if snapshot.Digest() == previousSnapshot.Digest() {
		c.mu.Lock()
		c.diagnostics.State = LifecycleRunning
		c.diagnostics.Ready = true
		c.diagnostics.Phase = "no-op"
		c.diagnostics.CandidateGeneration = 0
		c.diagnostics.ChangedSections = nil
		c.diagnostics.LastFailurePhase = ""
		c.diagnostics.LastFailureOwner = ""
		c.diagnostics.LastFailureType = ""
		c.diagnostics.Since = time.Now()
		c.mu.Unlock()
		return result, nil
	}

	c.setPhase("prepare", changed)
	prepared, err := c.prepare(ctx, snapshot, previous)
	if err != nil {
		wrapped := generationOperationError("prepare", 0, err)
		c.reject("prepare", wrapped)
		return result, wrapped
	}
	c.setCandidate(prepared.ID())
	active, err := prepared.Commit(previous)
	if err != nil {
		abortErr := c.abortPrepared(ctx, prepared)
		joined := generationOperationError("commit", prepared.ID(), errors.Join(err, abortErr))
		c.reject("commit", joined)
		return result, joined
	}
	c.mu.Lock()
	c.current = active
	c.retiring = previous
	c.diagnostics.RetiringGeneration = previous.ID()
	c.publishLocked(active, changed)
	c.diagnostics.Phase = "retire"
	c.mu.Unlock()
	result.Applied = true
	result.CurrentGeneration = active.ID()
	result.CurrentDigest = snapshot.Digest()

	retireCtx, cancelRetire := context.WithTimeout(context.WithoutCancel(ctx), c.options.ReloadTimeout)
	err = previous.Retire(retireCtx)
	cancelRetire()
	if err != nil {
		committed := &CommittedCleanupError{Err: generationOperationError("retire", previous.ID(), err)}
		c.mu.Lock()
		c.diagnostics.State = LifecycleDegraded
		c.diagnostics.Ready = false
		c.diagnostics.CleanupRequired = true
		c.diagnostics.LastFailurePhase = "retire"
		c.diagnostics.LastFailureOwner = "application-generation"
		c.diagnostics.LastFailureType = fmt.Sprintf("%T", err)
		c.diagnostics.Phase = "cleanup-debt"
		c.diagnostics.Since = time.Now()
		c.mu.Unlock()
		return result, committed
	}
	c.mu.Lock()
	c.retiring = nil
	c.diagnostics.RetiringGeneration = 0
	c.diagnostics.RetiringAddress = ""
	c.diagnostics.RetiringConnections = 0
	c.diagnostics.RetiringRequests = 0
	c.diagnostics.Phase = "active"
	c.diagnostics.Since = time.Now()
	c.mu.Unlock()
	c.options.Logging.Info("application generation reload completed",
		pkglogger.Any("generation", active.ID()),
		pkglogger.Any("changed_sections", changed),
		pkglogger.String("bound_address", active.BoundAddress()),
	)
	return result, nil
}

// Stop 停止当前代际及 Factory 的进程级 owner。
func (c *GenerationCoordinator) Stop(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	c.mu.Lock()
	if c.diagnostics.State == LifecycleStopped {
		c.mu.Unlock()
		return nil
	}
	current := c.current
	retiring := c.retiring
	c.diagnostics.State = LifecycleDraining
	c.diagnostics.Ready = false
	c.diagnostics.Phase = "shutdown"
	c.diagnostics.Since = time.Now()
	c.mu.Unlock()
	var joined error
	if current != nil {
		joined = errors.Join(joined, current.Stop(ctx))
	}
	if retiring != nil {
		joined = errors.Join(joined, retiring.Stop(ctx))
	}
	if joined == nil {
		if stopper, ok := c.factory.(generationFactoryStopper); ok {
			joined = errors.Join(joined, stopper.Stop(ctx))
		}
	}
	c.mu.Lock()
	if joined != nil {
		c.diagnostics.State = LifecycleFailed
		c.diagnostics.CleanupRequired = true
		c.diagnostics.LastFailurePhase = "shutdown"
		c.diagnostics.LastFailureOwner = "application-generation"
		c.diagnostics.LastFailureType = fmt.Sprintf("%T", joined)
		c.diagnostics.Phase = "shutdown-failed"
	} else {
		c.current = nil
		c.retiring = nil
		c.diagnostics.State = LifecycleStopped
		c.diagnostics.CleanupRequired = false
		c.diagnostics.Phase = "stopped"
	}
	c.diagnostics.Since = time.Now()
	c.mu.Unlock()
	return joined
}

// ForceStop 在 Supervisor 的剩余预算内显式强制终结未完成 generation。
func (c *GenerationCoordinator) ForceStop(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	c.mu.Lock()
	current := c.current
	retiring := c.retiring
	c.mu.Unlock()
	var joined error
	if current != nil {
		joined = errors.Join(joined, current.ForceStop(ctx))
	}
	if retiring != nil {
		joined = errors.Join(joined, retiring.ForceStop(ctx))
	}
	if stopper, ok := c.factory.(generationFactoryStopper); ok {
		joined = errors.Join(joined, stopper.Stop(ctx))
	}
	c.mu.Lock()
	if joined != nil {
		c.diagnostics.State = LifecycleFailed
		c.diagnostics.CleanupRequired = true
		c.diagnostics.LastFailurePhase = "force-stop"
		c.diagnostics.LastFailureOwner = "application-generation"
		c.diagnostics.LastFailureType = fmt.Sprintf("%T", joined)
		c.diagnostics.Phase = "force-stop-failed"
	} else {
		c.current = nil
		c.retiring = nil
		c.diagnostics.State = LifecycleStopped
		c.diagnostics.CleanupRequired = false
		c.diagnostics.Phase = "stopped"
	}
	c.diagnostics.Since = time.Now()
	c.mu.Unlock()
	return joined
}

// Monitor 把当前 generation 的意外 Serve 退出提升为进程级 runner 失败。
func (c *GenerationCoordinator) Monitor(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	source, ok := c.factory.(generationFailureSource)
	if !ok {
		<-ctx.Done()
		return nil
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-source.Failures():
		if err == nil {
			return fmt.Errorf("application generation failure channel closed")
		}
		return err
	}
}

// Watch 监听配置文件并以容量一的 latest-wins 通知串行触发完整 reload。
func (c *GenerationCoordinator) Watch(ctx context.Context, onReloadError func(error), ready chan<- struct{}) error {
	if ctx == nil {
		return ErrNilContext
	}
	if onReloadError == nil {
		return fmt.Errorf("application generation reload error callback is nil")
	}
	paths := c.loader.FilePaths()
	if len(paths) == 0 {
		return fmt.Errorf("application generation has no file config source")
	}
	changes := make(chan struct{}, 1)
	notify := func() {
		select {
		case changes <- struct{}{}:
		default:
		}
	}
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	watchDone := make(chan error, 1)
	go func() {
		watchDone <- config.WatchFiles(watchCtx, paths, c.options.Debounce, config.WatchCallbacks{
			OnReady: func() {
				notify()
				if ready != nil {
					close(ready)
					ready = nil
				}
			},
			OnChange: notify,
		})
	}()
	for {
		select {
		case <-ctx.Done():
			cancel()
			return <-watchDone
		case err := <-watchDone:
			if ctx.Err() != nil {
				return nil
			}
			if err == nil {
				return fmt.Errorf("application generation watcher stopped unexpectedly")
			}
			return err
		case <-changes:
			if _, err := c.Reload(ctx); err != nil && ctx.Err() == nil {
				onReloadError(err)
			}
		}
	}
}

// Diagnostics 返回完整应用代际的独立诊断快照。
func (c *GenerationCoordinator) Diagnostics() GenerationDiagnostics {
	if c == nil {
		return GenerationDiagnostics{State: LifecycleFailed}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	result := c.diagnostics
	result.ConfigProvenance = append([]string(nil), result.ConfigProvenance...)
	result.ChangedSections = append([]string(nil), result.ChangedSections...)
	result.ResourceReused = append([]string(nil), result.ResourceReused...)
	result.ResourceBuilt = append([]string(nil), result.ResourceBuilt...)
	if c.current != nil {
		result.ActiveRequests = c.current.ActiveRequests()
		result.ActiveConnections = c.current.ActiveConnections()
		result.ConfiguredAddress = c.current.ConfiguredAddress()
		result.BoundAddress = c.current.BoundAddress()
		stats := c.current.ResourceStats()
		result.ResourceReused = append([]string(nil), stats.Reused...)
		result.ResourceBuilt = append([]string(nil), stats.Built...)
	}
	if c.retiring != nil {
		result.RetiringAddress = c.retiring.BoundAddress()
		result.RetiringConnections = c.retiring.ActiveConnections()
		result.RetiringRequests = c.retiring.ActiveRequests()
	}
	return result
}

func (c *GenerationCoordinator) loadCandidate(ctx context.Context) (config.Snapshot, error) {
	operationCtx, cancel := context.WithTimeout(ctx, c.options.ReloadTimeout)
	defer cancel()
	snapshot, err := c.loader.Load(operationCtx)
	if err == nil {
		err = config.ValidateCandidate(snapshot, c.bindings...)
	}
	return snapshot, err
}

func (c *GenerationCoordinator) prepare(ctx context.Context, snapshot config.Snapshot, previous ActiveGeneration) (PreparedGeneration, error) {
	operationCtx, cancel := context.WithTimeout(ctx, c.options.ReloadTimeout)
	defer cancel()
	return c.factory.Prepare(operationCtx, snapshot, previous)
}

func (c *GenerationCoordinator) abortPrepared(ctx context.Context, prepared PreparedGeneration) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.options.ReloadTimeout)
	defer cancel()
	return prepared.Abort(cleanupCtx)
}

func (c *GenerationCoordinator) beginAttemptLocked(state LifecycleState, phase string) {
	c.diagnostics.Attempt++
	c.diagnostics.State = state
	c.diagnostics.Ready = state == LifecycleReloading
	c.diagnostics.Phase = phase
	c.diagnostics.Since = time.Now()
}

func (c *GenerationCoordinator) publishLocked(active ActiveGeneration, changed []string) {
	snapshot := active.Snapshot()
	c.diagnostics.State = LifecycleRunning
	c.diagnostics.Ready = true
	c.diagnostics.CurrentGeneration = active.ID()
	c.diagnostics.CandidateGeneration = 0
	c.diagnostics.ConfigDigest = snapshot.Digest()
	c.diagnostics.ConfigProvenance = snapshot.Provenance()
	c.diagnostics.ChangedSections = append([]string(nil), changed...)
	c.diagnostics.ConfiguredAddress = active.ConfiguredAddress()
	c.diagnostics.BoundAddress = active.BoundAddress()
	c.diagnostics.ActiveConnections = active.ActiveConnections()
	c.diagnostics.ActiveRequests = active.ActiveRequests()
	stats := active.ResourceStats()
	c.diagnostics.ResourceReused = append([]string(nil), stats.Reused...)
	c.diagnostics.ResourceBuilt = append([]string(nil), stats.Built...)
	c.diagnostics.RestartPolicy = ""
	c.diagnostics.CleanupRequired = false
	c.diagnostics.LastFailurePhase = ""
	c.diagnostics.LastFailureOwner = ""
	c.diagnostics.LastFailureType = ""
	c.diagnostics.Phase = "active"
	c.diagnostics.Since = time.Now()
}

func (c *GenerationCoordinator) setCandidate(id uint64) {
	c.mu.Lock()
	c.diagnostics.CandidateGeneration = id
	c.mu.Unlock()
}

func (c *GenerationCoordinator) setPhase(phase string, changed []string) {
	c.mu.Lock()
	c.diagnostics.Phase = phase
	c.diagnostics.ChangedSections = append([]string(nil), changed...)
	c.diagnostics.Since = time.Now()
	c.mu.Unlock()
}

func (c *GenerationCoordinator) reject(phase string, err error) {
	c.mu.Lock()
	c.diagnostics.State = LifecycleRunning
	c.diagnostics.Ready = true
	c.diagnostics.Phase = "rejected:" + phase
	c.diagnostics.CandidateGeneration = 0
	c.diagnostics.LastFailurePhase = phase
	c.diagnostics.LastFailureOwner = "application-generation"
	c.diagnostics.LastFailureType = fmt.Sprintf("%T", err)
	c.diagnostics.Since = time.Now()
	c.mu.Unlock()
}

func (c *GenerationCoordinator) fail(state LifecycleState, phase string, err error) {
	c.mu.Lock()
	c.diagnostics.State = state
	c.diagnostics.Ready = false
	c.diagnostics.Phase = phase
	c.diagnostics.CandidateGeneration = 0
	c.diagnostics.LastFailurePhase = phase
	c.diagnostics.LastFailureOwner = "application-generation"
	c.diagnostics.LastFailureType = fmt.Sprintf("%T", err)
	c.diagnostics.Since = time.Now()
	c.mu.Unlock()
}

func changedConfigSections(previous, candidate config.Snapshot, bindings []config.Binding) ([]string, error) {
	seen := make(map[string]struct{}, len(bindings))
	changed := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if _, exists := seen[binding.ConfigPath]; exists {
			continue
		}
		seen[binding.ConfigPath] = struct{}{}
		before, err := previous.SectionDigest(binding.ConfigPath)
		if err != nil {
			return nil, err
		}
		after, err := candidate.SectionDigest(binding.ConfigPath)
		if err != nil {
			return nil, err
		}
		if before != after {
			changed = append(changed, binding.ConfigPath)
		}
	}
	return changed, nil
}

func generationOperationError(phase string, generation uint64, err error) error {
	if err == nil {
		return nil
	}
	return &GenerationOperationError{
		Phase: phase, Owner: "application-generation", Generation: generation, Err: err,
	}
}

var _ interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
	ForceStop(context.Context) error
} = (*GenerationCoordinator)(nil)
