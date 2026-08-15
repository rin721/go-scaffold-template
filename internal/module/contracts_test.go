package module

import (
	"context"
	"testing"

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
