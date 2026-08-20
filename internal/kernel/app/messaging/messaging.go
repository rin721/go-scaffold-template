// Package messaging 装配 generation-owned 消息 Provider、统一 Publisher 与 Consumer 可靠性治理。
package messaging

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	pkgclock "github.com/rin721/go-scaffold-template/pkg/clock"
	pkgexecution "github.com/rin721/go-scaffold-template/pkg/execution"
	"github.com/rin721/go-scaffold-template/pkg/fault"
	"github.com/rin721/go-scaffold-template/pkg/health"
	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
	pkgmessaging "github.com/rin721/go-scaffold-template/pkg/messaging"
	pkgobservability "github.com/rin721/go-scaffold-template/pkg/observability"
	"github.com/rin721/go-scaffold-template/pkg/resilience"
)

const (
	publishWorkKind = "message.publish"
	consumeWorkKind = "message.consume"
)

// Dependencies 是 Messaging Component 的显式项目能力输入。
type Dependencies struct {
	Generation      uint64
	Logger          pkglogger.Logger
	Clock           pkgclock.Clock
	Execution       pkgexecution.OperationExecutor
	ExecutionHealth func() (health.Result, error)
	Telemetry       pkgobservability.Telemetry
	Factories       []Factory
}

// Output 分离业务可注入的 Publisher 与只供 composition 使用的 Control。
type Output struct {
	Publisher pkgmessaging.Publisher
	Control   Control
}

// Control 管理 candidate-local Catalog 冻结、代际准入和诊断。
type Control interface {
	Freeze(pkgmessaging.Catalog) error
	OpenPublisher(context.Context) error
	Activate(context.Context) error
	Deactivate(context.Context) error
	Diagnostics(context.Context) (pkgmessaging.Diagnostics, error)
	Health(context.Context) (health.Result, error)
}

type publisher struct{ delegate app.Lease[*resource] }
type control struct{ delegate app.Lease[*resource] }

// Definition 构造由 Application Generation 持有的 Messaging Component 声明。
func Definition(dependencies Dependencies) (app.Definition[Output], error) {
	source, err := app.Configured(ConfigPath, decode, defaults{})
	if err != nil {
		return app.Definition[Output]{}, err
	}
	return app.ManagedConfigured(
		ID, source, app.FixedDependencies(dependencies), build, app.Leased(newOutput), app.KernelInstanceSwap,
		app.WithReady(ready), app.WithTerminalFinalizer(stop),
	)
}

func newOutput(delegate app.Lease[*resource]) (Output, error) {
	if delegate == nil {
		return Output{}, fmt.Errorf("messaging lease is nil")
	}
	return Output{Publisher: &publisher{delegate: delegate}, Control: &control{delegate: delegate}}, nil
}

type resource struct {
	config         Config
	dependencies   Dependencies
	providers      map[string]Provider
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	admissionMu    sync.Mutex
	executionReady atomic.Bool

	mu              sync.RWMutex
	frozen          bool
	active          bool // Publisher admission；旧代 HTTP 排空期间保持开放。
	consuming       bool
	admitted        bool
	admissionCancel context.CancelFunc
	retired         bool
	producers       map[pkgmessaging.ProducerID]producerRuntime
	consumers       []*consumerRuntime
}

type producerRuntime struct {
	binding  pkgmessaging.ProducerBinding
	contract pkgmessaging.Contract
	route    Route
	provider Provider
}

type consumerRuntime struct {
	owner    *resource
	binding  pkgmessaging.ConsumerBinding
	route    Route
	provider Provider

	active       atomic.Bool
	inFlight     atomic.Int64
	redelivered  atomic.Uint64
	acknowledged atomic.Uint64
	rejected     atomic.Uint64
	deadLettered atomic.Uint64
	lastError    atomic.Value
}

