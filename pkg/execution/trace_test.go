package execution

import (
	"context"
	"errors"
	"testing"
)

func TestTraceRoundTripThroughContext(t *testing.T) {
	ctx := WithTrace(context.Background(), "trace-abc-123")
	if got := TraceFrom(ctx); got != "trace-abc-123" {
		t.Fatalf("TraceFrom=%q want trace-abc-123", got)
	}
	if got := TraceFrom(context.Background()); got != "" {
		t.Fatalf("TraceFrom(background)=%q want empty", got)
	}
	if got := WithTrace(nil, "x"); got == nil {
		t.Fatal("WithTrace(nil) should return non-nil context")
	}
}

func TestExecutorRecordsTraceInFailureRecord(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)
	op := func(context.Context) (any, error) { return nil, errors.New("boom") }
	ctx := WithTrace(context.Background(), "trace-xyz-99")
	if _, err := executor.Execute(ctx, Execution{Key: "k", Operation: op}); err == nil {
		t.Fatal("want failure")
	}
	records, err := store.Records(context.Background(), "k")
	if err != nil || len(records) == 0 {
		t.Fatalf("records=%d err=%v", len(records), err)
	}
	if records[0].Trace != "trace-xyz-99" {
		t.Fatalf("record trace=%q want trace-xyz-99", records[0].Trace)
	}
}

func TestExecutorRecordsEmptyTraceWhenAbsent(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)
	if _, err := executor.Execute(context.Background(), Execution{
		Key: "k", Operation: func(context.Context) (any, error) { return nil, errors.New("boom") },
	}); err == nil {
		t.Fatal("want failure")
	}
	records, _ := store.Records(context.Background(), "k")
	if records[0].Trace != "" {
		t.Fatalf("record trace=%q want empty", records[0].Trace)
	}
}
