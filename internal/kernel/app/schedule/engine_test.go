package schedule

import (
	"context"
	"testing"
	"time"

	pkgclock "github.com/rin721/go-scaffold-template/pkg/clock"
	pkgschedule "github.com/rin721/go-scaffold-template/pkg/schedule"
)

func TestGocronEngineRejectsInvalidCronAtPrepare(t *testing.T) {
	trigger, err := pkgschedule.Cron("invalid * * * *", "UTC", false)
	if err != nil {
		t.Fatalf("project trigger validation should leave parser validation to adapter: %v", err)
	}
	binding, err := pkgschedule.Bind(pkgschedule.Spec{
		ID: "invalid.cron", Trigger: trigger, Concurrency: pkgschedule.SerialSkip(), Coordination: pkgschedule.Local(),
	}, func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	engine, err := newGocronEngine(defaultConfig(), pkgclock.System())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if shutdownErr := engine.Shutdown(ctx); shutdownErr != nil {
			t.Errorf("shutdown: %v", shutdownErr)
		}
	}()
	if err := engine.Validate(binding, "UTC"); err == nil {
		t.Fatal("gocron parser should reject an invalid cron expression")
	}
}

func TestGocronEngineValidatesAndRemovesCronBeforeStart(t *testing.T) {
	trigger, err := pkgschedule.Cron("*/5 * * * *", "Asia/Shanghai", false)
	if err != nil {
		t.Fatalf("cron: %v", err)
	}
	binding, err := pkgschedule.Bind(pkgschedule.Spec{
		ID: "valid.cron", Trigger: trigger, Concurrency: pkgschedule.SerialSkip(), Coordination: pkgschedule.Local(),
	}, func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	engine, err := newGocronEngine(defaultConfig(), pkgclock.System())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if shutdownErr := engine.Shutdown(ctx); shutdownErr != nil {
			t.Errorf("shutdown: %v", shutdownErr)
		}
	}()
	if err := engine.Validate(binding, "UTC"); err != nil {
		t.Fatalf("validate cron before start: %v", err)
	}
}

func TestGocronFixedDelayStartsNextRunAfterCompletion(t *testing.T) {
	const delay = 60 * time.Millisecond
	trigger, err := pkgschedule.FixedDelay(delay, time.Millisecond)
	if err != nil {
		t.Fatalf("fixedDelay: %v", err)
	}
	starts := make(chan time.Time, 2)
	completions := make(chan time.Time, 2)
	binding, err := pkgschedule.Bind(pkgschedule.Spec{
		ID: "fixed.delay", Trigger: trigger, Concurrency: pkgschedule.SerialSkip(), Coordination: pkgschedule.Local(),
	}, func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	engine, err := newGocronEngine(defaultConfig(), pkgclock.System())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = engine.Add(binding, "UTC", ctx, func(context.Context) error {
		starts <- time.Now()
		time.Sleep(25 * time.Millisecond)
		completions <- time.Now()
		return nil
	})
	if err != nil {
		t.Fatalf("add fixedDelay job: %v", err)
	}
	engine.Start()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		if shutdownErr := engine.Shutdown(shutdownCtx); shutdownErr != nil {
			t.Errorf("shutdown: %v", shutdownErr)
		}
	}()
	firstStart := receiveTime(t, starts)
	firstCompletion := receiveTime(t, completions)
	secondStart := receiveTime(t, starts)
	if firstCompletion.Before(firstStart) {
		t.Fatalf("completion %s preceded start %s", firstCompletion, firstStart)
	}
	if gap := secondStart.Sub(firstCompletion); gap < delay-15*time.Millisecond {
		t.Fatalf("fixedDelay gap=%s want at least %s after completion", gap, delay-15*time.Millisecond)
	}
}

func receiveTime(t *testing.T, values <-chan time.Time) time.Time {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for gocron job")
		return time.Time{}
	}
}