func build(ctx context.Context, cfg Config, dependencies Dependencies) (*resource, error) {
	if ctx == nil {
		return nil, app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if dependencies.Generation == 0 || dependencies.Logger == nil || dependencies.Clock == nil ||
		dependencies.Execution == nil || dependencies.ExecutionHealth == nil || dependencies.Telemetry == nil {
		return nil, fmt.Errorf("messaging dependencies are incomplete")
	}
	factories := make(map[Driver]Factory, len(dependencies.Factories))
	for index, factory := range dependencies.Factories {
		if factory == nil {
			return nil, fmt.Errorf("messaging factory %d is nil", index)
		}
		kind := factory.Kind()
		if kind == "" {
			return nil, fmt.Errorf("messaging factory %d kind is empty", index)
		}
		if _, exists := factories[kind]; exists {
			return nil, fmt.Errorf("messaging factory kind %q is duplicated", kind)
		}
		factories[kind] = factory
	}
	ownedCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	result := &resource{
		config: cfg, dependencies: dependencies, providers: make(map[string]Provider),
		producers: make(map[pkgmessaging.ProducerID]producerRuntime), ctx: ownedCtx, cancel: cancel,
	}
	result.executionReady.Store(true)
	if !cfg.Enabled {
		return result, nil
	}
	for _, name := range cfg.SortedProviderNames() {
		providerConfig := cfg.Providers[name]
		factory := factories[providerConfig.Driver]
		if factory == nil {
			return nil, fmt.Errorf("messaging provider %q has no factory for driver %q", name, providerConfig.Driver)
		}
		provider, err := factory.Build(ctx, name, providerConfig, ProviderDependencies{
			Generation: dependencies.Generation,
			Logger:     dependencies.Logger, Clock: dependencies.Clock, Recovery: cfg.Recovery,
		})
		if err != nil {
			return nil, fmt.Errorf("build messaging provider %q: %w", name, err)
		}
		if provider == nil {
			return nil, fmt.Errorf("build messaging provider %q returned nil", name)
		}
		result.providers[name] = provider
	}
	return result, nil
}

func ready(ctx context.Context, current *resource) error {
	if ctx == nil {
		return app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if current == nil || current.providers == nil {
		return fmt.Errorf("messaging resource is incomplete")
	}
	return nil
}

func stop(ctx context.Context, current *resource) error {
	if current == nil {
		return nil
	}
	if ctx == nil {
		return app.ErrNilContext
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, current.config.ShutdownTimeout)
	defer shutdownCancel()
	current.mu.Lock()
	current.active = false
	current.consuming = false
	current.retired = true
	for _, consumer := range current.consumers {
		consumer.active.Store(false)
	}
	if current.admissionCancel != nil {
		current.admissionCancel()
	}
	current.mu.Unlock()
	current.cancel()
	current.wg.Wait()
	var joined error
	names := make([]string, 0, len(current.providers))
	for name := range current.providers {
		names = append(names, name)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, name := range names {
		joined = errors.Join(joined, current.providers[name].Close(shutdownCtx))
	}
	return joined
}

func (c *control) Freeze(catalog pkgmessaging.Catalog) error {
	return c.delegate.Use(context.Background(), func(current *resource) error { return current.freeze(catalog) })
}

func (c *control) Activate(ctx context.Context) error {
	return c.delegate.Use(ctx, func(current *resource) error { return current.activate(ctx) })
}

func (c *control) OpenPublisher(ctx context.Context) error {
	return c.delegate.Use(ctx, func(current *resource) error { return current.openPublisher() })
}

func (c *control) Deactivate(ctx context.Context) error {
	return c.delegate.Use(ctx, func(current *resource) error { return current.deactivate(ctx) })
}

func (c *control) Diagnostics(ctx context.Context) (pkgmessaging.Diagnostics, error) {
	var diagnostics pkgmessaging.Diagnostics
	err := c.delegate.Use(ctx, func(current *resource) error {
		diagnostics = current.diagnostics()
		return nil
	})
	return diagnostics, err
}

func (c *control) Health(ctx context.Context) (health.Result, error) {
	var result health.Result
	err := c.delegate.Use(ctx, func(current *resource) error {
		result = current.health()
		return nil
	})
	return result, err
}

func (p *publisher) Publish(ctx context.Context, producerID pkgmessaging.ProducerID, message pkgmessaging.Message) (pkgmessaging.Receipt, error) {
	if ctx == nil {
		return pkgmessaging.Receipt{}, fmt.Errorf("%w: nil context", pkgmessaging.ErrInvalidMessage)
	}
	var receipt pkgmessaging.Receipt
	err := p.delegate.Use(ctx, func(current *resource) error {
		var err error
		receipt, err = current.publish(ctx, producerID, message)
		return err
	})
	return receipt, err
}

func (r *resource) freeze(catalog pkgmessaging.Catalog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return fmt.Errorf("messaging catalog is already frozen")
	}
	if r.active || r.retired {
		return pkgmessaging.ErrNotActive
	}
	if !r.config.Enabled {
		if len(catalog.Contracts()) != 0 || len(catalog.Producers()) != 0 || len(catalog.Consumers()) != 0 {
			return fmt.Errorf("disabled messaging cannot accept module bindings")
		}
		r.frozen = true
		return nil
	}
	contracts := make(map[pkgmessaging.ContractRef]pkgmessaging.Contract)
	for _, contract := range catalog.Contracts() {
		contracts[contract.Ref()] = contract
	}
	usedRoutes := make(map[pkgmessaging.RouteID]struct{})
	for _, binding := range catalog.Producers() {
		resolved, err := r.resolveRoute(binding.Route())
		if err != nil {
			return fmt.Errorf("bind producer %s: %w", binding.ID(), err)
		}
		capabilities := resolved.provider.Capabilities()
		if !capabilities.PublisherConfirm || !capabilities.MandatoryRoute {
			return fmt.Errorf("%w: producer %s requires confirm and mandatory routing", pkgmessaging.ErrProviderCapability, binding.ID())
		}
		contract, exists := contracts[binding.Contract()]
		if !exists {
			return fmt.Errorf("producer %s references unknown contract", binding.ID())
		}
		resolved.route.Contract = contract.Ref()
		resolved.route.ContentType = contract.ContentType()
		resolved.route.MaxPayloadBytes = contract.MaxPayloadBytes()
		r.producers[binding.ID()] = producerRuntime{binding: binding, contract: contract, route: resolved.route, provider: resolved.provider}
		usedRoutes[binding.Route()] = struct{}{}
	}
	byProvider := make(map[string][]Consumer)
	for _, binding := range catalog.Consumers() {
		resolved, err := r.resolveRoute(binding.Route())
		if err != nil {
			return fmt.Errorf("bind consumer %s: %w", binding.ID(), err)
		}
		configured := r.config.Routes[string(binding.Route())]
		if configured.Queue == "" {
			return fmt.Errorf("consumer %s route %s has no queue", binding.ID(), binding.Route())
		}
		if configured.Importance != binding.Importance() {
			return fmt.Errorf("consumer %s importance conflicts with route %s", binding.ID(), binding.Route())
		}
		// RabbitMQ delivery-limit 统计重投次数，而公共策略统计包含首次投递在内的总次数。
		if configured.Reliable && configured.DeliveryLimit+1 != binding.Delivery().MaxDeliveries() {
			return fmt.Errorf("consumer %s delivery limit conflicts with route %s", binding.ID(), binding.Route())
		}
		capabilities := resolved.provider.Capabilities()
		if !capabilities.ManualAck || !capabilities.DeadLetter || (configured.Reliable && !capabilities.DelayedRetry) {
			return fmt.Errorf("%w: consumer %s reliable delivery requirements are unsupported", pkgmessaging.ErrProviderCapability, binding.ID())
		}
		contract, exists := contracts[binding.Contract()]
		if !exists {
			return fmt.Errorf("consumer %s references unknown contract", binding.ID())
		}
		resolved.route.Contract = contract.Ref()
		resolved.route.ContentType = contract.ContentType()
		resolved.route.MaxPayloadBytes = contract.MaxPayloadBytes()
		consumer := &consumerRuntime{owner: r, binding: binding, route: resolved.route, provider: resolved.provider}
		consumer.lastError.Store("")
		r.consumers = append(r.consumers, consumer)
		byProvider[configured.Provider] = append(byProvider[configured.Provider], Consumer{
			Binding: binding, Route: resolved.route, Handle: consumer.handle,
		})
		usedRoutes[binding.Route()] = struct{}{}
	}
	for id := range r.config.Routes {
		if _, exists := usedRoutes[pkgmessaging.RouteID(id)]; !exists {
			return fmt.Errorf("messaging configured route %q has no module binding", id)
		}
	}
	for name, provider := range r.providers {
		if err := provider.Bind(byProvider[name]); err != nil {
			return fmt.Errorf("bind messaging provider %q consumers: %w", name, err)
		}
	}
	r.frozen = true
	return nil
}

type resolvedRoute struct {
	route    Route
	provider Provider
}

func (r *resource) resolveRoute(id pkgmessaging.RouteID) (resolvedRoute, error) {
	configured, exists := r.config.Route(id)
	if !exists {
		return resolvedRoute{}, fmt.Errorf("%w: %s", pkgmessaging.ErrUnknownRoute, id)
	}
	provider := r.providers[configured.Provider]
	if provider == nil {
		return resolvedRoute{}, fmt.Errorf("%w: provider %s", pkgmessaging.ErrUnavailable, configured.Provider)
	}
	return resolvedRoute{provider: provider, route: Route{
		ID: id, Exchange: configured.Exchange, ExchangeType: configured.ExchangeType,
		RoutingKey: configured.RoutingKey, Queue: configured.Queue,
		QueueType: configured.QueueType, Reliable: configured.Reliable, DeliveryLimit: configured.DeliveryLimit,
		DelayedRetryMin: configured.DelayedRetryMin, DelayedRetryMax: configured.DelayedRetryMax,
		DeadLetterExchange: configured.DeadLetterExchange, DeadLetterRoutingKey: configured.DeadLetterRoutingKey,
		AtLeastOnceDeadLetter: configured.AtLeastOnceDLX,
	}}, nil
}

func (r *resource) activate(ctx context.Context) error {
	if ctx == nil {
		return app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	if !r.frozen {
		r.mu.Unlock()
		return fmt.Errorf("messaging catalog is not frozen")
	}
	if r.retired {
		r.mu.Unlock()
		return pkgmessaging.ErrRetired
	}
	if !r.active {
		r.mu.Unlock()
		return fmt.Errorf("messaging publisher admission is not open")
	}
	if r.consuming {
		r.mu.Unlock()
		return nil
	}
	r.consuming = true
	monitorCtx, monitorCancel := context.WithCancel(r.ctx)
	r.admissionCancel = monitorCancel
	r.mu.Unlock()
	if len(r.consumers) == 0 {
		monitorCancel()
		return nil
	}
	ready := r.executionHealthy()
	if ready {
		if err := r.setConsumerAdmission(ctx, true); err != nil {
			_ = r.deactivate(context.WithoutCancel(ctx))
			return err
		}
	}
	r.wg.Add(1)
	go r.monitorExecution(monitorCtx, ready)
	return nil
}

func (r *resource) openPublisher() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.frozen {
		return fmt.Errorf("messaging catalog is not frozen")
	}
	if r.retired {
		return pkgmessaging.ErrRetired
	}
	r.active = true
	return nil
}

func (r *resource) deactivate(ctx context.Context) error {
	if ctx == nil {
		return app.ErrNilContext
	}
	handoffCtx, handoffCancel := context.WithTimeout(ctx, r.config.HandoffTimeout)
	defer handoffCancel()
	r.mu.Lock()
	if r.retired || !r.consuming {
		r.mu.Unlock()
		return nil
	}
	r.consuming = false
	if r.admissionCancel != nil {
		r.admissionCancel()
		r.admissionCancel = nil
	}
	r.mu.Unlock()
	return r.setConsumerAdmission(handoffCtx, false)
}

func (r *resource) setConsumerAdmission(ctx context.Context, enabled bool) error {
	r.admissionMu.Lock()
	defer r.admissionMu.Unlock()
	r.mu.RLock()
	desired, admitted, retired := r.consuming, r.admitted, r.retired
	r.mu.RUnlock()
	if enabled && (!desired || retired) {
		return nil
	}
	if admitted == enabled {
		return nil
	}
	names := r.config.SortedProviderNames()
	if !enabled {
		sort.Sort(sort.Reverse(sort.StringSlice(names)))
	}
	var joined error
	for _, name := range names {
		if enabled {
			if err := r.providers[name].Activate(ctx); err != nil {
				joined = errors.Join(joined, fmt.Errorf("activate messaging provider %q: %w", name, err))
				break
			}
			continue
		}
		joined = errors.Join(joined, r.providers[name].Deactivate(ctx))
	}
	if joined != nil && enabled {
		for _, name := range names {
			_ = r.providers[name].Deactivate(context.WithoutCancel(ctx))
		}
		return joined
	}
	r.mu.Lock()
	r.admitted = enabled
	for _, consumer := range r.consumers {
		consumer.active.Store(enabled)
	}
	r.mu.Unlock()
	return joined
}

func (r *resource) monitorExecution(ctx context.Context, initial bool) {
	defer r.wg.Done()
	ready := initial
	ticker := time.NewTicker(r.config.Recovery.InitialBackoff)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current := r.executionHealthy()
			if current == ready {
				continue
			}
			if err := r.setConsumerAdmission(ctx, current); err != nil && ctx.Err() == nil {
				r.dependencies.Logger.Warn("messaging execution admission transition failed",
					pkglogger.String("owner", "messaging-consumer"), pkglogger.String("phase", "execution-health"),
					pkglogger.Any("generation", r.dependencies.Generation),
					pkglogger.Bool("desired", current),
					pkglogger.String("error_type", messageErrorType(err)),
				)
				continue
			}
			ready = current
		}
	}
}

