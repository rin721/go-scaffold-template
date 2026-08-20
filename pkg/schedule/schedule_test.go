package schedule

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTaskIDAndCronExpressionHaveBounds(t *testing.T) {
	trigger, err := FixedDelay(time.Second, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Bind(Spec{
		ID: TaskID("a" + strings.Repeat("b", maxTaskIDLength)), Trigger: trigger,
		Concurrency: SerialSkip(), Coordination: Local(),
	}, func(context.Context) error { return nil })
	if !errors.Is(err, ErrInvalidTaskID) {
		t.Fatalf("long task id error=%v", err)
	}
	_, err = Cron(strings.Repeat("a", maxCronExpressionSize+1)+" * * * *", "UTC", false)
	if !errors.Is(err, ErrInvalidTrigger) {
		t.Fatalf("long cron error=%v", err)
	}
}

func TestBindingPreservesTypedPolicies(t *testing.T) {
	trigger, err := Cron("*/5 * * * *", "Asia/Shanghai", false)
	if err != nil {
		t.Fatal(err)
	}
	coordination, err := DistributedSingleton(true, UnavailablePause)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := Bind(Spec{
		ID: "billing.reconcile", Trigger: trigger, Concurrency: SerialSkip(),
		Coordination: coordination, ExecutionPolicy: "billing",
	}, func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if binding.ID() != "billing.reconcile" || binding.Trigger().Kind() != TriggerCron {
		t.Fatalf("Binding = %#v", binding)
	}
	if binding.Coordination().Mode() != CoordinationDistributedStrict || binding.ExecutionPolicy() != "billing" {
		t.Fatalf("Binding policies = %#v", binding)
	}
}

func TestBindingRejectsInvalidCombinations(t *testing.T) {
	fixed, err := FixedDelay(time.Minute, 0)
	if err != nil {
		t.Fatal(err)
	}
	wait, err := Concurrency(2, CongestionWait, 3)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Bind(Spec{ID: "cleanup", Trigger: fixed, Concurrency: wait, Coordination: Local()}, func(context.Context) error { return nil })
	if !errors.Is(err, ErrInvalidConcurrency) {
		t.Fatalf("Bind() error = %v", err)
	}
	if _, err := DistributedSingleton(true, UnavailableLocal); !errors.Is(err, ErrInvalidCoordination) {
		t.Fatalf("DistributedSingleton() error = %v", err)
	}
}

func TestCronAndFixedDelayValidation(t *testing.T) {
	if _, err := Cron("* * * *", "UTC", false); !errors.Is(err, ErrInvalidTrigger) {
		t.Fatalf("Cron() error = %v", err)
	}
	if _, err := Cron("* * * * *", "Not/AZone", false); !errors.Is(err, ErrInvalidTrigger) {
		t.Fatalf("Cron(timezone) error = %v", err)
	}
	if _, err := FixedDelay(0, 0); !errors.Is(err, ErrInvalidTrigger) {
		t.Fatalf("FixedDelay() error = %v", err)
	}
}
