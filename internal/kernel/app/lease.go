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

	state         leaseState
	current       *instanceSlot[I]
	activeUses    int
	transition    chan struct{}
	drained       chan struct{}
	drainDone     bool
	terminalDrain bool
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
			instance := l.current.instance
			l.mu.Unlock()
			var once sync.Once
			return instance, func() { once.Do(l.release) }, nil
		case leasePending:
			transition := l.transition
			l.mu.Unlock()
			select {
			case <-ctx.Done():
				return zero, nil, ctx.Err()
			case <-transition:
			}
		case leaseDraining:
			if l.terminalDrain {
				l.mu.Unlock()
				return zero, nil, ErrStopped
			}
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

func (l *lease[I]) publishInitial(slot *instanceSlot[I]) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != leasePending || slot == nil {
		panic("component lease initial publish from invalid state")
	}
	l.current = slot
	l.state = leaseServing
	close(l.transition)
}

func (l *lease[I]) beginOrContinueDrain(terminal bool) (<-chan struct{}, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch l.state {
	case leaseServing:
		l.state = leaseDraining
		l.terminalDrain = terminal
		l.transition = make(chan struct{})
		l.drained = make(chan struct{})
		l.drainDone = false
		if l.current != nil {
			l.current.transition(FinalizationPhaseCurrent, OwnershipWaitingForDrain)
		}
		if l.activeUses == 0 {
			close(l.drained)
			l.drainDone = true
		}
		return l.drained, nil
	case leaseDraining:
		if terminal {
			l.terminalDrain = true
		}
		return l.drained, nil
	case leaseStopped:
		closed := make(chan struct{})
		close(closed)
		return closed, nil
	default:
		return nil, fmt.Errorf("component lease cannot drain from state %d", l.state)
	}
}

func (l *lease[I]) replaceWhileDraining(slot *instanceSlot[I]) *instanceSlot[I] {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != leaseDraining || !l.drainDone || slot == nil || l.terminalDrain {
		panic("component lease replace before reload drain completed")
	}
	previous := l.current
	l.current = slot
	return previous
}

func (l *lease[I]) resume() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != leaseDraining || l.terminalDrain {
		panic("component lease resume from invalid state")
	}
	l.state = leaseServing
	if l.current != nil {
		l.current.transition(FinalizationPhaseCurrent, OwnershipServing)
	}
	close(l.transition)
	l.drained = nil
	l.drainDone = false
}

func (l *lease[I]) currentSnapshot(componentID ID) *OwnershipSnapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.current == nil {
		return nil
	}
	snapshot := l.current.snapshot(componentID)
	return &snapshot
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

func (l *lease[I]) takeWhileDraining() *instanceSlot[I] {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != leaseDraining || !l.drainDone {
		panic("component lease stop before terminal drain completed")
	}
	l.terminalDrain = true
	previous := l.current
	l.current = nil
	l.state = leaseStopped
	close(l.transition)
	l.drained = nil
	l.drainDone = false
	return previous
}
