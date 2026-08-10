package kernel

import (
	"context"
	"fmt"
	"sync"
)

type handleState uint8

const (
	handlePending handleState = iota
	handleServing
	handleDraining
	handleStopped
)

// Handle 保存一个稳定能力入口背后的当前实例。
//
// Handle 只在 internal 范围内公开，以便具体 adapter 将它收敛为 adapter.Access。
type Handle[T any] struct {
	mu sync.Mutex

	state      handleState
	instance   T
	activeUses int
	transition chan struct{}
	drained    chan struct{}
	drainDone  bool
}

func newHandle[T any]() *Handle[T] {
	return &Handle[T]{transition: make(chan struct{})}
}

// Use 在一个受跟踪租约内调用当前实例。
func (h *Handle[T]) Use(ctx context.Context, use func(T) error) error {
	if ctx == nil {
		return ErrNilContext
	}
	if use == nil {
		return fmt.Errorf("kernel access callback is nil")
	}

	instance, release, err := h.acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	return use(instance)
}

func (h *Handle[T]) acquire(ctx context.Context) (T, func(), error) {
	var zero T
	for {
		h.mu.Lock()
		switch h.state {
		case handleServing:
			h.activeUses++
			instance := h.instance
			h.mu.Unlock()
			var once sync.Once
			return instance, func() {
				once.Do(h.release)
			}, nil
		case handlePending, handleDraining:
			transition := h.transition
			h.mu.Unlock()
			select {
			case <-ctx.Done():
				return zero, nil, ctx.Err()
			case <-transition:
			}
		case handleStopped:
			h.mu.Unlock()
			return zero, nil, ErrStopped
		default:
			h.mu.Unlock()
			return zero, nil, fmt.Errorf("unknown kernel handle state")
		}
	}
}

func (h *Handle[T]) release() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.activeUses--
	if h.activeUses < 0 {
		panic("kernel handle active use count became negative")
	}
	if h.state == handleDraining && h.activeUses == 0 && !h.drainDone {
		close(h.drained)
		h.drainDone = true
	}
}

func (h *Handle[T]) publishInitial(instance T) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state != handlePending {
		panic("kernel handle initial publish from invalid state")
	}
	h.instance = instance
	h.state = handleServing
	close(h.transition)
}

func (h *Handle[T]) beginDrain() (<-chan struct{}, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state != handleServing {
		return nil, fmt.Errorf("kernel handle cannot drain from state %d", h.state)
	}
	h.state = handleDraining
	h.transition = make(chan struct{})
	h.drained = make(chan struct{})
	h.drainDone = false
	if h.activeUses == 0 {
		close(h.drained)
		h.drainDone = true
	}
	return h.drained, nil
}

func (h *Handle[T]) replaceWhileDraining(instance T) T {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state != handleDraining || !h.drainDone {
		panic("kernel handle replace before drain completed")
	}
	previous := h.instance
	h.instance = instance
	return previous
}

func (h *Handle[T]) resume() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state != handleDraining {
		panic("kernel handle resume from invalid state")
	}
	h.state = handleServing
	close(h.transition)
	h.drained = nil
	h.drainDone = false
}

func (h *Handle[T]) stopPending() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state != handlePending {
		panic("kernel pending handle stop from invalid state")
	}
	h.state = handleStopped
	close(h.transition)
}

func (h *Handle[T]) stopWhileDraining() T {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state != handleDraining || !h.drainDone {
		panic("kernel handle stop before drain completed")
	}
	previous := h.instance
	var zero T
	h.instance = zero
	h.state = handleStopped
	close(h.transition)
	h.drained = nil
	h.drainDone = false
	return previous
}
