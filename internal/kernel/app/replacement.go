package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

type replacementMode uint8

const (
	replacementStartup replacementMode = iota
	replacementManaged
)

// ReplacementDefinition 声明一个只可用于显式替换内置 Role 的组件。
// 它不会发布独立 Binding，因而不会为同一资源建立第二套租约。
type ReplacementDefinition[TTarget any] struct {
	spec                 Spec
	mode                 replacementMode
	defaults             config.DefaultContract
	markConsumers        func(*Plan)
	dependencyReferences []reference
	instantiate          func(*Plan, *builtinEntry[TTarget]) (RuntimeComponent, TTarget, func(context.Context) error, error)
}

// ManagedReplacement 创建由配置驱动并参与 Kernel 换代事务的替换声明。
func ManagedReplacement[C, D, I, TTarget any](
	spec Spec,
	source ConfiguredSource[C],
	dependencies Dependencies[D],
	build Builder[C, D, I],
	target func(I) (TTarget, error),
	options ...Option[I],
) (ReplacementDefinition[TTarget], error) {
	if err := spec.ValidateConfigured(); err != nil {
		return ReplacementDefinition[TTarget]{}, err
	}
	if source.path != spec.ConfigPath {
		return ReplacementDefinition[TTarget]{}, fmt.Errorf("component %s configured source path %s does not match spec path %s", spec.ID, source.path, spec.ConfigPath)
	}
	if build == nil || target == nil || dependencies.decode == nil {
		return ReplacementDefinition[TTarget]{}, fmt.Errorf("component %s replacement contract is incomplete", spec.ID)
	}
	lifecycle := lifecycle[I]{}
	for index, option := range options {
		if option == nil {
			return ReplacementDefinition[TTarget]{}, fmt.Errorf("component %s option %d is nil", spec.ID, index)
		}
		if err := option(&lifecycle); err != nil {
			return ReplacementDefinition[TTarget]{}, fmt.Errorf("component %s option %d: %w", spec.ID, index, err)
		}
	}
	return ReplacementDefinition[TTarget]{
		spec: spec, mode: replacementManaged, defaults: source.defaults, markConsumers: dependencies.markBuiltinConsumers, dependencyReferences: append([]reference(nil), dependencies.references...),
		instantiate: func(plan *Plan, role *builtinEntry[TTarget]) (RuntimeComponent, TTarget, func(context.Context) error, error) {
			if err := dependencies.validate(plan, len(plan.outputs)); err != nil {
				return nil, *new(TTarget), nil, fmt.Errorf("component %s dependencies: %w", spec.ID, err)
			}
			component := &replacementComponent[C, D, I, TTarget]{
				componentID: spec.ID, configPath: spec.ConfigPath, decode: source.decode,
				resolveDependencies: func() (D, error) { return dependencies.resolve(plan) },
				build:               build, target: target, lifecycle: lifecycle, role: role,
			}
			return component, *new(TTarget), nil, nil
		},
	}, nil
}

// StartupReplacement 创建仅在内置 Role 所属阶段冻结一次的替换声明。
func StartupReplacement[C, D, I, TTarget any](
	spec Spec,
	configuration C,
	dependencies Dependencies[D],
	build Builder[C, D, I],
	target func(I) (TTarget, error),
	options ...Option[I],
) (ReplacementDefinition[TTarget], error) {
	if spec.ID == "" {
		return ReplacementDefinition[TTarget]{}, fmt.Errorf("component id is required")
	}
	if spec.ConfigPath != "" {
		return ReplacementDefinition[TTarget]{}, fmt.Errorf("startup replacement %s must not own a runtime config path", spec.ID)
	}
	if build == nil || target == nil || dependencies.decode == nil {
		return ReplacementDefinition[TTarget]{}, fmt.Errorf("component %s replacement contract is incomplete", spec.ID)
	}
	lifecycle := lifecycle[I]{}
	for index, option := range options {
		if option == nil {
			return ReplacementDefinition[TTarget]{}, fmt.Errorf("component %s option %d is nil", spec.ID, index)
		}
		if err := option(&lifecycle); err != nil {
			return ReplacementDefinition[TTarget]{}, fmt.Errorf("component %s option %d: %w", spec.ID, index, err)
		}
	}
	return ReplacementDefinition[TTarget]{
		spec: spec, mode: replacementStartup, markConsumers: dependencies.markBuiltinConsumers, dependencyReferences: append([]reference(nil), dependencies.references...),
		instantiate: func(plan *Plan, role *builtinEntry[TTarget]) (RuntimeComponent, TTarget, func(context.Context) error, error) {
			if err := dependencies.validatePhase(plan, len(plan.outputs), role.phase); err != nil {
				return nil, *new(TTarget), nil, fmt.Errorf("component %s dependencies: %w", spec.ID, err)
			}
			resolved, err := dependencies.resolve(plan)
			if err != nil {
				return nil, *new(TTarget), nil, err
			}
			instance, err := build(context.Background(), configuration, resolved)
			if err != nil {
				return nil, *new(TTarget), nil, err
			}
			if isNil(instance) {
				return nil, *new(TTarget), nil, fmt.Errorf("startup replacement %s build returned a nil instance", spec.ID)
			}
			projected, err := target(instance)
			if err != nil || isNil(projected) {
				var cleanupErr error
				if lifecycle.stop != nil && !isNil(instance) {
					cleanupErr = lifecycle.stop(context.Background(), instance)
				}
				if err == nil {
					err = fmt.Errorf("startup replacement %s target is nil", spec.ID)
				}
				return nil, *new(TTarget), nil, errors.Join(err, cleanupErr)
			}
			close := func(ctx context.Context) error {
				if lifecycle.stop == nil {
					return nil
				}
				return lifecycle.stop(ctx, instance)
			}
			return nil, projected, close, nil
		},
	}, nil
}

