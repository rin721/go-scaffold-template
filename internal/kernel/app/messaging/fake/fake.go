// Package fake 提供 deterministic Messaging Provider，用于项目状态机和代际测试。
package fake

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	messagingapp "github.com/rin721/go-scaffold-template/internal/kernel/app/messaging"
	pkgclock "github.com/rin721/go-scaffold-template/pkg/clock"
	pkgmessaging "github.com/rin721/go-scaffold-template/pkg/messaging"
)

// Factory 是可由测试控制的 deterministic Provider Factory。
type Factory struct {
	mu        sync.RWMutex
	providers map[string]*provider
}

// New 创建尚未构造 Provider 的 Factory。
func New() *Factory { return &Factory{providers: make(map[string]*provider)} }

// Kind 返回 fake Driver identity。
func (*Factory) Kind() messagingapp.Driver { return messagingapp.DriverFake }

// Build 构造不启动 goroutine、不访问网络的 fake Provider。
func (f *Factory) Build(
	ctx context.Context,
	name string,
	config messagingapp.ProviderConfig,
	dependencies messagingapp.ProviderDependencies,
) (messagingapp.Provider, error) {
	if ctx == nil || dependencies.Clock == nil || config.Driver != messagingapp.DriverFake {
		return nil, fmt.Errorf("fake messaging provider input is invalid")
	}
	current := &provider{name: name, clock: dependencies.Clock, consumers: make(map[pkgmessaging.ConsumerID]messagingapp.Consumer)}
	current.available.Store(true)
	current.state.Store(pkgmessaging.ProviderReady)
	f.mu.Lock()
	if _, exists := f.providers[name]; exists {
		f.mu.Unlock()
		return nil, fmt.Errorf("fake messaging provider %q is duplicated", name)
	}
	f.providers[name] = current
	f.mu.Unlock()
	return current, nil
}

// SetAvailable 切换命名 Provider 的外部可用性。
func (f *Factory) SetAvailable(name string, available bool) error {
	current, err := f.provider(name)
	if err != nil {
		return err
	}
	current.available.Store(available)
	current.refreshState()
	return nil
}

// Deliver 向指定 Consumer 提交一次 deterministic delivery，并返回统一处置结果。
func (f *Factory) Deliver(
	ctx context.Context,
	name string,
	consumerID pkgmessaging.ConsumerID,
	message pkgmessaging.Message,
	deliveryCount uint64,
) (messagingapp.Disposition, error) {
	current, err := f.provider(name)
	if err != nil {
		return 0, err
	}
	if !current.active.Load() || !current.available.Load() {
		return 0, pkgmessaging.ErrUnavailable
	}
	current.mu.RLock()
	consumer, exists := current.consumers[consumerID]
	current.mu.RUnlock()
	if !exists {
		return 0, fmt.Errorf("fake messaging consumer %q is unknown", consumerID)
	}
	return consumer.Handle(ctx, messagingapp.Incoming{
		Message: message, DeliveryCount: deliveryCount, Redelivered: deliveryCount > 0,
	}), nil
}

// Published 返回命名 Provider 已确认发布的消息副本。
func (f *Factory) Published(name string) ([]pkgmessaging.Message, error) {
	current, err := f.provider(name)
	if err != nil {
		return nil, err
	}
	current.mu.RLock()
	defer current.mu.RUnlock()
	return append([]pkgmessaging.Message(nil), current.published...), nil
}

func (f *Factory) provider(name string) (*provider, error) {
	f.mu.RLock()
	current := f.providers[name]
	f.mu.RUnlock()
	if current == nil {
		return nil, fmt.Errorf("fake messaging provider %q is unknown", name)
	}
	return current, nil
}

type provider struct {
	name  string
	clock pkgclock.Clock

	mu        sync.RWMutex
	consumers map[pkgmessaging.ConsumerID]messagingapp.Consumer
	published []pkgmessaging.Message
	active    atomic.Bool
	available atomic.Bool
	stopped   atomic.Bool
	state     atomic.Value
	confirmed atomic.Uint64
	failed    atomic.Uint64
}

func (*provider) Capabilities() messagingapp.Capabilities {
	return messagingapp.Capabilities{
		PublisherConfirm: true, MandatoryRoute: true, ManualAck: true, DelayedRetry: true, DeadLetter: true,
	}
}

func (p *provider) Bind(consumers []messagingapp.Consumer) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.consumers) != 0 {
		return fmt.Errorf("fake messaging provider is already bound")
	}
	for _, consumer := range consumers {
		if consumer.Handle == nil {
			return fmt.Errorf("fake messaging consumer %q handler is nil", consumer.Binding.ID())
		}
		if _, exists := p.consumers[consumer.Binding.ID()]; exists {
			return fmt.Errorf("fake messaging consumer %q is duplicated", consumer.Binding.ID())
		}
		p.consumers[consumer.Binding.ID()] = consumer
	}
	return nil
}

func (p *provider) Activate(context.Context) error {
	if p.stopped.Load() {
		return pkgmessaging.ErrRetired
	}
	p.active.Store(true)
	p.refreshState()
	return nil
}

func (p *provider) Deactivate(context.Context) error {
	p.active.Store(false)
	p.refreshState()
	return nil
}

func (p *provider) Close(context.Context) error {
	p.active.Store(false)
	p.stopped.Store(true)
	p.state.Store(pkgmessaging.ProviderStopped)
	return nil
}

func (p *provider) Publish(_ context.Context, _ messagingapp.Route, message pkgmessaging.Message) (messagingapp.PublishResult, error) {
	if !p.available.Load() || p.stopped.Load() {
		p.failed.Add(1)
		return messagingapp.PublishResult{}, pkgmessaging.ErrUnavailable
	}
	p.mu.Lock()
	p.published = append(p.published, message)
	p.mu.Unlock()
	p.confirmed.Add(1)
	return messagingapp.PublishResult{ConfirmedAt: p.clock.Now(), Reference: "fake-confirmed"}, nil
}

func (p *provider) Diagnostics() pkgmessaging.ProviderDiagnostics {
	state, _ := p.state.Load().(pkgmessaging.ProviderState)
	return pkgmessaging.ProviderDiagnostics{
		Name: p.name, Driver: string(messagingapp.DriverFake), State: state,
		Ready: state == pkgmessaging.ProviderReady, Confirmed: p.confirmed.Load(), Failed: p.failed.Load(),
	}
}

func (p *provider) refreshState() {
	state := pkgmessaging.ProviderConnecting
	if p.stopped.Load() {
		state = pkgmessaging.ProviderStopped
	} else if p.available.Load() {
		state = pkgmessaging.ProviderReady
	} else {
		state = pkgmessaging.ProviderRecovering
	}
	p.state.Store(state)
}

var _ messagingapp.Factory = (*Factory)(nil)
var _ messagingapp.Provider = (*provider)(nil)