func (r *resource) executionHealthy() bool {
	result, err := r.dependencies.ExecutionHealth()
	ready := err == nil && result.Status != health.StatusFail
	r.executionReady.Store(ready)
	return ready
}

func (r *resource) publish(ctx context.Context, producerID pkgmessaging.ProducerID, message pkgmessaging.Message) (pkgmessaging.Receipt, error) {
	r.mu.RLock()
	active, retired := r.active, r.retired
	producer, exists := r.producers[producerID]
	r.mu.RUnlock()
	if retired {
		return pkgmessaging.Receipt{}, pkgmessaging.ErrRetired
	}
	if !active {
		return pkgmessaging.Receipt{}, pkgmessaging.ErrNotActive
	}
	if !exists {
		return pkgmessaging.Receipt{}, fmt.Errorf("%w: %s", pkgmessaging.ErrUnknownProducer, producerID)
	}
	if message.Contract() != producer.binding.Contract() {
		return pkgmessaging.Receipt{}, fmt.Errorf("%w: producer %s", pkgmessaging.ErrContractMismatch, producerID)
	}
	if len(message.Payload()) > producer.contract.MaxPayloadBytes() {
		return pkgmessaging.Receipt{}, fmt.Errorf("%w: producer %s payload exceeds contract limit", pkgmessaging.ErrInvalidMessage, producerID)
	}
	var providerResult PublishResult
	err := r.dependencies.Telemetry.Observe(ctx, pkgobservability.Work{Name: string(producerID), Kind: publishWorkKind}, func(observed context.Context) error {
		publishCtx, cancel := context.WithTimeout(observed, r.config.PublishConfirmTimeout)
		defer cancel()
		var err error
		providerResult, err = producer.provider.Publish(publishCtx, producer.route, message)
		return err
	})
	if err != nil {
		return pkgmessaging.Receipt{}, err
	}
	return pkgmessaging.Receipt{
		MessageID: message.ID(), ProducerID: producerID,
		ConfirmedAt: providerResult.ConfirmedAt, Reference: providerResult.Reference,
	}, nil
}