// Replace 把唯一显式 replacer 原子登记到同一 Plan 的内置 Role。
func Replace[TTarget any](plan *Plan, role BuiltinRole[TTarget], replacement ReplacementDefinition[TTarget]) error {
	if plan == nil || role.plan == nil || role.entry == nil {
		return fmt.Errorf("builtin replacement arguments are invalid")
	}
	if plan != role.plan {
		return fmt.Errorf("builtin role %s belongs to another plan", role.entry.id)
	}
	if plan.state != planOpen {
		return fmt.Errorf("component plan is frozen")
	}
	if role.entry.replacer {
		return fmt.Errorf("builtin role %s already has a replacement", role.entry.id)
	}
	if role.entry.consumerSet {
		return fmt.Errorf("builtin role %s replacement must be declared before its first consumer", role.entry.id)
	}
	for _, reference := range replacement.dependencies() {
		if reference.plan == plan && reference.index == role.entry.rootIndex && plan.rootRoles[reference.index] == role.entry {
			return fmt.Errorf("builtin role %s replacement cannot depend on its own root binding", role.entry.id)
		}
	}
	if replacement.instantiate == nil || replacement.spec.ID == "" {
		return fmt.Errorf("builtin role %s replacement definition is invalid", role.entry.id)
	}
	if role.entry.policy == Fixed ||
		(role.entry.policy == StartupReplace && replacement.mode != replacementStartup) ||
		(role.entry.policy == RuntimeTransaction && replacement.mode != replacementManaged) {
		return fmt.Errorf("builtin role %s policy %d rejects replacement mode %d", role.entry.id, role.entry.policy, replacement.mode)
	}
	if err := plan.validateIdentity(replacement.spec.ID, replacement.spec.ConfigPath); err != nil {
		return err
	}
	if replacement.mode == replacementStartup && !role.entry.active {
		role.entry.startupReplacement = func() (TTarget, func(context.Context) error, error) {
			_, target, close, err := replacement.instantiate(plan, role.entry)
			return target, close, err
		}
		plan.ids[replacement.spec.ID] = struct{}{}
		if replacement.markConsumers != nil {
			replacement.markConsumers(plan)
		}
		role.entry.replacer = true
		return nil
	}
	component, target, close, err := replacement.instantiate(plan, role.entry)
	if err != nil {
		return fmt.Errorf("instantiate replacement %s for builtin role %s: %w", replacement.spec.ID, role.entry.id, err)
	}
	if replacement.mode == replacementManaged && component == nil {
		return fmt.Errorf("replacement %s runtime node is nil", replacement.spec.ID)
	}
	if replacement.mode == replacementStartup {
		role.entry.target = target
		role.entry.replacementStop = close
	}
	plan.ids[replacement.spec.ID] = struct{}{}
	if replacement.spec.ConfigPath != "" {
		plan.configPaths[replacement.spec.ConfigPath] = replacement.spec.ID
	}
	if replacement.markConsumers != nil {
		replacement.markConsumers(plan)
	}
	if component != nil {
		plan.components = append(plan.components, component)
	}
	if replacement.defaults != nil {
		plan.defaults = append(plan.defaults, config.Binding{CapabilityID: string(replacement.spec.ID), ConfigPath: replacement.spec.ConfigPath, Contract: replacement.defaults})
	}
	role.entry.replacer = true
	return nil
}

func (definition ReplacementDefinition[TTarget]) dependencies() []reference {
	if definition.dependencyReferences == nil {
		return nil
	}
	return append([]reference(nil), definition.dependencyReferences...)
}

