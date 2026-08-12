package app

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
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
	Commit()
	Resume()
	Rollback()
	StopPrevious(context.Context) error
	PrepareStop()
	StopCurrent(context.Context) error
	StopPending()
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

	currentDigest string
	pendingDigest string
	pendingConfig C
	candidate     I
	hasCandidate  bool
	previous      I
	hasPrevious   bool
}

func (c *managedComponent[C, D, I]) ID() ID               { return c.componentID }
func (c *managedComponent[C, D, I]) Policy() ReloadPolicy { return c.policy }
func (c *managedComponent[C, D, I]) ConfigPath() string   { return c.configPath }

func (c *managedComponent[C, D, I]) Stage(snapshot config.Snapshot) (bool, error) {
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
	c.candidate = instance
	c.hasCandidate = true
	return nil
}

func (c *managedComponent[C, D, I]) Start(ctx context.Context) error {
	if c.lifecycle.start == nil {
		return nil
	}
	if err := c.lifecycle.start(ctx, c.candidate); err != nil {
		return fmt.Errorf("start component %s: %w", c.ID(), err)
	}
	return nil
}

func (c *managedComponent[C, D, I]) Ready(ctx context.Context) error {
	if c.lifecycle.ready == nil {
		return nil
	}
	if err := c.lifecycle.ready(ctx, c.candidate); err != nil {
		return fmt.Errorf("ready component %s: %w", c.ID(), err)
	}
	return nil
}

func (c *managedComponent[C, D, I]) PublishInitial() {
	c.lease.publishInitial(c.candidate)
	if c.lifecycle.activate != nil {
		c.lifecycle.activate(c.candidate)
	}
	c.currentDigest = c.pendingDigest
	c.clearCandidate()
}

func (c *managedComponent[C, D, I]) DiscardCandidate(ctx context.Context) error {
	if !c.hasCandidate {
		return nil
	}
	err := c.stop(ctx, c.candidate)
	c.clearCandidate()
	if err != nil {
		return fmt.Errorf("stop candidate component %s: %w", c.ID(), err)
	}
	return nil
}

func (c *managedComponent[C, D, I]) BeginDrain() (<-chan struct{}, error) {
	return c.lease.beginDrain()
}

func (c *managedComponent[C, D, I]) Commit() {
	c.previous = c.lease.replaceWhileDraining(c.candidate)
	c.hasPrevious = true
	if c.lifecycle.activate != nil {
		c.lifecycle.activate(c.candidate)
	}
	c.currentDigest = c.pendingDigest
	c.clearCandidate()
}

func (c *managedComponent[C, D, I]) Resume() { c.lease.resume() }

func (c *managedComponent[C, D, I]) Rollback() {
	c.lease.resume()
	var zero C
	c.pendingConfig = zero
	c.pendingDigest = ""
}

func (c *managedComponent[C, D, I]) StopPrevious(ctx context.Context) error {
	if !c.hasPrevious {
		return nil
	}
	err := c.stop(ctx, c.previous)
	var zero I
	c.previous = zero
	c.hasPrevious = false
	if err != nil {
		return fmt.Errorf("stop previous component %s: %w", c.ID(), err)
	}
	return nil
}

func (c *managedComponent[C, D, I]) PrepareStop() {
	c.previous = c.lease.takeWhileDraining()
	c.hasPrevious = true
}

func (c *managedComponent[C, D, I]) StopCurrent(ctx context.Context) error {
	if c.lifecycle.deactivate != nil {
		c.lifecycle.deactivate(c.previous)
	}
	return c.StopPrevious(ctx)
}

func (c *managedComponent[C, D, I]) StopPending() { c.lease.stopPending() }

func (c *managedComponent[C, D, I]) stop(ctx context.Context, instance I) error {
	if c.lifecycle.stop == nil {
		return nil
	}
	return c.lifecycle.stop(ctx, instance)
}

func (c *managedComponent[C, D, I]) clearCandidate() {
	var zero I
	c.candidate = zero
	c.hasCandidate = false
}
