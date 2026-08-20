package rabbitmq

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	messagingapp "github.com/rin721/go-scaffold-template/internal/kernel/app/messaging"
	pkgclock "github.com/rin721/go-scaffold-template/pkg/clock"
	"github.com/rin721/go-scaffold-template/pkg/logger"
	pkgmessaging "github.com/rin721/go-scaffold-template/pkg/messaging"
)

func TestDecodeDeliveryValidatesEnvelopeAndMapsDeliveryCount(t *testing.T) {
	contract := testContract(t)
	route := testRoute(contract)
	delivery := amqp.Delivery{
		Headers: amqp.Table{
			headerContractID: "orders.created", headerContractVer: int64(1),
			headerDeliveryCount: int64(2), headerTraceID: "trace-42",
		},
		ContentType: "application/json", MessageId: "message-42", Timestamp: time.Now(),
		Type: contract.Ref().String(), Body: []byte(`{"orderId":"42"}`), Redelivered: true,
	}
	incoming, err := decodeDelivery(route, delivery)
	if err != nil {
		t.Fatalf("decodeDelivery() error = %v", err)
	}
	if incoming.DeliveryCount != 2 || incoming.TraceID != "trace-42" || !incoming.Redelivered {
		t.Fatalf("incoming = %+v", incoming)
	}
	delivery.ContentType = "text/plain"
	if _, err := decodeDelivery(route, delivery); !errors.Is(err, pkgmessaging.ErrContractMismatch) {
		t.Fatalf("contract mismatch error = %v", err)
	}
}

func TestHandleDeliveryMapsCountedAndUncountedDisposition(t *testing.T) {
	contract := testContract(t)
	route := testRoute(contract)
	tests := []struct {
		name        string
		disposition messagingapp.Disposition
		operation   string
		requeue     bool
	}{
		{name: "ack", disposition: messagingapp.DispositionAck, operation: "ack"},
		{name: "counted reject", disposition: messagingapp.DispositionRetryCounted, operation: "reject", requeue: true},
		{name: "uncounted nack", disposition: messagingapp.DispositionDeferUncounted, operation: "nack", requeue: true},
		{name: "dead letter", disposition: messagingapp.DispositionDeadLetter, operation: "reject"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			acknowledger := &recordingAcknowledger{}
			delivery := amqp.Delivery{
				Acknowledger: acknowledger, DeliveryTag: 7,
				Headers:     amqp.Table{headerContractID: "orders.created", headerContractVer: int64(1)},
				ContentType: "application/json", MessageId: "message-42", Timestamp: time.Now(),
				Type: contract.Ref().String(), Body: []byte(`{}`),
			}
			current := &provider{ctx: context.Background()}
			current.lastError.Store("")
			current.handleDelivery(context.Background(), messagingapp.Consumer{
				Route: route, Handle: func(context.Context, messagingapp.Incoming) messagingapp.Disposition { return test.disposition },
			}, delivery)
			if acknowledger.operation != test.operation || acknowledger.requeue != test.requeue {
				t.Fatalf("ack operation=%q requeue=%v", acknowledger.operation, acknowledger.requeue)
			}
		})
	}
}

func TestHandleDeliveryLogsRejectedDecodeBoundary(t *testing.T) {
	contract := testContract(t)
	acknowledger := &recordingAcknowledger{}
	logs := logger.NewTestLogger()
	delivery := amqp.Delivery{
		Acknowledger: acknowledger, DeliveryTag: 7,
		Headers:     amqp.Table{headerContractID: "orders.created", headerContractVer: int64(1)},
		ContentType: "text/plain", MessageId: "message-42", Timestamp: time.Now(),
		Type: contract.Ref().String(), Body: []byte(`{"orderId":"42"}`),
	}
	current := &provider{
		name: "primary", ctx: context.Background(),
		dependencies: messagingapp.ProviderDependencies{Generation: 9, Logger: logs, Clock: pkgclock.System()},
	}
	current.lastError.Store("")
	current.handleDelivery(context.Background(), messagingapp.Consumer{
		Route: testRoute(contract), Handle: func(context.Context, messagingapp.Incoming) messagingapp.Disposition {
			return messagingapp.DispositionAck
		},
	}, delivery)

	if acknowledger.operation != "reject" || acknowledger.requeue {
		t.Fatalf("ack operation=%q requeue=%v", acknowledger.operation, acknowledger.requeue)
	}
	entries := logs.Entries()
	if len(entries) != 1 {
		t.Fatalf("entries=%d want 1", len(entries))
	}
	if entries[0].Level != "warn" || entries[0].Message != "messaging delivery rejected" {
		t.Fatalf("entry=%#v", entries[0])
	}
}

