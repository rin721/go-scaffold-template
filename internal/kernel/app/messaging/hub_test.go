package messaging

import (
	"context"
	"errors"
	"testing"

	"github.com/rin721/go-scaffold-template/pkg/health"
	pkgmessaging "github.com/rin721/go-scaffold-template/pkg/messaging"
)

func TestHubRestoresPreviousAdmissionWhenCandidateActivationFails(t *testing.T) {
	hub := NewHub()
	previous := &recordingControl{}
	if err := hub.Commit(t.Context(), previous); err != nil {
		t.Fatal(err)
	}
	candidate := &recordingControl{activateErr: errors.New("candidate unavailable")}
	if err := hub.Commit(t.Context(), candidate); err == nil {
		t.Fatal("Commit() error = nil")
	}
	if previous.activations != 2 || previous.deactivations != 1 {
		t.Fatalf("previous activations=%d deactivations=%d", previous.activations, previous.deactivations)
	}
	if hub.current != previous {
		t.Fatal("candidate 失败后 Hub 未保留旧代")
	}
}

func TestHubRestoresPreviousAdmissionWhenHandoffTimesOut(t *testing.T) {
	hub := NewHub()
	previous := &recordingControl{}
	if err := hub.Commit(t.Context(), previous); err != nil {
		t.Fatal(err)
	}
	previous.deactivateErr = context.DeadlineExceeded
	if err := hub.Commit(t.Context(), &recordingControl{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Commit() error = %v", err)
	}
	if previous.activations != 2 || previous.deactivations != 1 {
		t.Fatalf("previous activations=%d deactivations=%d", previous.activations, previous.deactivations)
	}
	if hub.current != previous {
		t.Fatal("handoff 失败后 Hub 未保留旧代")
	}
}

type recordingControl struct {
	activations   int
	deactivations int
	activateErr   error
	deactivateErr error
}

func (*recordingControl) Freeze(pkgmessaging.Catalog) error   { return nil }
func (*recordingControl) OpenPublisher(context.Context) error { return nil }
func (c *recordingControl) Activate(context.Context) error {
	c.activations++
	return c.activateErr
}
func (c *recordingControl) Deactivate(context.Context) error {
	c.deactivations++
	return c.deactivateErr
}
func (*recordingControl) Diagnostics(context.Context) (pkgmessaging.Diagnostics, error) {
	return pkgmessaging.Diagnostics{}, nil
}
func (*recordingControl) Health(context.Context) (health.Result, error) {
	return health.Result{}, nil
}

var _ Control = (*recordingControl)(nil)
