package httpx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
)

const defaultListenerRouteCapacity = 64

// ListenerHub 在进程内独占物理 TCP listener，并把连接交给当前 HTTP generation。
type ListenerHub struct {
	mu       sync.Mutex
	entries  map[string]*listenerEntry
	capacity int
	closed   bool
}

// PreparedRoute 是已完成物理 bind、但尚未获得生产连接 admission 的候选 route。
type PreparedRoute struct {
	hub        *ListenerHub
	entry      *listenerEntry
	route      *generationListener
	configured string

	mu        sync.Mutex
	committed bool
	aborted   bool
}

type listenerEntry struct {
	hub        *ListenerHub
	configured string
	physical   net.Listener

	dispatchMu sync.Mutex
	mu         sync.Mutex
	active     *generationListener
	retiring   *generationListener
	activate   chan struct{}
	stop       chan struct{}
	done       chan struct{}
	stopOnce   sync.Once
	err        error
}

type generationListener struct {
	address net.Addr
	queue   chan net.Conn

	mu          sync.Mutex
	retiring    bool
	closed      bool
	drained     chan struct{}
	drainedOnce sync.Once
	closeSignal chan struct{}
	closeOnce   sync.Once
}

// NewListenerHub 创建尚未绑定地址的进程级 listener owner。
func NewListenerHub(queueCapacity int) (*ListenerHub, error) {
	if queueCapacity < 0 {
		return nil, fmt.Errorf("listener route capacity must be non-negative")
	}
	if queueCapacity == 0 {
		queueCapacity = defaultListenerRouteCapacity
	}
	return &ListenerHub{entries: make(map[string]*listenerEntry), capacity: queueCapacity}, nil
}

