package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/module/auth/model"
	"github.com/rin721/go-scaffold-template/pkg/clock"
)

func TestServiceFailsClosedAndEnforcesOperationAndOwner(t *testing.T) {
	now := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	principal, err := model.NewPrincipal("actor-a", model.ActorService, []model.Scope{"todos:read"}, now, now)
	if err != nil {
		t.Fatalf("NewPrincipal() error = %v", err)
	}
	verifier := &testVerifier{ready: true, principal: principal}
	audit := &testAudit{}
	service, err := New(clock.Fixed(now), verifier, nil, audit, []model.Policy{
		{Operation: "getTodo", Mode: model.PolicyProtected, Scope: "todos:read", Action: "todo.read"},
		{Operation: "createTodo", Mode: model.PolicyProtected, Scope: "todos:write", Action: "todo.create"},
		{Operation: "live", Mode: model.PolicyPublic},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	verified, err := service.Authenticate(t.Context(), model.Credential{Scheme: "Bearer", Value: "opaque"})
	if err != nil || verified.Subject != principal.Subject {
		t.Fatalf("Authenticate() = %#v, %v", verified, err)
	}
	decision, err := service.AuthorizeOperation(t.Context(), verified, "getTodo")
	if err != nil || !decision.Allowed {
		t.Fatalf("AuthorizeOperation(read) = %#v, %v", decision, err)
	}
	decision, err = service.AuthorizeOperation(t.Context(), verified, "createTodo")
	if err != nil || decision.Allowed || decision.Reason != model.ReasonMissingScope {
		t.Fatalf("AuthorizeOperation(write) = %#v, %v", decision, err)
	}
	decision, err = service.AuthorizeAction(t.Context(), verified, "todo.read", model.ResourceFacts{OwnerSubject: "actor-b"})
	if err != nil || decision.Allowed || decision.Reason != model.ReasonOwnerMismatch {
		t.Fatalf("AuthorizeAction(other owner) = %#v, %v", decision, err)
	}
	decision, err = service.AuthorizeOperation(t.Context(), model.Principal{}, "live")
	if err != nil || !decision.Allowed || decision.Reason != model.ReasonPublic {
		t.Fatalf("AuthorizeOperation(public) = %#v, %v", decision, err)
	}
	decision, err = service.AuthorizeOperation(t.Context(), verified, "unknown")
	if err != nil || decision.Allowed || decision.Reason != model.ReasonMissingPolicy {
		t.Fatalf("AuthorizeOperation(unknown) = %#v, %v", decision, err)
	}

	verifier.ready = false
	if _, err := service.Authenticate(t.Context(), model.Credential{Scheme: "Bearer", Value: "opaque"}); !errors.Is(err, model.ErrUnauthenticated) {
		t.Fatalf("Authenticate(not ready) error = %v", err)
	}
}

func TestServiceRejectsIncompleteAndDuplicatePolicies(t *testing.T) {
	now := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	principal, err := model.NewPrincipal("development", model.ActorDevelopment, []model.Scope{"todos:read"}, now, now)
	if err != nil {
		t.Fatalf("NewPrincipal() error = %v", err)
	}
	tests := [][]model.Policy{
		nil,
		{{Operation: "read", Mode: model.PolicyProtected, Scope: "todos:read"}},
		{{Operation: "read", Mode: model.PolicyPublic, Scope: "todos:read"}},
		{
			{Operation: "read", Mode: model.PolicyProtected, Scope: "todos:read", Action: "todo.read"},
			{Operation: "read", Mode: model.PolicyProtected, Scope: "todos:read", Action: "todo.read-again"},
		},
	}
	for _, policies := range tests {
		if _, err := New(clock.Fixed(now), nil, &principal, &testAudit{}, policies); err == nil {
			t.Fatalf("New(%#v) error = nil", policies)
		}
	}
}

type testVerifier struct {
	ready     bool
	principal model.Principal
}

func (v *testVerifier) Verify(context.Context, model.Credential) (model.Principal, error) {
	return v.principal, nil
}

func (v *testVerifier) Ready() bool { return v.ready }

type testAudit struct{ events []model.AuditEvent }

func (a *testAudit) Record(_ context.Context, event model.AuditEvent) error {
	a.events = append(a.events, event)
	return nil
}
