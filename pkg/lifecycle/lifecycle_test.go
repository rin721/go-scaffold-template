package lifecycle

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunnerStartsAndStopsInOrder(t *testing.T) {
	var events []string
	runner := New(Config{}, &recordParticipant{name: "db", events: &events}, &recordParticipant{name: "http", events: &events})
	if err := runner.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := runner.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	got := strings.Join(events, ",")
	want := "start:db,start:http,stop:http,stop:db"
	if got != want {
		t.Fatalf("events = %s, want %s", got, want)
	}
}

func TestRunnerJoinsStopErrors(t *testing.T) {
	firstErr := errors.New("first")
	secondErr := errors.New("second")
	runner := New(Config{ShutdownTimeout: time.Second},
		&recordParticipant{name: "first", stopErr: firstErr},
		&recordParticipant{name: "second", stopErr: secondErr},
	)
	err := runner.Stop(context.Background())
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("Stop() = %v, want both errors", err)
	}
}

type recordParticipant struct {
	name    string
	events  *[]string
	stopErr error
}

func (p *recordParticipant) Name() string {
	return p.name
}

func (p *recordParticipant) Start(context.Context) error {
	if p.events != nil {
		*p.events = append(*p.events, "start:"+p.name)
	}
	return nil
}

func (p *recordParticipant) Stop(context.Context) error {
	if p.events != nil {
		*p.events = append(*p.events, "stop:"+p.name)
	}
	return p.stopErr
}