// Prepare 同步完成目标地址 bind，并返回尚未激活的虚拟 listener。
func (h *ListenerHub) Prepare(ctx context.Context, address string) (*PreparedRoute, error) {
	if h == nil {
		return nil, fmt.Errorf("listener hub is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("listener hub prepare context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if address == "" {
		return nil, fmt.Errorf("listener hub address is empty")
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, net.ErrClosed
	}
	entry := h.entries[address]
	if entry == nil {
		physical, err := net.Listen("tcp", address)
		if err != nil {
			return nil, fmt.Errorf("bind listener hub address %s: %w", address, err)
		}
		entry = &listenerEntry{
			hub: h, configured: address, physical: physical,
			activate: make(chan struct{}), stop: make(chan struct{}), done: make(chan struct{}),
		}
		h.entries[address] = entry
		go entry.run()
	}
	entry.mu.Lock()
	if entry.err != nil {
		err := entry.err
		entry.mu.Unlock()
		return nil, fmt.Errorf("listener hub address %s failed: %w", address, err)
	}
	if entry.active == nil && entry.retiring != nil {
		entry.mu.Unlock()
		return nil, fmt.Errorf("listener hub address %s is retiring", address)
	}
	route := newGenerationListener(entry.physical.Addr(), h.capacity)
	entry.mu.Unlock()
	return &PreparedRoute{hub: h, entry: entry, route: route, configured: address}, nil
}

// Listener 返回候选 http.Server 使用的虚拟 listener。
func (p *PreparedRoute) Listener() net.Listener {
	if p == nil {
		return nil
	}
	return p.route
}

// ConfiguredAddress 返回配置中的地址。
func (p *PreparedRoute) ConfiguredAddress() string {
	if p == nil {
		return ""
	}
	return p.configured
}

// BoundAddress 返回物理 listener 的实际地址。
func (p *PreparedRoute) BoundAddress() net.Addr {
	if p == nil || p.route == nil {
		return nil
	}
	return p.route.Addr()
}

// Commit 在 dispatch 屏障内把新连接线性切换到 next，并返回旧 route。
func (h *ListenerHub) Commit(next, previous *PreparedRoute) (*PreparedRoute, error) {
	if h == nil || next == nil || next.hub != h || next.entry == nil || next.route == nil {
		return nil, fmt.Errorf("listener hub next route is invalid")
	}
	next.mu.Lock()
	if next.committed || next.aborted {
		next.mu.Unlock()
		return nil, fmt.Errorf("listener hub next route is already settled")
	}
	next.mu.Unlock()

	if previous != nil && previous.entry != next.entry {
		previous.entry.retirePhysical(previous.route)
	}
	next.entry.dispatchMu.Lock()
	next.entry.mu.Lock()
	current := next.entry.active
	if previous != nil && previous.entry == next.entry && current != previous.route {
		next.entry.mu.Unlock()
		next.entry.dispatchMu.Unlock()
		return nil, fmt.Errorf("listener hub previous route is not active")
	}
	next.entry.active = next.route
	next.entry.retiring = current
	next.entry.mu.Unlock()
	next.entry.dispatchMu.Unlock()
	next.entry.startAccepting()

	next.mu.Lock()
	next.committed = true
	next.mu.Unlock()
	if current != nil {
		current.retire()
	}
	return previous, nil
}

// Abort 释放未提交候选；新地址没有 active route 时同时释放物理 listener。
func (h *ListenerHub) Abort(candidate *PreparedRoute) error {
	if candidate == nil {
		return nil
	}
	if candidate.hub != h {
		return fmt.Errorf("listener route belongs to another hub")
	}
	candidate.mu.Lock()
	if candidate.committed {
		candidate.mu.Unlock()
		return fmt.Errorf("listener route is already committed")
	}
	if candidate.aborted {
		candidate.mu.Unlock()
		return nil
	}
	candidate.aborted = true
	candidate.mu.Unlock()
	_ = candidate.route.Close()

	h.mu.Lock()
	entry := candidate.entry
	entry.mu.Lock()
	unused := entry.active == nil && entry.retiring == nil
	entry.mu.Unlock()
	if unused && h.entries[candidate.configured] == entry {
		delete(h.entries, candidate.configured)
		entry.stopEntry()
	}
	h.mu.Unlock()
	if unused {
		<-entry.done
		return entry.result()
	}
	return nil
}

// Retire 停止 current route 的新连接 admission。相同地址换代由 Commit 调用
// route.retire；进程停止或地址迁移时本方法同时关闭物理 listener。
func (h *ListenerHub) Retire(current *PreparedRoute) error {
	if h == nil || current == nil || current.hub != h || current.entry == nil {
		return fmt.Errorf("listener hub current route is invalid")
	}
	current.entry.dispatchMu.Lock()
	current.entry.mu.Lock()
	active := current.entry.active == current.route
	if active {
		current.entry.active = nil
		current.entry.retiring = current.route
	}
	current.entry.mu.Unlock()
	if active {
		current.entry.stopOnce.Do(func() {
			close(current.entry.stop)
			_ = current.entry.physical.Close()
		})
	}
	current.entry.dispatchMu.Unlock()
	current.route.retire()
	return nil
}

// Release 在 generation Server 已停止后清除 route 引用；没有 active route 的
// 已关闭地址会从 Hub 中移除。
func (h *ListenerHub) Release(route *PreparedRoute) error {
	if h == nil || route == nil || route.hub != h || route.entry == nil {
		return fmt.Errorf("listener hub release route is invalid")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	entry := route.entry
	entry.mu.Lock()
	if entry.active == route.route {
		entry.mu.Unlock()
		return fmt.Errorf("listener hub cannot release active route")
	}
	if entry.retiring == route.route {
		entry.retiring = nil
	}
	remove := entry.active == nil && entry.retiring == nil
	entry.mu.Unlock()
	if remove && h.entries[entry.configured] == entry {
		delete(h.entries, entry.configured)
		entry.stopEntry()
	}
	return nil
}

// Stop 关闭全部物理 listener，并等待 accept owner 退出。
func (h *ListenerHub) Stop(ctx context.Context) error {
	if h == nil {
		return fmt.Errorf("listener hub is nil")
	}
	if ctx == nil {
		return fmt.Errorf("listener hub stop context is nil")
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	entries := make([]*listenerEntry, 0, len(h.entries))
	for _, entry := range h.entries {
		entries = append(entries, entry)
		entry.stopEntry()
	}
	h.entries = make(map[string]*listenerEntry)
	h.mu.Unlock()
	var joined error
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return errors.Join(joined, ctx.Err())
		case <-entry.done:
			joined = errors.Join(joined, entry.result())
		}
	}
	return joined
}

func (e *listenerEntry) startAccepting() {
	select {
	case <-e.activate:
	default:
		close(e.activate)
	}
}

func (e *listenerEntry) retirePhysical(route *generationListener) {
	e.dispatchMu.Lock()
	e.mu.Lock()
	e.active = nil
	e.retiring = route
	e.mu.Unlock()
	e.stopOnce.Do(func() {
		close(e.stop)
		_ = e.physical.Close()
	})
	e.dispatchMu.Unlock()
	if route != nil {
		route.retire()
	}
}

func (e *listenerEntry) stopEntry() {
	e.stopOnce.Do(func() {
		close(e.stop)
		_ = e.physical.Close()
	})
}

func (e *listenerEntry) run() {
	defer close(e.done)
	select {
	case <-e.activate:
	case <-e.stop:
		return
	}
	for {
		connection, err := e.physical.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			e.mu.Lock()
			e.err = fmt.Errorf("accept physical listener: %w", err)
			e.mu.Unlock()
			return
		}
		e.dispatchMu.Lock()
		e.mu.Lock()
		target := e.active
		if target == nil {
			target = e.retiring
		}
		e.mu.Unlock()
		if target == nil {
			_ = connection.Close()
			e.dispatchMu.Unlock()
			continue
		}
		err = target.dispatch(connection, e.stop)
		e.dispatchMu.Unlock()
		if err != nil {
			_ = connection.Close()
		}
	}
}

func (e *listenerEntry) result() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.err
}

