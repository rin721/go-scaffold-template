package schedule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	pkgclock "github.com/rin721/go-scaffold-template/pkg/clock"
	pkgschedule "github.com/rin721/go-scaffold-template/pkg/schedule"
)

type engineJobID string

type triggerEngine interface {
	Validate(pkgschedule.Binding, string) error
	Add(pkgschedule.Binding, string, context.Context, func(context.Context) error) (engineJobID, error)
	Remove(engineJobID) error
	Start()
	Shutdown(context.Context) error
}

type gocronEngine struct {
	scheduler gocron.Scheduler
	clock     pkgclock.Clock
	mu        sync.Mutex
	jobs      map[engineJobID]uuid.UUID
}

func newGocronEngine(value Config, clock pkgclock.Clock) (triggerEngine, error) {
	if clock == nil {
		return nil, fmt.Errorf("scheduler clock is nil")
	}
	location, err := time.LoadLocation(value.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load scheduler location: %w", err)
	}
	scheduler, err := gocron.NewScheduler(
		gocron.WithLocation(location),
	)
	if err != nil {
		return nil, fmt.Errorf("create gocron scheduler: %w", err)
	}
	return &gocronEngine{scheduler: scheduler, clock: clock, jobs: make(map[engineJobID]uuid.UUID)}, nil
}

func (e *gocronEngine) Validate(binding pkgschedule.Binding, defaultTimezone string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	id, err := e.Add(binding, defaultTimezone, ctx, func(context.Context) error { return nil })
	if err != nil {
		return err
	}
	if err := e.Remove(id); err != nil {
		return fmt.Errorf("remove validation job %s: %w", binding.ID(), err)
	}
	return nil
}

func (e *gocronEngine) Add(
	binding pkgschedule.Binding,
	defaultTimezone string,
	ctx context.Context,
	run func(context.Context) error,
) (engineJobID, error) {
	if ctx == nil || run == nil {
		return "", fmt.Errorf("schedule job dependencies are incomplete")
	}
	definition, options, err := gocronDefinition(binding.Trigger(), defaultTimezone, e.clock.Now())
	if err != nil {
		return "", err
	}
	options = append(options, gocron.WithName(string(binding.ID())), gocron.WithContext(ctx))
	job, err := e.scheduler.NewJob(definition, gocron.NewTask(run), options...)
	if err != nil {
		return "", fmt.Errorf("create gocron job %s: %w", binding.ID(), err)
	}
	id := engineJobID(job.ID().String())
	e.mu.Lock()
	e.jobs[id] = job.ID()
	e.mu.Unlock()
	return id, nil
}

func gocronDefinition(trigger pkgschedule.Trigger, defaultTimezone string, now time.Time) (gocron.JobDefinition, []gocron.JobOption, error) {
	switch trigger.Kind() {
	case pkgschedule.TriggerCron:
		expression, timezone, withSeconds, ok := trigger.CronValues()
		if !ok {
			return nil, nil, fmt.Errorf("cron trigger values are unavailable")
		}
		if timezone == "" {
			timezone = defaultTimezone
		}
		if _, err := time.LoadLocation(timezone); err != nil {
			return nil, nil, fmt.Errorf("load task cron location: %w", err)
		}
		expression = "CRON_TZ=" + timezone + " " + expression
		return gocron.CronJob(expression, withSeconds), nil, nil
	case pkgschedule.TriggerFixedDelay:
		delay, initial, ok := trigger.FixedDelayValues()
		if !ok {
			return nil, nil, fmt.Errorf("fixedDelay trigger values are unavailable")
		}
		options := []gocron.JobOption{gocron.WithIntervalFromCompletion()}
		if initial > 0 {
			options = append(options, gocron.WithStartAt(gocron.WithStartDateTime(now.Add(initial))))
		}
		return gocron.DurationJob(delay), options, nil
	default:
		return nil, nil, fmt.Errorf("unsupported schedule trigger %q", trigger.Kind())
	}
}

func (e *gocronEngine) Remove(id engineJobID) error {
	e.mu.Lock()
	jobID, exists := e.jobs[id]
	e.mu.Unlock()
	if !exists {
		return nil
	}
	if err := e.scheduler.RemoveJob(jobID); err != nil {
		return fmt.Errorf("remove gocron job: %w", err)
	}
	e.mu.Lock()
	delete(e.jobs, id)
	e.mu.Unlock()
	return nil
}

func (e *gocronEngine) Start() { e.scheduler.Start() }

func (e *gocronEngine) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("gocron shutdown context is nil")
	}
	return e.scheduler.ShutdownWithContext(ctx)
}

var _ triggerEngine = (*gocronEngine)(nil)