func TestHandleDeliveryKeepsAcknowledgementAmbiguityInDiagnostics(t *testing.T) {
	contract := testContract(t)
	acknowledger := &recordingAcknowledger{err: errors.New("channel closed")}
	delivery := amqp.Delivery{
		Acknowledger: acknowledger, DeliveryTag: 7,
		Headers:     amqp.Table{headerContractID: "orders.created", headerContractVer: int64(1)},
		ContentType: "application/json", MessageId: "message-42", Timestamp: time.Now(),
		Type: contract.Ref().String(), Body: []byte(`{}`),
	}
	current := &provider{ctx: context.Background()}
	current.lastError.Store("")
	current.handleDelivery(context.Background(), messagingapp.Consumer{
		Route: testRoute(contract), Handle: func(context.Context, messagingapp.Incoming) messagingapp.Disposition {
			return messagingapp.DispositionAck
		},
	}, delivery)
	if lastError, _ := current.lastError.Load().(string); lastError == "" {
		t.Fatal("ack failure was not retained in provider diagnostics")
	}
}

func TestRecoveryDelayIsBoundedAndJittered(t *testing.T) {
	config := messagingapp.RecoveryConfig{InitialBackoff: 100 * time.Millisecond, MaxBackoff: time.Second}
	for attempt := 1; attempt < 20; attempt++ {
		delay := recoveryDelay(attempt, config)
		if delay <= 0 || delay > 1100*time.Millisecond {
			t.Fatalf("attempt %d delay=%s", attempt, delay)
		}
	}
}

func TestDeterministicBrokerErrorRejectsConfigurationAndTopologyFailures(t *testing.T) {
	for _, code := range []int{403, 404, 405, 406, 530} {
		if !deterministicBrokerError(&amqp.Error{Code: code}) {
			t.Fatalf("AMQP code %d should be deterministic", code)
		}
	}
	if deterministicBrokerError(&amqp.Error{Code: 320}) {
		t.Fatal("AMQP connection-forced should remain recoverable")
	}
	if deterministicBrokerError(errors.New("network unavailable")) {
		t.Fatal("network failure should remain recoverable")
	}
}

func TestInitialBrokerUnavailabilityEntersRecoveringWithoutFailingBind(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	built, err := Factory().Build(t.Context(), "offline", messagingapp.ProviderConfig{
		Driver:   messagingapp.DriverRabbitMQ,
		RabbitMQ: messagingapp.RabbitMQConfig{URI: "amqp://" + address + "/", Heartbeat: time.Second},
	}, messagingapp.ProviderDependencies{
		Logger: logger.Noop(), Clock: pkgclock.System(),
		Recovery: messagingapp.RecoveryConfig{ConnectTimeout: 20 * time.Millisecond, InitialBackoff: time.Second, MaxBackoff: time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	current := built.(*provider)
	if err := current.Bind(nil); err != nil {
		t.Fatalf("temporary broker unavailability should not fail Bind: %v", err)
	}
	if diagnostics := current.Diagnostics(); diagnostics.State != pkgmessaging.ProviderRecovering {
		t.Fatalf("provider state=%q want recovering", diagnostics.State)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := current.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestConsumerStopTimeoutRestoresAdmissionAfterHandlersDrain(t *testing.T) {
	providerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	current := &provider{ctx: providerCtx, cancel: cancel, activation: make(chan struct{}, 1)}
	current.active.Store(true)
	consumerCtx, consumerCancel := context.WithCancel(providerCtx)
	session := &session{ctx: consumerCtx, cancel: consumerCancel, consuming: true}
	session.wg.Add(1)

	deadline, deadlineCancel := context.WithCancel(context.Background())
	deadlineCancel()
	if err := current.stopConsumers(deadline, session, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("stopConsumers() error=%v want context.Canceled", err)
	}
	session.wg.Done()
	select {
	case <-current.activation:
	case <-time.After(time.Second):
		t.Fatal("handler 排空后未触发 Consumer admission 恢复")
	}
	current.cleanupWG.Wait()
	if session.isConsuming() {
		t.Fatal("handler 排空后 session 仍标记 consuming")
	}
}

type recordingAcknowledger struct {
	operation string
	requeue   bool
	err       error
}

func (r *recordingAcknowledger) Ack(uint64, bool) error {
	r.operation = "ack"
	return r.err
}
func (r *recordingAcknowledger) Nack(_ uint64, _ bool, requeue bool) error {
	r.operation, r.requeue = "nack", requeue
	return nil
}
func (r *recordingAcknowledger) Reject(_ uint64, requeue bool) error {
	r.operation, r.requeue = "reject", requeue
	return nil
}

func testContract(t *testing.T) pkgmessaging.Contract {
	t.Helper()
	contract, err := pkgmessaging.DefineContract(pkgmessaging.ContractSpec{
		ID: "orders.created", Version: 1, ContentType: "application/json",
		MaxPayloadBytes: 1024, Fingerprint: "sha256:orders-created-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

func testRoute(contract pkgmessaging.Contract) messagingapp.Route {
	return messagingapp.Route{
		ID: "orders.events", Exchange: "orders.events", ExchangeType: "topic", RoutingKey: "orders.created",
		Queue: "orders.created.v1", QueueType: "quorum", Reliable: true,
		Contract: contract.Ref(), ContentType: contract.ContentType(), MaxPayloadBytes: contract.MaxPayloadBytes(),
	}
}

var _ amqp.Acknowledger = (*recordingAcknowledger)(nil)