func (c *consumerRuntime) handle(ctx context.Context, incoming Incoming) Disposition {
	if !c.active.Load() {
		return DispositionDeferUncounted
	}
	c.inFlight.Add(1)
	defer c.inFlight.Add(-1)
	if incoming.Redelivered {
		c.redelivered.Add(1)
	}
	ctx = pkgobservability.WithTraceID(ctx, incoming.TraceID)
	ctx = pkgexecution.WithTrace(ctx, incoming.TraceID)
	result, err := c.owner.dependencies.Execution.Execute(ctx, pkgexecution.Execution{
		Key:     pkgexecution.Key("message:" + string(c.binding.ID()) + ":" + incoming.Message.Contract().String() + ":" + string(incoming.Message.ID())),
		Policy:  resilience.RetryPolicy{MaxAttempts: 1, Retryable: func(error) bool { return false }},
		Timeout: c.binding.Delivery().HandlerTimeout(), LeaseTTL: c.binding.Delivery().ProcessingLease(),
		RetentionTTL: c.binding.Delivery().IdempotencyRetention(), Trigger: "message." + string(c.binding.ID()),
		Operation: func(executionCtx context.Context) (result any, runErr error) {
			defer func() {
				if recover() != nil {
					runErr = consumerPanicError{}
				}
			}()
			runErr = c.owner.dependencies.Telemetry.Observe(executionCtx, pkgobservability.Work{
				Name: string(c.binding.ID()), Kind: consumeWorkKind,
			}, func(observed context.Context) error { return c.binding.Handle(observed, incoming.Message) })
			return nil, runErr
		},
	})
	if err == nil || result.Duplicate {
		c.acknowledged.Add(1)
		c.lastError.Store("")
		return DispositionAck
	}
	c.lastError.Store(messageErrorType(err))
	if errors.Is(err, pkgexecution.ErrBackend) || errors.Is(err, pkgexecution.ErrAlreadyRunning) || ctx.Err() != nil {
		c.logDisposition(DispositionDeferUncounted, incoming, err)
		return DispositionDeferUncounted
	}
	// Handler 自身超时必须消耗业务交付预算；只有上游 delivery context 取消才属于不计数的基础设施延后。
	if (fault.Retryable(err) || errors.Is(err, context.DeadlineExceeded)) &&
		incoming.DeliveryCount+1 < c.binding.Delivery().MaxDeliveries() {
		c.rejected.Add(1)
		c.logDisposition(DispositionRetryCounted, incoming, err)
		return DispositionRetryCounted
	}
	c.deadLettered.Add(1)
	c.logDisposition(DispositionDeadLetter, incoming, err)
	return DispositionDeadLetter
}

