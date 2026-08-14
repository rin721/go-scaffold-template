package module

import (
	"context"
	"testing"

	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

func TestValidateContributionsAcceptsCanonicalRoute(t *testing.T) {
	err := ValidateContributions(Contribution{ID: "todo", Routes: []Route{{
		Method: httpx.MethodGet, Path: "/api/v1/todos/{id}", Handler: func(*httpx.Context) error { return nil },
	}}, Participants: []supervisor.Participant{testParticipant("todo.schema")}})
	if err != nil {
		t.Fatalf("ValidateContributions() error = %v", err)
	}
}

func TestValidateContributionsRejectsConflictsAndInvalidRoutes(t *testing.T) {
	handler := func(*httpx.Context) error { return nil }
	tests := []struct {
		name          string
		contributions []Contribution
	}{
		{name: "duplicate module", contributions: []Contribution{{ID: "todo"}, {ID: "todo"}}},
		{name: "duplicate route", contributions: []Contribution{
			{ID: "todo", Routes: []Route{{Method: httpx.MethodGet, Path: "/todos", Handler: handler}}},
			{ID: "other", Routes: []Route{{Method: httpx.MethodGet, Path: "/todos", Handler: handler}}},
		}},
		{name: "non canonical", contributions: []Contribution{{ID: "todo", Routes: []Route{{Method: httpx.MethodGet, Path: "/todos/../todos", Handler: handler}}}}},
		{name: "nil handler", contributions: []Contribution{{ID: "todo", Routes: []Route{{Method: httpx.MethodGet, Path: "/todos"}}}}},
		{name: "duplicate participant", contributions: []Contribution{
			{ID: "todo", Participants: []supervisor.Participant{testParticipant("schema")}},
			{ID: "other", Participants: []supervisor.Participant{testParticipant("schema")}},
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

type testParticipant string

func (p testParticipant) Name() string              { return string(p) }
func (testParticipant) Start(context.Context) error { return nil }
func (testParticipant) Stop(context.Context) error  { return nil }
