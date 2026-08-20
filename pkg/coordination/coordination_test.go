package coordination

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestValidateAcquire(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		key     Key
		options LeaseOptions
		want    error
	}{
		{name: "valid", ctx: context.Background(), key: "scheduler:app:task", options: LeaseOptions{TTL: time.Second}},
		{name: "nil context", key: "task", options: LeaseOptions{TTL: time.Second}, want: ErrNilContext},
		{name: "empty key", ctx: context.Background(), options: LeaseOptions{TTL: time.Second}, want: ErrInvalidKey},
		{name: "invalid ttl", ctx: context.Background(), key: "task", want: ErrInvalidTTL},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateAcquire(test.ctx, test.key, test.options)
			if !errors.Is(err, test.want) {
				t.Fatalf("ValidateAcquire() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestUnavailableManagerPreservesValidationAndUnavailable(t *testing.T) {
	manager := Unavailable()
	if _, err := manager.Acquire(context.Background(), "task", LeaseOptions{TTL: time.Second}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Acquire() error = %v", err)
	}
	if _, err := manager.Acquire(context.Background(), "", LeaseOptions{TTL: time.Second}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Acquire(empty) error = %v", err)
	}
}