func (c *consumerRuntime) logDisposition(disposition Disposition, incoming Incoming, err error) {
	if c == nil || c.owner == nil || c.owner.dependencies.Logger == nil {
		return
	}
	fields := []pkglogger.Field{
		pkglogger.String("owner", "messaging-consumer"),
		pkglogger.String("phase", "delivery"),
		pkglogger.Any("generation", c.owner.dependencies.Generation),
		pkglogger.String("consumer", string(c.binding.ID())),
		pkglogger.String("route", string(c.binding.Route())),
		pkglogger.String("contract", incoming.Message.Contract().String()),
		pkglogger.String("message_id", string(incoming.Message.ID())),
		pkglogger.Any("delivery_count", incoming.DeliveryCount),
		pkglogger.String("disposition", disposition.String()),
		pkglogger.String("error_type", messageErrorType(err)),
	}
	if incoming.TraceID != "" {
		fields = append(fields, pkglogger.String("trace_id", incoming.TraceID))
	}
	switch disposition {
	case DispositionDeadLetter:
		c.owner.dependencies.Logger.Error("messaging consumer dead-lettered delivery", fields...)
	case DispositionRetryCounted:
		c.owner.dependencies.Logger.Warn("messaging consumer scheduled retry", fields...)
	case DispositionDeferUncounted:
		c.owner.dependencies.Logger.Warn("messaging consumer deferred delivery", fields...)
	}
}

