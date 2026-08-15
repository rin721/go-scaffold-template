package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
)

// RuntimeComponent 是 Kernel 执行 FrozenPlan 时使用的组件状态机边界。
type RuntimeComponent interface {
	ID() ID
	Policy() ReloadPolicy
	Stage(config.Snapshot) (bool, error)
	Build(context.Context) error
	Start(context.Context) error
	Ready(context.Context) error
	PublishInitial()
	DiscardCandidate(context.Context) error
	BeginDrain() (<-chan struct{}, error)
	BeginTerminalDrain() (<-chan struct{}, error)
	Commit()
	Resume()
	Rollback()
	StopPrevious(context.Context) error
	PrepareStop()
	StopCurrent(context.Context) error
	StopPending()
	Ownerships() []OwnershipSnapshot
}

type managedComponent[C, D, I any] struct {
	mu sync.Mutex

	componentID         ID
	configPath          string
	policy              ReloadPolicy
	decode              Decoder[C]
	fixed               func() (C, error)
	resolveDependencies func() (D, error)
	build               Builder[C, D, I]
	lifecycle           lifecycle[I]
	lease               *lease[I]

	currentDigest  string
	pendingDigest  string
	pendingConfig  C
	candidate      *instanceSlot[I]
	retired        *instanceSlot[I]
	stopping       *instanceSlot[I]
	lastTerminal   *OwnershipSnapshot
	nextGeneration uint64
}

func (c *managedComponent[C, D, I]) ID() ID               { return c.componentID }
func (c *managedComponent[C, D, I]) Policy() ReloadPolicy { return c.policy }
func (c *managedComponent[C, D, I]) ConfigPath() string   { return c.configPath }

func (c *managedComponent[C, D, I]) Stage(snapshot config.Snapshot) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.retired != nil || c.stopping != nil || c.candidate != nil {
		return false, fmt.Errorf("component %s has unresolved finalization responsibility", c.ID())
	}
	if c.configPath == "" {
		if c.currentDigest != "" {
			return false, nil
		}
		decoded, err := c.fixed()
		if err != nil {
			return false, fmt.Errorf("decode fixed component %s: %w", c.ID(), err)
		}
		c.pendingConfig = decoded
		c.pendingDigest = "fixed"
		return true, nil
	}
	digest, err := snapshot.SectionDigest(c.configPath)
	if err != nil {
		return false, fmt.Errorf("digest component %s config: %w", c.ID(), err)
	}
	if digest == c.currentDigest {
		return false, nil
	}
	decoded, err := c.decode(snapshot)
	if err != nil {
		return false, fmt.Errorf("decode component %s config: %w", c.ID(), err)
	}
	c.pendingConfig = decoded
	c.pendingDigest = digest
	return true, nil
}

func (c *managedComponent[C, D, I]) Build(ctx context.Context) error {
	c.mu.Lock()
	if c.candidate != nil {
		c.mu.Unlock()
		return fmt.Errorf("component %s candidate already exists", c.ID())
	}
	configuration := c.pendingConfig
	c.mu.Unlock()
	dependencies, err := c.resolveDependencies()
	if err != nil {
		return fmt.Errorf("resolve component %s dependencies: %w", c.ID(), err)
	}
	instance, err := c.build(ctx, configuration, dependencies)
	if err != nil {
		return fmt.Errorf("build component %s: %w", c.ID(), err)
	}
	if isNil(instance) {
		return fmt.Errorf("build component %s returned a nil instance", c.ID())
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.candidate != nil {
		return fmt.Errorf("component %s candidate already exists", c.ID())
	}
	c.nextGeneration++
	c.candidate = newInstanceSlot(c.nextGeneration, instance, FinalizationPhaseCandidate, c.lifecycle.finalizationPolicy)
	return nil
}

func (c *managedComponent[C, D, I]) Start(ctx context.Context) error {
	c.mu.Lock()
	candidate := c.candidate
	c.mu.Unlock()
	if candidate == nil {
		return fmt.Errorf("component %s candidate is missing", c.ID())
	}
	if c.lifecycle.start == nil {
		return nil
	}
	if err := c.lifecycle.start(ctx, candidate.instance); err != nil {
		return fmt.Errorf("start component %s: %w", c.ID(), err)
	}
	return nil
}

func (c *managedComponent[C, D, I]) Ready(ctx context.Context) error {
	c.mu.Lock()
	candidate := c.candidate
	c.mu.Unlock()
	if candidate == nil {
		return fmt.Errorf("component %s candidate is missing", c.ID())
	}
	if c.lifecycle.ready == nil {
		return nil
	}
	if err := c.lifecycle.ready(ctx, candidate.instance); err != nil {
		return fmt.Errorf("ready component %s: %w", c.ID(), err)
	}
	return nil
}

func (c *managedComponent[C, D, I]) PublishInitial() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.candidate == nil {
		panic("component initial publish without candidate")
	}
	c.candidate.transition(FinalizationPhaseCurrent, OwnershipServing)
	c.lease.publishInitial(c.candidate)
	if c.lifecycle.activate != nil {
		c.lifecycle.activate(c.candidate.instance)
	}
	c.currentDigest = c.pendingDigest
	c.candidate = nil
}

