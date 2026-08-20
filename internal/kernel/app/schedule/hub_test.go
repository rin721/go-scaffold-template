package schedule

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/rin721/go-scaffold-template/pkg/health"
	pkgschedule "github.com/rin721/go-scaffold-template/pkg/schedule"
)

func TestHubCommitClosesOldAdmissionBeforeOpeningCandidate(t *testing.T) {
	var mu sync.Mutex
	sequence := make([]string, 0, 3)
	record := func(value string) {
		mu.Lock()
		sequence = append(sequence, value)
		mu.Unlock()
	}
	first := &fakeAccess{name: "first", generation: 1, record: record}
	second := &fakeAccess{name: "second", generation: 2, record: record}
	hub := NewHub()
	if err := hub.Commit(context.Background(), first); err != nil {
		t.Fatalf("commit first: %v", err)
	}
	if err := hub.Commit(context.Background(), second); err != nil {
		t.Fatalf("commit second: %v", err)
	}
	if got := fmt.Sprint(sequence); got != "[activate:first deactivate:first activate:second]" {
		t.Fatalf("sequence=%s", got)
	}
	diagnostics, err := hub.Diagnostics(context.Background())
	if err != nil {
		t.Fatalf("diagnostics: %v", err)
	}
	if diagnostics.Generation != 2 {
		t.Fatalf("current generation=%d want 2", diagnostics.Generation)
	}
	if err := hub.Retire(context.Background(), first); err != nil {
		t.Fatalf("retire old generation: %v", err)
	}
	if second.deactivated {
		t.Fatal("retiring an old generation must not close current admission")
	}
	if err := hub.Retire(context.Background(), second); err != nil {
		t.Fatalf("retire current generation: %v", err)
	}
	if !second.deactivated {
		t.Fatal("current generation should be deactivated")
	}
}

type fakeAccess struct {
	name        string
	generation  uint64
	record      func(string)
	deactivated bool
}

func (a *fakeAccess) Activate(context.Context) error {
	a.record("activate:" + a.name)
	a.deactivated = false
	return nil
}

func (a *fakeAccess) Deactivate(context.Context) error {
	a.record("deactivate:" + a.name)
	a.deactivated = true
	return nil
}

func (a *fakeAccess) Diagnostics(context.Context) (pkgschedule.Diagnostics, error) {
	return pkgschedule.Diagnostics{Ready: true, Generation: a.generation}, nil
}

func (*fakeAccess) Health(context.Context) (health.Result, error) {
	return health.Result{Status: health.StatusPass}, nil
}

var _ Access = (*fakeAccess)(nil)