type replacementComponent[C, D, I, TTarget any] struct {
	componentID                           ID
	configPath                            string
	decode                                Decoder[C]
	resolveDependencies                   func() (D, error)
	build                                 Builder[C, D, I]
	target                                func(I) (TTarget, error)
	lifecycle                             lifecycle[I]
	role                                  *builtinEntry[TTarget]
	currentDigest, pendingDigest          string
	pendingConfig                         C
	candidate, current, previous          I
	candidateTarget, previousTarget       TTarget
	hasCandidate, hasCurrent, hasPrevious bool
}

func (c *replacementComponent[C, D, I, T]) ID() ID               { return c.componentID }
func (c *replacementComponent[C, D, I, T]) Policy() ReloadPolicy { return KernelInstanceSwap }
func (c *replacementComponent[C, D, I, T]) ConfigPath() string   { return c.configPath }
func (c *replacementComponent[C, D, I, T]) Stage(snapshot config.Snapshot) (bool, error) {
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
	c.pendingConfig, c.pendingDigest = decoded, digest
	return true, nil
}
func (c *replacementComponent[C, D, I, T]) Build(ctx context.Context) error {
	deps, err := c.resolveDependencies()
	if err != nil {
		return fmt.Errorf("resolve component %s dependencies: %w", c.ID(), err)
	}
	instance, err := c.build(ctx, c.pendingConfig, deps)
	if err != nil {
		return fmt.Errorf("build component %s: %w", c.ID(), err)
	}
	if isNil(instance) {
		return fmt.Errorf("build component %s returned a nil instance", c.ID())
	}
	target, err := c.target(instance)
	if err != nil {
		return errors.Join(fmt.Errorf("project component %s target: %w", c.ID(), err), c.stop(context.WithoutCancel(ctx), instance))
	}
	if isNil(target) {
		return errors.Join(fmt.Errorf("project component %s target is nil", c.ID()), c.stop(context.WithoutCancel(ctx), instance))
	}
	c.candidate, c.candidateTarget, c.hasCandidate = instance, target, true
	return nil
}
func (c *replacementComponent[C, D, I, T]) Start(ctx context.Context) error {
	if c.lifecycle.start == nil {
		return nil
	}
	if err := c.lifecycle.start(ctx, c.candidate); err != nil {
		return fmt.Errorf("start component %s: %w", c.ID(), err)
	}
	return nil
}
func (c *replacementComponent[C, D, I, T]) Ready(ctx context.Context) error {
	if c.lifecycle.ready == nil {
		return nil
	}
	if err := c.lifecycle.ready(ctx, c.candidate); err != nil {
		return fmt.Errorf("ready component %s: %w", c.ID(), err)
	}
	return nil
}
func (c *replacementComponent[C, D, I, T]) PublishInitial() {
	c.role.slot.replaceServing(c.candidateTarget)
	c.current, c.hasCurrent = c.candidate, true
	c.currentDigest = c.pendingDigest
	c.clearCandidate()
}
func (c *replacementComponent[C, D, I, T]) DiscardCandidate(ctx context.Context) error {
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
func (c *replacementComponent[C, D, I, T]) BeginDrain() (<-chan struct{}, error) {
	return c.role.slot.beginDrain()
}
func (c *replacementComponent[C, D, I, T]) Commit() {
	c.previousTarget = c.role.slot.replaceWhileDraining(c.candidateTarget)
	c.previous, c.hasPrevious = c.current, c.hasCurrent
	c.current, c.hasCurrent = c.candidate, true
	c.currentDigest = c.pendingDigest
	c.clearCandidate()
}
func (c *replacementComponent[C, D, I, T]) Resume() { c.role.slot.resume() }
func (c *replacementComponent[C, D, I, T]) Rollback() {
	c.role.slot.resume()
	var zero C
	c.pendingConfig = zero
	c.pendingDigest = ""
}
func (c *replacementComponent[C, D, I, T]) StopPrevious(ctx context.Context) error {
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
func (c *replacementComponent[C, D, I, T]) PrepareStop() {
	c.previousTarget = c.role.slot.replaceWhileDraining(c.role.baseline)
	c.previous, c.hasPrevious = c.current, c.hasCurrent
	var zero I
	c.current = zero
	c.hasCurrent = false
	c.role.slot.resume()
}
func (c *replacementComponent[C, D, I, T]) StopCurrent(ctx context.Context) error {
	return c.StopPrevious(ctx)
}
func (c *replacementComponent[C, D, I, T]) StopPending() {
	if c.hasCandidate {
		_ = c.stop(context.Background(), c.candidate)
		c.clearCandidate()
	}
}
func (c *replacementComponent[C, D, I, T]) stop(ctx context.Context, instance I) error {
	if c.lifecycle.stop == nil {
		return nil
	}
	return c.lifecycle.stop(ctx, instance)
}
func (c *replacementComponent[C, D, I, T]) clearCandidate() {
	var zi I
	var zt T
	c.candidate = zi
	c.candidateTarget = zt
	c.hasCandidate = false
}
