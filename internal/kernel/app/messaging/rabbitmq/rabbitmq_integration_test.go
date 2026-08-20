//go:build integration

package rabbitmq

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	messagingapp "github.com/rin721/go-scaffold-template/internal/kernel/app/messaging"
	pkgclock "github.com/rin721/go-scaffold-template/pkg/clock"
	"github.com/rin721/go-scaffold-template/pkg/logger"
	pkgmessaging "github.com/rin721/go-scaffold-template/pkg/messaging"
)

const (
	integrationExchange = "gst.messaging.events"
	integrationQueue    = "gst.messaging.events.q"
	integrationDLX      = "gst.messaging.dlx"
	integrationDLQ      = "gst.messaging.events.dlq"
)

func TestRabbitMQProviderIntegration(t *testing.T) {
	uri := os.Getenv("RABBITMQ_MESSAGING_URI")
	if uri == "" {
		t.Skip("RABBITMQ_MESSAGING_URI 未设置")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	adminConnection, err := amqp.Dial(uri)
	if err != nil {
		t.Fatalf("连接 RabbitMQ: %v", err)
	}
	defer adminConnection.Close()
	admin, err := adminConnection.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	prepareIntegrationTopology(t, admin)

	contract := testContract(t)
	route := messagingapp.Route{
		ID: "orders.events", Exchange: integrationExchange, ExchangeType: "topic", RoutingKey: "event",
		Queue: integrationQueue, QueueType: "quorum", Reliable: true, DeliveryLimit: 2,
		DelayedRetryMin: 100 * time.Millisecond, DelayedRetryMax: 200 * time.Millisecond,
		DeadLetterExchange: integrationDLX, DeadLetterRoutingKey: "dead", AtLeastOnceDeadLetter: true,
		Contract: contract.Ref(), ContentType: contract.ContentType(), MaxPayloadBytes: contract.MaxPayloadBytes(),
	}
	deliveryPolicy, err := pkgmessaging.NewDeliveryPolicy(3, time.Second, 2*time.Second, time.Hour, pkgmessaging.DeadLetterRequired)
	if err != nil {
		t.Fatal(err)
	}
	concurrency, err := pkgmessaging.NewConcurrencyPolicy(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	attempts := make(map[string]int)
	deliveries := make(chan messagingapp.Incoming, 16)
	binding, err := pkgmessaging.BindConsumer(pkgmessaging.ConsumerSpec{
		ID: "orders.reader", Contract: contract.Ref(), Route: route.ID, Delivery: deliveryPolicy,
		Concurrency: concurrency, Importance: pkgmessaging.ImportanceRequired,
	}, func(context.Context, pkgmessaging.Message) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	consumer := messagingapp.Consumer{Binding: binding, Route: route, Handle: func(_ context.Context, incoming messagingapp.Incoming) messagingapp.Disposition {
		deliveries <- incoming
		kind := string(incoming.Message.Payload())
		mu.Lock()
		attempts[kind]++
		attempt := attempts[kind]
		mu.Unlock()
		switch kind {
		case "retry":
			if attempt == 1 {
				return messagingapp.DispositionRetryCounted
			}
			return messagingapp.DispositionAck
		case "defer":
			if attempt == 1 {
				return messagingapp.DispositionDeferUncounted
			}
			return messagingapp.DispositionAck
		case "poison":
			return messagingapp.DispositionRetryCounted
		default:
			return messagingapp.DispositionAck
		}
	}}
	built, err := Factory().Build(ctx, "integration", messagingapp.ProviderConfig{
		Driver:   messagingapp.DriverRabbitMQ,
		RabbitMQ: messagingapp.RabbitMQConfig{URI: uri, Heartbeat: time.Second},
	}, messagingapp.ProviderDependencies{
		Logger: logger.Noop(), Clock: pkgclock.System(),
		Recovery: messagingapp.RecoveryConfig{ConnectTimeout: 2 * time.Second, InitialBackoff: 50 * time.Millisecond, MaxBackoff: time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	current := built.(*provider)
	defer current.Close(context.Background())
	if err := current.Bind([]messagingapp.Consumer{consumer}); err != nil {
		t.Fatal(err)
	}
	waitProvider(t, current, func(d pkgmessaging.ProviderDiagnostics) bool { return d.Ready })
	if err := current.Activate(ctx); err != nil {
		t.Fatal(err)
	}
	waitConsuming(t, current)

	publishIntegration(t, ctx, current, route, contract, "retry-1", "retry")
	first := waitIncoming(t, deliveries, "retry")
	second := waitIncoming(t, deliveries, "retry")
	if first.DeliveryCount != 0 || second.DeliveryCount != 1 {
		t.Fatalf("counted reject delivery counts=%d,%d", first.DeliveryCount, second.DeliveryCount)
	}

	publishIntegration(t, ctx, current, route, contract, "defer-1", "defer")
	first = waitIncoming(t, deliveries, "defer")
	second = waitIncoming(t, deliveries, "defer")
	if first.DeliveryCount != 0 || second.DeliveryCount != 0 {
		t.Fatalf("uncounted nack delivery counts=%d,%d", first.DeliveryCount, second.DeliveryCount)
	}

	unroutable := route
	unroutable.RoutingKey = "missing"
	message := integrationMessage(t, contract, "unroutable-1", "unroutable")
	if _, err := current.Publish(ctx, unroutable, message); !errors.Is(err, pkgmessaging.ErrUnroutable) {
		t.Fatalf("unroutable publish error=%v", err)
	}

	publishIntegration(t, ctx, current, route, contract, "poison-1", "poison")
	waitDeadLetter(t, ctx, admin, "poison-1")

	current.mu.RLock()
	connection := current.current.connection
	current.mu.RUnlock()
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	waitProvider(t, current, func(d pkgmessaging.ProviderDiagnostics) bool { return d.Recoveries > 0 && d.Ready })
	waitConsuming(t, current)
	publishIntegration(t, ctx, current, route, contract, "recovered-1", "recovered")
	waitIncoming(t, deliveries, "recovered")

	if err := current.Deactivate(ctx); err != nil {
		t.Fatal(err)
	}
	publishIntegration(t, ctx, current, route, contract, "drain-publish-1", "drain")
}

func prepareIntegrationTopology(t *testing.T, channel *amqp.Channel) {
	t.Helper()
	for _, queue := range []string{integrationQueue, integrationDLQ} {
		_, _ = channel.QueueDelete(queue, false, false, false)
	}
	for _, exchange := range []string{integrationExchange, integrationDLX} {
		_ = channel.ExchangeDelete(exchange, false, false)
	}
	if err := channel.ExchangeDeclare(integrationExchange, "topic", true, false, false, false, nil); err != nil {
		t.Fatal(err)
	}
	if err := channel.ExchangeDeclare(integrationDLX, "direct", true, false, false, false, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.QueueDeclare(integrationQueue, true, false, false, false, amqp.Table{"x-queue-type": "quorum"}); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.QueueDeclare(integrationDLQ, true, false, false, false, amqp.Table{"x-queue-type": "quorum"}); err != nil {
		t.Fatal(err)
	}
	if err := channel.QueueBind(integrationQueue, "event", integrationExchange, false, nil); err != nil {
		t.Fatal(err)
	}
	if err := channel.QueueBind(integrationDLQ, "dead", integrationDLX, false, nil); err != nil {
		t.Fatal(err)
	}
}

func publishIntegration(t *testing.T, ctx context.Context, provider *provider, route messagingapp.Route, contract pkgmessaging.Contract, id, body string) {
	t.Helper()
	if _, err := provider.Publish(ctx, route, integrationMessage(t, contract, id, body)); err != nil {
		t.Fatalf("publish %s: %v", body, err)
	}
}

func integrationMessage(t *testing.T, contract pkgmessaging.Contract, id, body string) pkgmessaging.Message {
	t.Helper()
	message, err := pkgmessaging.NewMessage(pkgmessaging.MessageSpec{
		ID: pkgmessaging.MessageID(id), Contract: contract.Ref(), OccurredAt: time.Now(), Payload: []byte(body),
	})
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func waitIncoming(t *testing.T, incoming <-chan messagingapp.Incoming, body string) messagingapp.Incoming {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case delivery := <-incoming:
			if string(delivery.Message.Payload()) == body {
				return delivery
			}
		case <-deadline:
			t.Fatalf("等待 %s delivery 超时", body)
		}
	}
}

func waitDeadLetter(t *testing.T, ctx context.Context, channel *amqp.Channel, messageID string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		delivery, ok, err := channel.Get(integrationDLQ, false)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			_ = delivery.Ack(false)
			if delivery.MessageId == messageID {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.Fatal("等待 poison message 进入 DLQ 超时")
}

func waitProvider(t *testing.T, provider *provider, condition func(pkgmessaging.ProviderDiagnostics) bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if condition(provider.Diagnostics()) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等待 Provider 状态超时: %+v", provider.Diagnostics())
}

func waitConsuming(t *testing.T, provider *provider) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		provider.mu.RLock()
		current := provider.current
		provider.mu.RUnlock()
		if current != nil && current.isConsuming() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("等待 RabbitMQ Consumer 激活超时")
}