func (c *managedComponent[C, D, I]) DiscardCandidate(ctx context.Context) error {
	c.mu.Lock()
	candidate := c.candidate
	c.mu.Unlock()
	if candidate == nil {
		return nil
	}
	if err := finalizeSlot(ctx, candidate, c.lifecycle.terminalFinalizer); err != nil {
		return fmt.Errorf("finalize candidate component %s: %w", c.ID(), err)
	}
	c.mu.Lock()
	if c.candidate == candidate {
		snapshot := candidate.snapshot(c.ID())
		c.lastTerminal = &snapshot
		c.candidate = nil
	}
	c.mu.Unlock()
	return nil
}

func (c *managedComponent[C, D, I]) BeginDrain() (<-chan struct{}, error) {
	return c.lease.beginOrContinueDrain(false)
}

func (c *managedComponent[C, D, I]) BeginTerminalDrain() (<-chan struct{}, error) {
	return c.lease.beginOrContinueDrain(true)
}

func (c *managedComponent[C, D, I]) Commit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.candidate == nil || c.retired != nil {
		panic("component commit has unresolved ownership")
	}
	c.retired = c.lease.replaceWhileDraining(c.candidate)
	c.retired.transition(FinalizationPhaseRetired, OwnershipFinalizationPending)
	c.candidate.transition(FinalizationPhaseCurrent, OwnershipServing)
	if c.lifecycle.activate != nil {
		c.lifecycle.activate(c.candidate.instance)
	}
	c.currentDigest = c.pendingDigest
	c.candidate = nil
}

func (c *managedComponent[C, D, I]) Resume() { c.lease.resume() }

func (c *managedComponent[C, D, I]) Rollback() {
	c.lease.resume()
	c.mu.Lock()
	defer c.mu.Unlock()
	var zero C
	c.pendingConfig = zero
	c.pendingDigest = ""
}

func (c *managedComponent[C, D, I]) StopPrevious(ctx context.Context) error {
	c.mu.Lock()
	retired := c.retired
	c.mu.Unlock()
	if retired == nil {
		return nil
	}
	if err := finalizeSlot(ctx, retired, c.lifecycle.terminalFinalizer); err != nil {
		return fmt.Errorf("finalize retired component %s: %w", c.ID(), err)
	}
	c.mu.Lock()
	if c.retired == retired {
		snapshot := retired.snapshot(c.ID())
		c.lastTerminal = &snapshot
		c.retired = nil
	}
	c.mu.Unlock()
	return nil
}

func (c *managedComponent[C, D, I]) PrepareStop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopping != nil {
		return
	}
	c.stopping = c.lease.takeWhileDraining()
	if c.stopping != nil {
		c.stopping.transition(FinalizationPhaseCurrent, OwnershipFinalizationPending)
	}
}

func (c *managedComponent[C, D, I]) StopCurrent(ctx context.Context) error {
	c.mu.Lock()
	stopping := c.stopping
	c.mu.Unlock()
	if stopping == nil {
		return nil
	}
	if instance, needed := stopping.markDeactivated(); needed && c.lifecycle.deactivate != nil {
		c.lifecycle.deactivate(instance)
	}
	if err := finalizeSlot(ctx, stopping, c.lifecycle.terminalFinalizer); err != nil {
		return fmt.Errorf("finalize current component %s: %w", c.ID(), err)
	}
	c.mu.Lock()
	if c.stopping == stopping {
		snapshot := stopping.snapshot(c.ID())
		c.lastTerminal = &snapshot
		c.stopping = nil
	}
	c.mu.Unlock()
	return nil
}

func (c *managedComponent[C, D, I]) StopPending() { c.lease.stopPending() }

func (c *managedComponent[C, D, I]) Ownerships() []OwnershipSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]OwnershipSnapshot, 0, 5)
	for _, slot := range []*instanceSlot[I]{c.candidate, c.retired, c.stopping} {
		if slot != nil {
			result = append(result, slot.snapshot(c.ID()))
		}
	}
	if current := c.lease.currentSnapshot(c.ID()); current != nil {
		result = append(result, *current)
	}
	if c.lastTerminal != nil && !containsOwnership(result, *c.lastTerminal) {
		result = append(result, *c.lastTerminal)
	}
	return result
}

func containsOwnership(snapshots []OwnershipSnapshot, candidate OwnershipSnapshot) bool {
	for _, snapshot := range snapshots {
		if snapshot.ComponentID == candidate.ComponentID && snapshot.InstanceGeneration == candidate.InstanceGeneration && snapshot.Phase == candidate.Phase {
			return true
		}
	}
	return false
}
