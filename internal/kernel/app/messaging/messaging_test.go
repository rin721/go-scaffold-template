package messaging

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	kernelconfig "github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgclock "github.com/rin721/go-scaffold-template/pkg/clock"
	pkgexecution "github.com/rin721/go-scaffold-template/pkg/execution"
	"github.com/rin721/go-scaffold-template/pkg/fault"
	"github.com/rin721/go-scaffold-template/pkg/health"
	"github.com/rin721/go-scaffold-template/pkg/logger"
	pkgmessaging "github.com/rin721/go-scaffold-template/pkg/messaging"
	pkgobservability "github.com/rin721/go-scaffold-template/pkg/observability"
)

func TestResourceFreezesCatalogAndGatesPublisherByActivation(t *testing.T) {
	factory := &testFactory{}
	current := buildTestResource(t, factory, pkgexecution.NewExecutor(pkgexecution.NewMemoryStore()))
	catalog, message := testCatalog(t, func(context.Context, pkgmessaging.Message) error { return nil })
	if err := current.freeze(catalog); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if _, err := current.publish(context.Background(), "orders.writer", message); !errors.Is(err, pkgmessaging.ErrNotActive) {
		t.Fatalf("候选发布错误=%v want ErrNotActive", err)
	}
	if err := current.openPublisher(); err != nil {
		t.Fatalf("open publisher: %v", err)
	}
	if err := current.activate(context.Background()); err != nil {
		t.Fatalf("activate: %v", err)
	}
	receipt, err := current.publish(context.Background(), "orders.writer", message)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if receipt.MessageID != message.ID() || factory.provider.published.Load() != 1 {
		t.Fatalf("receipt=%+v published=%d", receipt, factory.provider.published.Load())
	}
	if err := current.deactivate(context.Background()); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := current.publish(context.Background(), "orders.writer", message); err != nil {
		t.Fatalf("Consumer quiesce 后旧代 Publisher 应继续服务 HTTP 排空: %v", err)
	}
}

func TestConsumerUsesExecutionForIdempotencyAndBrokerRetryBudget(t *testing.T) {
	factory := &testFactory{}
	var runs atomic.Int32
	current := buildTestResource(t, factory, pkgexecution.NewExecutor(pkgexecution.NewMemoryStore()))
	catalog, message := testCatalog(t, func(context.Context, pkgmessaging.Message) error {
		runs.Add(1)
		return nil
	})
	if err := current.freeze(catalog); err != nil {
		t.Fatal(err)
	}
	if err := current.openPublisher(); err != nil {
		t.Fatal(err)
	}
	if err := current.activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	consumer := factory.provider.consumer(t)
	if disposition := consumer.Handle(context.Background(), Incoming{Message: message}); disposition != DispositionAck {
		t.Fatalf("首次 disposition=%v want ack", disposition)
	}
	if disposition := consumer.Handle(context.Background(), Incoming{Message: message, Redelivered: true}); disposition != DispositionAck {
		t.Fatalf("重复 disposition=%v want ack", disposition)
	}
	if runs.Load() != 1 {
		t.Fatalf("业务 Handler 运行次数=%d want 1", runs.Load())
	}

	retryFactory := &testFactory{}
	retryCurrent := buildTestResource(t, retryFactory, pkgexecution.NewExecutor(pkgexecution.NewMemoryStore()))
	retryCatalog, retryMessage := testCatalog(t, func(context.Context, pkgmessaging.Message) error {
		return fault.Wrap(errors.New("dependency unavailable"), fault.CodeUnavailable, "orders.consume", true)
	})
	if err := retryCurrent.freeze(retryCatalog); err != nil {
		t.Fatal(err)
	}
	if err := retryCurrent.openPublisher(); err != nil {
		t.Fatal(err)
	}
	if err := retryCurrent.activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	retryConsumer := retryFactory.provider.consumer(t)
	if disposition := retryConsumer.Handle(context.Background(), Incoming{Message: retryMessage}); disposition != DispositionRetryCounted {
		t.Fatalf("首次失败 disposition=%v want counted retry", disposition)
	}
	if disposition := retryConsumer.Handle(context.Background(), Incoming{Message: retryMessage, DeliveryCount: 2, Redelivered: true}); disposition != DispositionDeadLetter {
		t.Fatalf("耗尽 disposition=%v want dead letter", disposition)
	}
}

