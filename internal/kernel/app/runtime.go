package app

import (
	"context"
	"fmt"

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
	Finalizations() []FinalizationSnapshot
}

type managedComponent[C, D, I any] struct {
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
	nextGeneration uint64
}

func (c *managedComponent[C, D, I]) ID() ID               { return c.componentID }
func (c *managedComponent[C, D, I]) Policy() ReloadPolicy { return c.policy }
func (c *managedComponent[C, D, I]) ConfigPath() string   { return c.configPath }

func (c *managedComponent[C, D, I]) Stage(snapshot config.Snapshot) (bool, error) {
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
	if c.candidate != nil {
		return fmt.Errorf("component %s candidate already exists", c.ID())
	}
	dependencies, err := c.resolveDependencies()
	if err != nil {
		return fmt.Errorf("resolve component %s dependencies: %w", c.ID(), err)
	}
	instance, err := c.build(ctx, c.pendingConfig, dependencies)
	if err != nil {
		return fmt.Errorf("build component %s: %w", c.ID(), err)
	}
	if isNil(instance) {
		return fmt.Errorf("build component %s returned a nil instance", c.ID())
	}
	c.nextGeneration++
	c.candidate = &instanceSlot[I]{
		generation: c.nextGeneration,
		instance:   instance,
		phase:      FinalizationPhaseCandidate,
		state:      FinalizationPending,
	}
	return nil
}

func (c *managedComponent[C, D, I]) Start(ctx context.Context) error {
	if c.candidate == nil {
		return fmt.Errorf("component %s candidate is missing", c.ID())
	}
	if c.lifecycle.start == nil {
		return nil
	}
	if err := c.lifecycle.start(ctx, c.candidate.instance); err != nil {
		return fmt.Errorf("start component %s: %w", c.ID(), err)
	}
	return nil
}

func (c *managedComponent[C, D, I]) Ready(ctx context.Context) error {
	if c.candidate == nil {
		return fmt.Errorf("component %s candidate is missing", c.ID())
	}
	if c.lifecycle.ready == nil {
		return nil
	}
	if err := c.lifecycle.ready(ctx, c.candidate.instance); err != nil {
		return fmt.Errorf("ready component %s: %w", c.ID(), err)
	}
	return nil
}

func (c *managedComponent[C, D, I]) PublishInitial() {
	if c.candidate == nil {
		panic("component initial publish without candidate")
	}
	c.candidate.phase = FinalizationPhaseCurrent
	c.lease.publishInitial(c.candidate)
	if c.lifecycle.activate != nil {
		c.lifecycle.activate(c.candidate.instance)
	}
	c.currentDigest = c.pendingDigest
	c.candidate = nil
}

func (c *managedComponent[C, D, I]) DiscardCandidate(ctx context.Context) error {
	if c.candidate == nil {
		return nil
	}
	if err := finalizeSlot(ctx, c.candidate, c.lifecycle.terminalFinalizer); err != nil {
		return fmt.Errorf("finalize candidate component %s: %w", c.ID(), err)
	}
	c.candidate = nil
	return nil
}

func (c *managedComponent[C, D, I]) BeginDrain() (<-chan struct{}, error) {
	return c.lease.beginOrContinueDrain(false)
}

func (c *managedComponent[C, D, I]) BeginTerminalDrain() (<-chan struct{}, error) {
	return c.lease.beginOrContinueDrain(true)
}

func (c *managedComponent[C, D, I]) Commit() {
	if c.candidate == nil || c.retired != nil {
		panic("component commit has unresolved ownership")
	}
	c.retired = c.lease.replaceWhileDraining(c.candidate)
	c.retired.phase = FinalizationPhaseRetired
	c.retired.state = FinalizationPending
	c.candidate.phase = FinalizationPhaseCurrent
	if c.lifecycle.activate != nil {
		c.lifecycle.activate(c.candidate.instance)
	}
	c.currentDigest = c.pendingDigest
	c.candidate = nil
}

func (c *managedComponent[C, D, I]) Resume() { c.lease.resume() }

func (c *managedComponent[C, D, I]) Rollback() {
	c.lease.resume()
	var zero C
	c.pendingConfig = zero
	c.pendingDigest = ""
}

func (c *managedComponent[C, D, I]) StopPrevious(ctx context.Context) error {
	if c.retired == nil {
		return nil
	}
	if err := finalizeSlot(ctx, c.retired, c.lifecycle.terminalFinalizer); err != nil {
		return fmt.Errorf("finalize retired component %s: %w", c.ID(), err)
	}
	c.retired = nil
	return nil
}

func (c *managedComponent[C, D, I]) PrepareStop() {
	if c.stopping != nil {
		return
	}
	c.stopping = c.lease.takeWhileDraining()
	if c.stopping != nil {
		c.stopping.phase = FinalizationPhaseCurrent
		c.stopping.state = FinalizationPending
	}
}

func (c *managedComponent[C, D, I]) StopCurrent(ctx context.Context) error {
	if c.stopping == nil {
		return nil
	}
	if !c.stopping.deactivated && c.lifecycle.deactivate != nil {
		c.lifecycle.deactivate(c.stopping.instance)
		c.stopping.deactivated = true
	}
	if err := finalizeSlot(ctx, c.stopping, c.lifecycle.terminalFinalizer); err != nil {
		return fmt.Errorf("finalize current component %s: %w", c.ID(), err)
	}
	c.stopping = nil
	return nil
}

func (c *managedComponent[C, D, I]) StopPending() { c.lease.stopPending() }

func (c *managedComponent[C, D, I]) Finalizations() []FinalizationSnapshot {
	result := make([]FinalizationSnapshot, 0, 3)
	for _, slot := range []*instanceSlot[I]{c.candidate, c.retired, c.stopping} {
		if slot != nil {
			result = append(result, slot.snapshot(c.ID()))
		}
	}
	return result
}
