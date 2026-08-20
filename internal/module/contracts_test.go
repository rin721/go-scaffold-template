package module

import (
	"context"
	"testing"
	"time"

	pkgmessaging "github.com/rin721/go-scaffold-template/pkg/messaging"
	pkgschedule "github.com/rin721/go-scaffold-template/pkg/schedule"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

func TestValidateContributionsAcceptsParticipant(t *testing.T) {
	err := ValidateContributions(Contribution{
		ID: "todo", Participants: []supervisor.Participant{testParticipant("todo.schema")},
	})
	if err != nil {
		t.Fatalf("ValidateContributions() error = %v", err)
	}
}

func TestValidateContributionsRejectsOwnershipConflicts(t *testing.T) {
	scheduled := testSchedule(t, "todo.cleanup")
	tests := []struct {
		name          string
		contributions []Contribution
	}{
		{name: "duplicate module", contributions: []Contribution{{ID: "todo"}, {ID: "todo"}}},
		{name: "empty module", contributions: []Contribution{{Participants: []supervisor.Participant{testParticipant("schema")}}}},
		{name: "duplicate participant", contributions: []Contribution{
			{ID: "todo", Participants: []supervisor.Participant{testParticipant("schema")}},
			{ID: "other", Participants: []supervisor.Participant{testParticipant("schema")}},
		}},
		{name: "duplicate schedule", contributions: []Contribution{
			{ID: "todo", Schedules: []pkgschedule.Binding{scheduled}},
			{ID: "other", Schedules: []pkgschedule.Binding{scheduled}},
		}},
		{name: "invalid schedule", contributions: []Contribution{
			{ID: "todo", Schedules: []pkgschedule.Binding{{}}},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateContributions(test.contributions...); err == nil {
				t.Fatal("ValidateContributions() error = nil")
			}
		})
	}
}

func TestScheduleBindingsReturnsStableOrder(t *testing.T) {
	bindings, err := ScheduleBindings(
		Contribution{ID: "billing", Schedules: []pkgschedule.Binding{testSchedule(t, "billing.reconcile")}},
		Contribution{ID: "todo", Schedules: []pkgschedule.Binding{testSchedule(t, "todo.cleanup")}},
	)
	if err != nil {
		t.Fatalf("ScheduleBindings() error = %v", err)
	}
	if len(bindings) != 2 || bindings[0].ID() != "billing.reconcile" || bindings[1].ID() != "todo.cleanup" {
		t.Fatalf("ScheduleBindings() = %#v", bindings)
	}
}

func TestMessageBindingsAggregatesModuleContributions(t *testing.T) {
	contract, err := pkgmessaging.DefineContract(pkgmessaging.ContractSpec{
		ID: "orders.created", Version: 1, ContentType: "application/json",
		MaxPayloadBytes: 1024, Fingerprint: "sha256:orders-created-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	producer, err := pkgmessaging.BindProducer(pkgmessaging.ProducerSpec{
		ID: "orders.writer", Contract: contract.Ref(), Route: "orders.events",
		Confirm: pkgmessaging.ConfirmBroker,
	})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := MessageBindings(Contribution{
		ID:       "orders",
		Messages: pkgmessaging.Contribute([]pkgmessaging.Contract{contract}, []pkgmessaging.ProducerBinding{producer}, nil),
	})
	if err != nil {
		t.Fatalf("MessageBindings() error = %v", err)
	}
	if len(catalog.Contracts()) != 1 || len(catalog.Producers()) != 1 {
		t.Fatalf("MessageBindings() = %+v", catalog)
	}
}

func TestMessageBindingsRejectsCrossModuleProducerConflict(t *testing.T) {
	contract, err := pkgmessaging.DefineContract(pkgmessaging.ContractSpec{
		ID: "orders.created", Version: 1, ContentType: "application/json",
		MaxPayloadBytes: 1024, Fingerprint: "sha256:orders-created-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	producer, err := pkgmessaging.BindProducer(pkgmessaging.ProducerSpec{
		ID: "orders.writer", Contract: contract.Ref(), Route: "orders.events",
		Confirm: pkgmessaging.ConfirmBroker,
	})
	if err != nil {
		t.Fatal(err)
	}
	messages := pkgmessaging.Contribute([]pkgmessaging.Contract{contract}, []pkgmessaging.ProducerBinding{producer}, nil)
	if _, err := MessageBindings(
		Contribution{ID: "orders", Messages: messages},
		Contribution{ID: "billing", Messages: messages},
	); err == nil {
		t.Fatal("MessageBindings() error = nil")
	}
}

func testSchedule(t *testing.T, id pkgschedule.TaskID) pkgschedule.Binding {
	t.Helper()
	trigger, err := pkgschedule.FixedDelay(time.Minute, 0)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := pkgschedule.Bind(pkgschedule.Spec{
		ID: id, Trigger: trigger, Concurrency: pkgschedule.SerialSkip(), Coordination: pkgschedule.Local(),
	}, func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

type testParticipant string

func (p testParticipant) Name() string              { return string(p) }
func (testParticipant) Start(context.Context) error { return nil }
func (testParticipant) Stop(context.Context) error  { return nil }