func (r *resource) diagnostics() pkgmessaging.Diagnostics {
	diagnostics := pkgmessaging.Diagnostics{Enabled: r.config.Enabled}
	for _, name := range r.config.SortedProviderNames() {
		diagnostics.Providers = append(diagnostics.Providers, r.providers[name].Diagnostics())
	}
	consumers := append([]*consumerRuntime(nil), r.consumers...)
	sort.Slice(consumers, func(left, right int) bool { return consumers[left].binding.ID() < consumers[right].binding.ID() })
	for _, consumer := range consumers {
		lastError, _ := consumer.lastError.Load().(string)
		diagnostics.Consumers = append(diagnostics.Consumers, pkgmessaging.ConsumerDiagnostics{
			ID: consumer.binding.ID(), Route: consumer.binding.Route(), Active: consumer.active.Load(),
			InFlight: consumer.inFlight.Load(), Redelivered: consumer.redelivered.Load(),
			Acknowledged: consumer.acknowledged.Load(), Rejected: consumer.rejected.Load(),
			DeadLettered: consumer.deadLettered.Load(), LastErrorType: lastError,
		})
	}
	return diagnostics
}

func (r *resource) health() health.Result {
	result := health.Result{Name: string(ID), Kind: health.KindReadiness, Status: health.StatusPass, Message: "messaging ready"}
	if !r.config.Enabled {
		result.Message = "messaging disabled"
		return result
	}
	if !r.executionReady.Load() {
		optionalOnly := true
		for _, consumer := range r.consumers {
			if consumer.binding.Importance() == pkgmessaging.ImportanceRequired {
				optionalOnly = false
				break
			}
		}
		if optionalOnly {
			result.Status = health.StatusWarn
			result.Message = "optional messaging consumption paused by execution health"
			return result
		}
		result.Status = health.StatusFail
		result.Message = "required messaging consumption paused by execution health"
		return result
	}
	providerReady := make(map[string]bool, len(r.providers))
	for name, provider := range r.providers {
		providerReady[name] = provider.Diagnostics().Ready
	}
	optionalUnavailable := false
	for _, route := range r.config.Routes {
		if providerReady[route.Provider] {
			continue
		}
		if route.Importance == pkgmessaging.ImportanceRequired {
			result.Status = health.StatusFail
			result.Message = "required messaging route unavailable"
			return result
		}
		optionalUnavailable = true
	}
	if optionalUnavailable {
		result.Status = health.StatusWarn
		result.Message = "optional messaging route unavailable"
	}
	return result
}

func messageErrorType(err error) string {
	if err == nil {
		return ""
	}
	known := []struct {
		target error
		name   string
	}{
		{context.Canceled, "context_canceled"},
		{context.DeadlineExceeded, "context_deadline_exceeded"},
		{pkgexecution.ErrBackend, "execution_backend_unavailable"},
		{pkgexecution.ErrAlreadyRunning, "execution_already_running"},
		{pkgexecution.ErrRetryExhausted, "handler_failed"},
	}
	for _, item := range known {
		if errors.Is(err, item.target) {
			return item.name
		}
	}
	var panicErr consumerPanicError
	if errors.As(err, &panicErr) {
		return "consumer_panic"
	}
	return reflect.TypeOf(err).String()
}

type consumerPanicError struct{}

func (consumerPanicError) Error() string { return "message consumer panicked" }

var _ pkgmessaging.Publisher = (*publisher)(nil)
var _ Control = (*control)(nil)