func newGenerationListener(address net.Addr, capacity int) *generationListener {
	return &generationListener{
		address: address, queue: make(chan net.Conn, capacity),
		drained: make(chan struct{}), closeSignal: make(chan struct{}),
	}
}

func (l *generationListener) Accept() (net.Conn, error) {
	for {
		select {
		case connection := <-l.queue:
			l.markDrainedIfNeeded()
			return connection, nil
		default:
		}
		l.mu.Lock()
		closed := l.closed
		l.mu.Unlock()
		if closed {
			return nil, net.ErrClosed
		}
		select {
		case connection := <-l.queue:
			l.markDrainedIfNeeded()
			return connection, nil
		case <-l.closeSignal:
		}
	}
}

func (l *generationListener) Close() error {
	l.mu.Lock()
	l.closed = true
	for {
		select {
		case connection := <-l.queue:
			_ = connection.Close()
		default:
			l.mu.Unlock()
			l.closeOnce.Do(func() { close(l.closeSignal) })
			l.markDrainedIfNeeded()
			return nil
		}
	}
}

func (l *generationListener) Addr() net.Addr { return l.address }

func (l *generationListener) dispatch(connection net.Conn, hubStop <-chan struct{}) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	closed := l.closed || l.retiring
	if closed {
		return net.ErrClosed
	}
	select {
	case l.queue <- connection:
		return nil
	case <-hubStop:
		return net.ErrClosed
	default:
		return fmt.Errorf("listener generation queue is full")
	}
}

func (l *generationListener) retire() {
	l.mu.Lock()
	l.retiring = true
	l.mu.Unlock()
	l.markDrainedIfNeeded()
}

func (l *generationListener) markDrainedIfNeeded() {
	l.mu.Lock()
	drained := l.retiring && len(l.queue) == 0
	l.mu.Unlock()
	if drained {
		l.drainedOnce.Do(func() { close(l.drained) })
	}
}

func (l *generationListener) waitDrained(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("listener route drain context is nil")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.drained:
		return nil
	}
}

// WaitDrained 等待已经属于该 route 的 pending connection 全部交给 http.Server。
func (p *PreparedRoute) WaitDrained(ctx context.Context) error {
	if p == nil || p.route == nil {
		return nil
	}
	return p.route.waitDrained(ctx)
}

var _ net.Listener = (*generationListener)(nil)
