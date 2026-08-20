package messaging

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewMessageCopiesPayloadAndAcceptsUUID(t *testing.T) {
	contract := mustContract(t, "orders.created")
	payload := []byte(`{"orderId":"42"}`)
	message, err := NewMessage(MessageSpec{
		ID: "019fef13-5508-7650-9ade-39b8173ba33a", Contract: contract.Ref(),
		OccurredAt: time.Now(), Payload: payload,
	})
	if err != nil {
		t.Fatalf("构造消息失败: %v", err)
	}
	payload[0] = 'x'
	got := message.Payload()
	if got[0] != '{' {
		t.Fatalf("消息保留了调用方的可变 payload: %q", got)
	}
	got[0] = 'y'
	if message.Payload()[0] != '{' {
		t.Fatal("Payload 返回值泄漏了内部字节切片")
	}
}

func TestDeliveryAndConcurrencyPoliciesRejectUnsafeValues(t *testing.T) {
	if _, err := NewDeliveryPolicy(0, time.Second, 2*time.Second, time.Hour, DeadLetterRequired); !errors.Is(err, ErrInvalidDelivery) {
		t.Fatalf("期望非法交付次数，得到 %v", err)
	}
	if _, err := NewDeliveryPolicy(3, time.Second, time.Second, time.Hour, DeadLetterRequired); !errors.Is(err, ErrInvalidDelivery) {
		t.Fatalf("期望 processing lease 大于 handler timeout，得到 %v", err)
	}
	if _, err := NewConcurrencyPolicy(2, 1); !errors.Is(err, ErrInvalidConcurrency) {
		t.Fatalf("期望 prefetch 不小于并发数，得到 %v", err)
	}
}

func TestBuildCatalogAllowsSharedContractAndSortsBindings(t *testing.T) {
	contract := mustContract(t, "orders.created")
	producer := mustProducer(t, "orders.writer", contract.Ref(), "orders.events")
	consumer := mustConsumer(t, "orders.reader", contract.Ref(), "orders.events")

	catalog, err := BuildCatalog(
		Contribute([]Contract{contract}, []ProducerBinding{producer}, nil),
		Contribute([]Contract{contract}, nil, []ConsumerBinding{consumer}),
	)
	if err != nil {
		t.Fatalf("聚合共享 Contract 失败: %v", err)
	}
	if len(catalog.Contracts()) != 1 || len(catalog.Producers()) != 1 || len(catalog.Consumers()) != 1 {
		t.Fatalf("Catalog 数量不符合预期: %+v", catalog)
	}
}

func TestBuildCatalogRejectsConflictsAndInvalidZeroValues(t *testing.T) {
	contract := mustContract(t, "orders.created")
	conflict, err := DefineContract(ContractSpec{
		ID: contract.Ref().ID(), Version: contract.Ref().Version(), ContentType: "application/json",
		MaxPayloadBytes: 2048, Fingerprint: "sha256:different",
	})
	if err != nil {
		t.Fatal(err)
	}
	producer := mustProducer(t, "orders.writer", contract.Ref(), "orders.events")
	consumer := mustConsumer(t, "orders.reader", contract.Ref(), "orders.events")
	if _, err := BuildCatalog(Contribute([]Contract{contract, conflict}, []ProducerBinding{producer}, []ConsumerBinding{consumer})); !errors.Is(err, ErrContractConflict) {
		t.Fatalf("期望 Contract 冲突，得到 %v", err)
	}
	if _, err := BuildCatalog(Contribute(nil, []ProducerBinding{{}}, nil)); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("期望拒绝零值 Binding，得到 %v", err)
	}
}

func TestBuildCatalogRejectsUnknownAndUnusedContracts(t *testing.T) {
	contract := mustContract(t, "orders.created")
	producer := mustProducer(t, "orders.writer", contract.Ref(), "orders.events")
	if _, err := BuildCatalog(Contribute(nil, []ProducerBinding{producer}, nil)); !errors.Is(err, ErrUnknownContract) {
		t.Fatalf("期望未知 Contract，得到 %v", err)
	}
	if _, err := BuildCatalog(Contribute([]Contract{contract}, nil, nil)); !errors.Is(err, ErrUnusedContract) {
		t.Fatalf("期望未使用 Contract，得到 %v", err)
	}
}

func TestBindConsumerRejectsNilHandler(t *testing.T) {
	contract := mustContract(t, "orders.created")
	delivery := mustDeliveryPolicy(t)
	concurrency, err := NewConcurrencyPolicy(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = BindConsumer(ConsumerSpec{
		ID: "orders.reader", Contract: contract.Ref(), Route: "orders.events",
		Delivery: delivery, Concurrency: concurrency, Importance: ImportanceRequired,
	}, nil)
	if !errors.Is(err, ErrNilHandler) {
		t.Fatalf("期望 nil Handler 错误，得到 %v", err)
	}
}

func mustContract(t *testing.T, id ContractID) Contract {
	t.Helper()
	contract, err := DefineContract(ContractSpec{
		ID: id, Version: 1, ContentType: "application/json", MaxPayloadBytes: 1024,
		Fingerprint: "sha256:orders-created-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

func mustProducer(t *testing.T, id ProducerID, contract ContractRef, route RouteID) ProducerBinding {
	t.Helper()
	binding, err := BindProducer(ProducerSpec{ID: id, Contract: contract, Route: route, Confirm: ConfirmBroker})
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func mustConsumer(t *testing.T, id ConsumerID, contract ContractRef, route RouteID) ConsumerBinding {
	t.Helper()
	delivery := mustDeliveryPolicy(t)
	concurrency, err := NewConcurrencyPolicy(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := BindConsumer(ConsumerSpec{
		ID: id, Contract: contract, Route: route, Delivery: delivery,
		Concurrency: concurrency, Importance: ImportanceRequired,
	}, func(context.Context, Message) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func mustDeliveryPolicy(t *testing.T) DeliveryPolicy {
	t.Helper()
	policy, err := NewDeliveryPolicy(3, time.Second, 2*time.Second, time.Hour, DeadLetterRequired)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}
