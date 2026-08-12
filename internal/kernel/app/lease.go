package app

import (
	"context"
	"fmt"
	"sync"
)

type leaseState uint8

const (
	leasePending leaseState = iota
	leaseServing
	leaseDraining
	leaseStopped
)

type lease[I any] struct {
	mu sync.Mutex

	state      leaseState
	instance   I
	activeUses int
	transition chan struct{}
	drained    chan struct{}
	drainDone  bool
}

func newLease[I any]() *lease[I] {
	return &lease[I]{transition: make(chan struct{})}
}

func (l *lease[I]) Use(ctx context.Context, use func(I) error) error {
	if ctx == nil {
		return ErrNilContext
	}
	if use == nil {
		return fmt.Errorf("component access callback is nil")
	}
	instance, release, err := l.acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	return use(instance)
}

func (l *lease[I]) acquire(ctx context.Context) (I, func(), error) {
	var zero I
	for {
		l.mu.Lock()
		switch l.state {
		case leaseServing:
			l.activeUses++
			instance := l.instance
			l.mu.Unlock()
			var once sync.Once
			return instance, func() { once.Do(l.release) }, nil
		case leasePending, leaseDraining:
			transition := l.transition
			l.mu.Unlock()
			select {
			case <-ctx.Done():
				return zero, nil, ctx.Err()
			case <-transition:
			}
		case leaseStopped:
			l.mu.Unlock()
			return zero, nil, ErrStopped
		default:
			l.mu.Unlock()
			return zero, nil, fmt.Errorf("unknown component lease state")
		}
	}
}

func (l *lease[I]) release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.activeUses--
	if l.activeUses < 0 {
		panic("component lease active use count became negative")
	}
	if l.state == leaseDraining && l.activeUses == 0 && !l.drainDone {
		close(l.drained)
		l.drainDone = true
	}
}

func (l *lease[I]) publishInitial(instance I) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != leasePending {
		panic("component lease initial publish from invalid state")
	}
	l.instance = instance
	l.state = leaseServing
	close(l.transition)
}

func (l *lease[I]) beginDrain() (<-chan struct{}, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != leaseServing {
		return nil, fmt.Errorf("component lease cannot drain from state %d", l.state)
	}
	l.state = leaseDraining
	l.transition = make(chan struct{})
	l.drained = make(chan struct{})
	l.drainDone = false
	if l.activeUses == 0 {
		close(l.drained)
		l.drainDone = true
	}
	return l.drained, nil
}

func (l *lease[I]) replaceWhileDraining(instance I) I {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != leaseDraining || !l.drainDone {
		panic("component lease replace before drain completed")
	}
	previous := l.instance
	l.instance = instance
	return previous
}

func (l *lease[I]) resume() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != leaseDraining {
		panic("component lease resume from invalid state")
	}
	l.state = leaseServing
	close(l.transition)
	l.drained = nil
	l.drainDone = false
}

func (l *lease[I]) stopPending() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != leasePending {
		return
	}
	l.state = leaseStopped
	close(l.transition)
}

func (l *lease[I]) takeWhileDraining() I {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != leaseDraining || !l.drainDone {
		panic("component lease stop before drain completed")
	}
	previous := l.instance
	var zero I
	l.instance = zero
	l.state = leaseStopped
	close(l.transition)
	l.drained = nil
	l.drainDone = false
	return previous
}
