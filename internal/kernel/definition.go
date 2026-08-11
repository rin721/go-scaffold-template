package kernel

import (
	"context"
	"fmt"
	"reflect"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

// ID 是 kernel 内基础能力的稳定标识。
type ID string

// Decoder 从完整快照中解码并校验一个能力的 typed 配置。
type Decoder[C any] func(config.Snapshot) (C, error)

// Definition 描述一个彼此独立、可重新构造的基础能力。
type Definition[C, T any] struct {
	ID         ID
	ConfigPath string
	Decode     Decoder[C]
	Defaults   config.DefaultContract
	Builder    Builder[C, T]
	Hooks      InstanceHooks[T]
	Activation ActivationHooks[T]
}

// Registration 是能力成功登记后返回的稳定 Access 和默认配置绑定。
type Registration[T any] struct {
	Access   *Handle[T]
	Defaults config.Binding
}

// Register 在 Kernel 启动前登记能力，并返回不可变的登记结果。
func Register[C, T any](runtime *Kernel, definition Definition[C, T]) (Registration[T], error) {
	if runtime == nil {
		return Registration[T]{}, fmt.Errorf("kernel is nil")
	}
	if definition.ID == "" {
		return Registration[T]{}, fmt.Errorf("kernel definition id is required")
	}
	if definition.ConfigPath == "" {
		return Registration[T]{}, fmt.Errorf("kernel definition %s config path is required", definition.ID)
	}
	if definition.Decode == nil {
		return Registration[T]{}, fmt.Errorf("kernel definition %s decoder is nil", definition.ID)
	}
	if isNilInterface(definition.Defaults) {
		return Registration[T]{}, fmt.Errorf("kernel definition %s defaults contract is nil", definition.ID)
	}
	if isNilInterface(definition.Builder) {
		return Registration[T]{}, fmt.Errorf("kernel definition %s builder is nil", definition.ID)
	}
	if isNilInterface(definition.Hooks) {
		return Registration[T]{}, fmt.Errorf("kernel definition %s instance hooks are nil", definition.ID)
	}

	handle := newHandle[T]()
	component := &typedComponent[C, T]{definition: definition, handle: handle}
	if err := runtime.register(component); err != nil {
		return Registration[T]{}, err
	}
	return Registration[T]{
		Access: handle,
		Defaults: config.Binding{
			CapabilityID: string(definition.ID),
			ConfigPath:   definition.ConfigPath,
			Contract:     definition.Defaults,
		},
	}, nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

type component interface {
	id() ID
	stage(config.Snapshot) (bool, error)
	buildStart(context.Context) error
	discardCandidate(context.Context) error
	publishInitial()
	beginDrain() (<-chan struct{}, error)
	prepareCommit()
	publish()
	rollback()
	stopPrevious(context.Context) error
	prepareStop()
	stopCurrent(context.Context) error
	stopPending()
}

type typedComponent[C, T any] struct {
	definition Definition[C, T]
	handle     *Handle[T]

	currentDigest string
	pendingDigest string
	pendingConfig C
	candidate     T
	hasCandidate  bool
	previous      T
	hasPrevious   bool
}

func (c *typedComponent[C, T]) id() ID {
	return c.definition.ID
}

func (c *typedComponent[C, T]) stage(snapshot config.Snapshot) (bool, error) {
	digest, err := snapshot.SectionDigest(c.definition.ConfigPath)
	if err != nil {
		return false, fmt.Errorf("digest component %s config: %w", c.id(), err)
	}
	if digest == c.currentDigest {
		return false, nil
	}
	decoded, err := c.definition.Decode(snapshot)
	if err != nil {
		return false, fmt.Errorf("decode component %s config: %w", c.id(), err)
	}
	c.pendingConfig = decoded
	c.pendingDigest = digest
	return true, nil
}

func (c *typedComponent[C, T]) buildStart(ctx context.Context) error {
	instance, err := c.definition.Builder.Build(ctx, c.pendingConfig)
	if err != nil {
		return fmt.Errorf("build component %s: %w", c.id(), err)
	}
	c.candidate = instance
	c.hasCandidate = true
	if err := c.definition.Hooks.Start(ctx, instance); err != nil {
		return fmt.Errorf("start component %s: %w", c.id(), err)
	}
	return nil
}

func (c *typedComponent[C, T]) discardCandidate(ctx context.Context) error {
	if !c.hasCandidate {
		return nil
	}
	err := c.definition.Hooks.Stop(ctx, c.candidate)
	var zero T
	c.candidate = zero
	c.hasCandidate = false
	if err != nil {
		return fmt.Errorf("stop candidate component %s: %w", c.id(), err)
	}
	return nil
}

func (c *typedComponent[C, T]) publishInitial() {
	c.handle.publishInitial(c.candidate)
	if c.definition.Activation != nil {
		c.definition.Activation.Activate(c.candidate)
	}
	c.currentDigest = c.pendingDigest
	var zero T
	c.candidate = zero
	c.hasCandidate = false
}

func (c *typedComponent[C, T]) beginDrain() (<-chan struct{}, error) {
	return c.handle.beginDrain()
}

func (c *typedComponent[C, T]) prepareCommit() {
	c.previous = c.handle.replaceWhileDraining(c.candidate)
	c.hasPrevious = true
	if c.definition.Activation != nil {
		c.definition.Activation.Activate(c.candidate)
	}
	c.currentDigest = c.pendingDigest
	var zero T
	c.candidate = zero
	c.hasCandidate = false
}

func (c *typedComponent[C, T]) publish() {
	c.handle.resume()
}

func (c *typedComponent[C, T]) rollback() {
	c.handle.resume()
	var zeroC C
	c.pendingConfig = zeroC
	c.pendingDigest = ""
}

func (c *typedComponent[C, T]) stopPrevious(ctx context.Context) error {
	if !c.hasPrevious {
		return nil
	}
	err := c.definition.Hooks.Stop(ctx, c.previous)
	var zero T
	c.previous = zero
	c.hasPrevious = false
	if err != nil {
		return fmt.Errorf("stop previous component %s: %w", c.id(), err)
	}
	return nil
}

func (c *typedComponent[C, T]) prepareStop() {
	c.previous = c.handle.stopWhileDraining()
	c.hasPrevious = true
}

func (c *typedComponent[C, T]) stopCurrent(ctx context.Context) error {
	if c.definition.Activation != nil {
		c.definition.Activation.Deactivate(c.previous)
	}
	return c.stopPrevious(ctx)
}

func (c *typedComponent[C, T]) stopPending() {
	c.handle.stopPending()
}
