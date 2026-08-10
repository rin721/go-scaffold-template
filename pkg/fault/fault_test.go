package fault

import (
	"context"
	"errors"
	"testing"
)

func TestWrapKeepsCauseCodeAndRetryable(t *testing.T) {
	cause := errors.New("dial failed")
	err := Wrap(cause, CodeUnavailable, "redis ping", true)
	if !errors.Is(err, cause) {
		t.Fatal("wrapped error does not keep cause")
	}
	if got := CodeOf(err); got != CodeUnavailable {
		t.Fatalf("CodeOf() = %q, want %q", got, CodeUnavailable)
	}
	if !Retryable(err) {
		t.Fatal("Retryable() = false, want true")
	}
}

func TestCodeOfContextErrors(t *testing.T) {
	if got := CodeOf(context.Canceled); got != CodeCanceled {
		t.Fatalf("CodeOf(context.Canceled) = %q", got)
	}
	if got := CodeOf(context.DeadlineExceeded); got != CodeTimeout {
		t.Fatalf("CodeOf(context.DeadlineExceeded) = %q", got)
	}
}

func TestJoinCloseKeepsBothErrors(t *testing.T) {
	primary := errors.New("run failed")
	closeErr := errors.New("flush failed")
	err := JoinClose(primary, "logger", closeErr)
	if !errors.Is(err, primary) || !errors.Is(err, closeErr) {
		t.Fatalf("JoinClose() = %v, want both errors", err)
	}
}