func TestConsumerIdempotencyIsScopedByConsumerAndContract(t *testing.T) {
	store := pkgexecution.NewMemoryStore()
	executor := pkgexecution.NewExecutor(store)
	var runs atomic.Int32
	firstFactory := &testFactory{}
	first := buildTestResource(t, firstFactory, executor)
	firstCatalog, message := testCatalogForConsumer(t, "orders.reader", func(context.Context, pkgmessaging.Message) error {
		runs.Add(1)
		return nil
	})
	secondFactory := &testFactory{}
	second := buildTestResource(t, secondFactory, executor)
	secondCatalog, _ := testCatalogForConsumer(t, "orders.audit", func(context.Context, pkgmessaging.Message) error {
		runs.Add(1)
		return nil
	})
	for _, item := range []struct {
		resource *resource
		catalog  pkgmessaging.Catalog
	}{{first, firstCatalog}, {second, secondCatalog}} {
		if err := item.resource.freeze(item.catalog); err != nil {
			t.Fatal(err)
		}
		if err := item.resource.openPublisher(); err != nil {
			t.Fatal(err)
		}
		if err := item.resource.activate(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if disposition := firstFactory.provider.consumer(t).Handle(context.Background(), Incoming{Message: message}); disposition != DispositionAck {
		t.Fatalf("first disposition=%v want ack", disposition)
	}
	if disposition := secondFactory.provider.consumer(t).Handle(context.Background(), Incoming{Message: message}); disposition != DispositionAck {
		t.Fatalf("second disposition=%v want ack", disposition)
	}
	if runs.Load() != 2 {
		t.Fatalf("不同 Consumer 业务 Handler 运行次数=%d want 2", runs.Load())
	}
}

func TestConsumerDefersWhenExecutionBackendIsUnavailable(t *testing.T) {
	factory := &testFactory{}
	current := buildTestResource(t, factory, unavailableExecutor{})
	catalog, message := testCatalog(t, func(context.Context, pkgmessaging.Message) error { return nil })
	if err := current.freeze(catalog); err != nil {
		t.Fatal(err)
	}
	if err := current.openPublisher(); err != nil {
		t.Fatal(err)
	}
	if err := current.activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if disposition := factory.provider.consumer(t).Handle(context.Background(), Incoming{Message: message}); disposition != DispositionDeferUncounted {
		t.Fatalf("disposition=%v want uncounted defer", disposition)
	}
}

func TestConsumerDefersActiveLeaseAndDeadLettersPermanentOrPanickingHandler(t *testing.T) {
	store := pkgexecution.NewMemoryStore()
	factory := &testFactory{}
	current := buildTestResource(t, factory, pkgexecution.NewExecutor(store))
	catalog, message := testCatalog(t, func(context.Context, pkgmessaging.Message) error { return nil })
	if err := current.freeze(catalog); err != nil {
		t.Fatal(err)
	}
	if err := current.openPublisher(); err != nil {
		t.Fatal(err)
	}
	if err := current.activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	key := pkgexecution.Key("message:orders.reader:" + message.Contract().String() + ":" + string(message.ID()))
	claimed, err := store.Claim(context.Background(), key, time.Minute, time.Now())
	if err != nil || !claimed {
		t.Fatalf("pre-claim active lease: claimed=%v error=%v", claimed, err)
	}
	if disposition := factory.provider.consumer(t).Handle(context.Background(), Incoming{Message: message}); disposition != DispositionDeferUncounted {
		t.Fatalf("active lease disposition=%v want uncounted defer", disposition)
	}

	for _, test := range []struct {
		name    string
		handler pkgmessaging.Handler
	}{
		{name: "permanent", handler: func(context.Context, pkgmessaging.Message) error { return errors.New("invalid payload") }},
		{name: "panic", handler: func(context.Context, pkgmessaging.Message) error { panic("boom") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			terminalFactory := &testFactory{}
			terminal := buildTestResource(t, terminalFactory, pkgexecution.NewExecutor(pkgexecution.NewMemoryStore()))
			terminalCatalog, terminalMessage := testCatalog(t, test.handler)
			if err := terminal.freeze(terminalCatalog); err != nil {
				t.Fatal(err)
			}
			if err := terminal.openPublisher(); err != nil {
				t.Fatal(err)
			}
			if err := terminal.activate(context.Background()); err != nil {
				t.Fatal(err)
			}
			if disposition := terminalFactory.provider.consumer(t).Handle(context.Background(), Incoming{Message: terminalMessage}); disposition != DispositionDeadLetter {
				t.Fatalf("disposition=%v want dead letter", disposition)
			}
		})
	}
}

func TestPublisherRejectsUnknownProducerAndContractMismatchAndPreservesProviderError(t *testing.T) {
	factory := &testFactory{}
	current := buildTestResource(t, factory, pkgexecution.NewExecutor(pkgexecution.NewMemoryStore()))
	catalog, message := testCatalog(t, func(context.Context, pkgmessaging.Message) error { return nil })
	if err := current.freeze(catalog); err != nil {
		t.Fatal(err)
	}
	if err := current.openPublisher(); err != nil {
		t.Fatal(err)
	}
	if _, err := current.publish(context.Background(), "unknown.writer", message); !errors.Is(err, pkgmessaging.ErrUnknownProducer) {
		t.Fatalf("unknown producer error=%v", err)
	}
	other, err := pkgmessaging.DefineContract(pkgmessaging.ContractSpec{
		ID: "orders.updated", Version: 1, ContentType: "application/json", MaxPayloadBytes: 1024, Fingerprint: "sha256:orders-updated-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	mismatch, err := pkgmessaging.NewMessage(pkgmessaging.MessageSpec{
		ID: "mismatch-1", Contract: other.Ref(), OccurredAt: time.Now(), Payload: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := current.publish(context.Background(), "orders.writer", mismatch); !errors.Is(err, pkgmessaging.ErrContractMismatch) {
		t.Fatalf("contract mismatch error=%v", err)
	}
	factory.provider.publishErr = pkgmessaging.ErrUnavailable
	if _, err := current.publish(context.Background(), "orders.writer", message); !errors.Is(err, pkgmessaging.ErrUnavailable) {
		t.Fatalf("provider error=%v want ErrUnavailable", err)
	}
}

func TestConsumerCountsHandlerTimeoutButDefersCanceledDelivery(t *testing.T) {
	timeoutFactory := &testFactory{}
	timeoutCurrent := buildTestResource(t, timeoutFactory, pkgexecution.NewExecutor(pkgexecution.NewMemoryStore()))
	timeoutCatalog, timeoutMessage := testCatalog(t, func(ctx context.Context, _ pkgmessaging.Message) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err := timeoutCurrent.freeze(timeoutCatalog); err != nil {
		t.Fatal(err)
	}
	if err := timeoutCurrent.openPublisher(); err != nil {
		t.Fatal(err)
	}
	if err := timeoutCurrent.activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if disposition := timeoutFactory.provider.consumer(t).Handle(context.Background(), Incoming{Message: timeoutMessage}); disposition != DispositionRetryCounted {
		t.Fatalf("handler timeout disposition=%v want counted retry", disposition)
	}

	cancelFactory := &testFactory{}
	cancelCurrent := buildTestResource(t, cancelFactory, pkgexecution.NewExecutor(pkgexecution.NewMemoryStore()))
	cancelCatalog, cancelMessage := testCatalog(t, func(ctx context.Context, _ pkgmessaging.Message) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err := cancelCurrent.freeze(cancelCatalog); err != nil {
		t.Fatal(err)
	}
	if err := cancelCurrent.openPublisher(); err != nil {
		t.Fatal(err)
	}
	if err := cancelCurrent.activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	deliveryCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if disposition := cancelFactory.provider.consumer(t).Handle(deliveryCtx, Incoming{Message: cancelMessage}); disposition != DispositionDeferUncounted {
		t.Fatalf("canceled delivery disposition=%v want uncounted defer", disposition)
	}
}

func TestExecutionHealthPausesAndRestoresConsumerAdmission(t *testing.T) {
	factory := &testFactory{}
	var executionReady atomic.Bool
	executionReady.Store(true)
	config := testConfig()
	config.Recovery.InitialBackoff = 10 * time.Millisecond
	current, err := build(context.Background(), config, Dependencies{
		Generation: 1, Logger: logger.Noop(), Clock: pkgclock.System(),
		Execution: pkgexecution.NewExecutor(pkgexecution.NewMemoryStore()),
		ExecutionHealth: func() (health.Result, error) {
			if executionReady.Load() {
				return health.Result{Status: health.StatusPass}, nil
			}
			return health.Result{Status: health.StatusFail}, nil
		},
		Telemetry: passthroughTelemetry{}, Factories: []Factory{factory},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop(context.Background(), current) })
	catalog, _ := testCatalog(t, func(context.Context, pkgmessaging.Message) error { return nil })
	if err := current.freeze(catalog); err != nil {
		t.Fatal(err)
	}
	if err := current.openPublisher(); err != nil {
		t.Fatal(err)
	}
	if err := current.activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !factory.provider.active.Load() {
		t.Fatal("健康 Execution 应开放 Consumer admission")
	}
	executionReady.Store(false)
	waitBool(t, time.Second, func() bool { return !factory.provider.active.Load() })
	if got := current.health().Status; got != health.StatusFail {
		t.Fatalf("Execution 故障 health=%q want fail", got)
	}
	executionReady.Store(true)
	waitBool(t, time.Second, factory.provider.active.Load)
}

func TestProducerOnlyCatalogDoesNotDependOnExecutionHealth(t *testing.T) {
	factory := &testFactory{}
	current, err := build(context.Background(), testConfig(), Dependencies{
		Generation: 1, Logger: logger.Noop(), Clock: pkgclock.System(),
		Execution: pkgexecution.NewExecutor(pkgexecution.NewMemoryStore()),
		ExecutionHealth: func() (health.Result, error) {
			return health.Result{Status: health.StatusFail}, nil
		},
		Telemetry: passthroughTelemetry{}, Factories: []Factory{factory},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop(context.Background(), current) })
	full, _ := testCatalog(t, func(context.Context, pkgmessaging.Message) error { return nil })
	producerOnly, err := pkgmessaging.BuildCatalog(pkgmessaging.Contribute(full.Contracts(), full.Producers(), nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := current.freeze(producerOnly); err != nil {
		t.Fatal(err)
	}
	if err := current.openPublisher(); err != nil {
		t.Fatal(err)
	}
	if err := current.activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := current.health().Status; got != health.StatusPass {
		t.Fatalf("producer-only health=%q want pass", got)
	}
}

func TestHealthDistinguishesRequiredAndOptionalRoutes(t *testing.T) {
	factory := &testFactory{}
	current := buildTestResource(t, factory, pkgexecution.NewExecutor(pkgexecution.NewMemoryStore()))
	catalog, _ := testCatalog(t, func(context.Context, pkgmessaging.Message) error { return nil })
	if err := current.freeze(catalog); err != nil {
		t.Fatal(err)
	}
	if err := current.openPublisher(); err != nil {
		t.Fatal(err)
	}
	if err := current.activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	factory.provider.ready.Store(false)
	if got := current.health().Status; got != "fail" {
		t.Fatalf("required route health=%q want fail", got)
	}
	route := current.config.Routes["orders.events"]
	route.Importance = pkgmessaging.ImportanceOptional
	current.config.Routes["orders.events"] = route
	if got := current.health().Status; got != "warn" {
		t.Fatalf("optional route health=%q want warn", got)
	}
}

func TestNormalizeConfigRejectsUnsafeReliableRoute(t *testing.T) {
	config := testConfig()
	route := config.Routes["orders.events"]
	route.DeliveryLimit = 0
	config.Routes["orders.events"] = route
	if _, err := normalizeConfig(config); err == nil {
		t.Fatal("normalizeConfig() error = nil")
	}
	disabled := defaultConfig()
	disabled.Providers["stale"] = ProviderConfig{Driver: DriverFake}
	if _, err := normalizeConfig(disabled); err == nil {
		t.Fatal("disabled messaging retained provider configuration")
	}
}

func TestDecodeAcceptsNamedProviderAndRouteMaps(t *testing.T) {
	snapshot, err := kernelconfig.New(kernelconfig.MapSource("test", map[string]any{
		"messaging": map[string]any{
			"enabled": true, "publishConfirmTimeout": "5s", "handoffTimeout": "30s", "shutdownTimeout": "30s",
			"recovery":  map[string]any{"connectTimeout": "3s", "initialBackoff": "500ms", "maxBackoff": "30s"},
			"providers": map[string]any{"primary": map[string]any{"driver": "fake"}},
			"routes": map[string]any{"orders.events": map[string]any{
				"provider": "primary", "routingKey": "orders.created", "queue": "orders.created.v1",
				"queueType": "quorum", "importance": "required", "reliable": true,
				"deliveryLimit": 3, "delayedRetryMin": "1s", "delayedRetryMax": "1m",
				"deadLetterExchange": "orders.dlx", "deadLetterRoutingKey": "dead", "atLeastOnceDeadLetter": true,
			}},
		},
	})).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decode(snapshot)
	if err != nil {
		t.Fatalf("decode() error = %v", err)
	}
	if decoded.Providers["primary"].Driver != DriverFake || decoded.Routes["orders.events"].Provider != "primary" {
		t.Fatalf("decoded config = %+v", decoded)
	}
}

func buildTestResource(t *testing.T, factory Factory, executor pkgexecution.OperationExecutor) *resource {
	t.Helper()
	current, err := build(context.Background(), testConfig(), Dependencies{
		Generation: 1, Logger: logger.Noop(), Clock: pkgclock.Fixed(time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC)),
		Execution: executor, ExecutionHealth: func() (health.Result, error) {
			return health.Result{Status: health.StatusPass}, nil
		}, Telemetry: passthroughTelemetry{}, Factories: []Factory{factory},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Cleanup(func() { _ = stop(context.Background(), current) })
	return current
}

func testConfig() Config {
	return Config{
		Enabled: true, PublishConfirmTimeout: time.Second, HandoffTimeout: time.Second, ShutdownTimeout: time.Second,
		Recovery:  RecoveryConfig{ConnectTimeout: time.Second, InitialBackoff: time.Millisecond, MaxBackoff: time.Second},
		Providers: map[string]ProviderConfig{"primary": {Driver: DriverFake}},
		Routes: map[string]RouteConfig{"orders.events": {
			Provider: "primary", RoutingKey: "orders.created", Queue: "orders.created.v1", QueueType: "quorum",
			Importance: pkgmessaging.ImportanceRequired, Reliable: true, DeliveryLimit: 2,
			DelayedRetryMin: time.Second, DelayedRetryMax: time.Minute,
			DeadLetterExchange: "orders.dlx", DeadLetterRoutingKey: "orders.created.dead", AtLeastOnceDLX: true,
		}},
	}
}

func testCatalog(t *testing.T, handler pkgmessaging.Handler) (pkgmessaging.Catalog, pkgmessaging.Message) {
	return testCatalogForConsumer(t, "orders.reader", handler)
}

func testCatalogForConsumer(t *testing.T, consumerID pkgmessaging.ConsumerID, handler pkgmessaging.Handler) (pkgmessaging.Catalog, pkgmessaging.Message) {
	t.Helper()
	contract, err := pkgmessaging.DefineContract(pkgmessaging.ContractSpec{
		ID: "orders.created", Version: 1, ContentType: "application/json", MaxPayloadBytes: 1024,
		Fingerprint: "sha256:orders-created-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	producer, err := pkgmessaging.BindProducer(pkgmessaging.ProducerSpec{
		ID: "orders.writer", Contract: contract.Ref(), Route: "orders.events", Confirm: pkgmessaging.ConfirmBroker,
	})
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := pkgmessaging.NewDeliveryPolicy(3, 100*time.Millisecond, time.Second, time.Hour, pkgmessaging.DeadLetterRequired)
	if err != nil {
		t.Fatal(err)
	}
	concurrency, err := pkgmessaging.NewConcurrencyPolicy(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := pkgmessaging.BindConsumer(pkgmessaging.ConsumerSpec{
		ID: consumerID, Contract: contract.Ref(), Route: "orders.events", Delivery: delivery,
		Concurrency: concurrency, Importance: pkgmessaging.ImportanceRequired,
	}, handler)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := pkgmessaging.BuildCatalog(pkgmessaging.Contribute(
		[]pkgmessaging.Contract{contract}, []pkgmessaging.ProducerBinding{producer}, []pkgmessaging.ConsumerBinding{consumer},
	))
	if err != nil {
		t.Fatal(err)
	}
	message, err := pkgmessaging.NewMessage(pkgmessaging.MessageSpec{
		ID: "019fef13-5508-7650-9ade-39b8173ba33a", Contract: contract.Ref(),
		OccurredAt: time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC), Payload: []byte(`{"orderId":"42"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	return catalog, message
}

type testFactory struct{ provider *testProvider }

func (*testFactory) Kind() Driver { return DriverFake }
func (f *testFactory) Build(context.Context, string, ProviderConfig, ProviderDependencies) (Provider, error) {
	f.provider = &testProvider{}
	f.provider.ready.Store(true)
	return f.provider, nil
}

type testProvider struct {
	mu         sync.Mutex
	consumers  []Consumer
	active     atomic.Bool
	ready      atomic.Bool
	published  atomic.Int32
	publishErr error
}

func (*testProvider) Capabilities() Capabilities {
	return Capabilities{PublisherConfirm: true, MandatoryRoute: true, ManualAck: true, DelayedRetry: true, DeadLetter: true}
}
func (p *testProvider) Bind(consumers []Consumer) error {
	p.consumers = append([]Consumer(nil), consumers...)
	return nil
}
func (p *testProvider) Activate(context.Context) error   { p.active.Store(true); return nil }
func (p *testProvider) Deactivate(context.Context) error { p.active.Store(false); return nil }
func (p *testProvider) Publish(context.Context, Route, pkgmessaging.Message) (PublishResult, error) {
	if p.publishErr != nil {
		return PublishResult{}, p.publishErr
	}
	if !p.ready.Load() {
		return PublishResult{}, pkgmessaging.ErrUnavailable
	}
	p.published.Add(1)
	return PublishResult{ConfirmedAt: time.Now(), Reference: "test"}, nil
}
func (p *testProvider) Diagnostics() pkgmessaging.ProviderDiagnostics {
	return pkgmessaging.ProviderDiagnostics{Name: "primary", Driver: "fake", State: pkgmessaging.ProviderReady, Ready: p.ready.Load()}
}
func (*testProvider) Close(context.Context) error { return nil }
func (p *testProvider) consumer(t *testing.T) Consumer {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.consumers) != 1 {
		t.Fatalf("consumers=%d want 1", len(p.consumers))
	}
	return p.consumers[0]
}

type unavailableExecutor struct{}

func (unavailableExecutor) Execute(context.Context, pkgexecution.Execution) (pkgexecution.Result, error) {
	return pkgexecution.Result{}, pkgexecution.WrapBackend(errors.New("offline"))
}

type passthroughTelemetry struct{}

func (passthroughTelemetry) HTTP([]pkgobservability.Operation) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}
func (passthroughTelemetry) Observe(ctx context.Context, _ pkgobservability.Work, work pkgobservability.WorkFunc) error {
	return work(ctx)
}
func (passthroughTelemetry) Diagnostics(context.Context) (pkgobservability.Diagnostics, error) {
	return pkgobservability.Diagnostics{Ready: true}, nil
}

var _ Factory = (*testFactory)(nil)
var _ Provider = (*testProvider)(nil)

func waitBool(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("等待状态转换超时")
}
