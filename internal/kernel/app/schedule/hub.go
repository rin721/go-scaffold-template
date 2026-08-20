package schedule

import (
	"context"
	"fmt"
	"sync"

	pkgschedule "github.com/rin721/go-scaffold-template/pkg/schedule"
)

// Hub 是进程稳定的 Application Generation 调度准入切换点。
// 它不计算触发时机、不持有 Redis lease，也不拥有业务任务。
type Hub struct {
	mu      sync.RWMutex
	current Access
}

// NewHub 创建空的进程级调度准入点。
func NewHub() *Hub { return &Hub{} }

// Commit 先关闭 Hub 自己持有的旧候选准入，再开放新候选，保证同进程同 Task ID 跨代不重叠。
func (h *Hub) Commit(ctx context.Context, candidate Access) error {
	if ctx == nil {
		return fmt.Errorf("schedule hub commit context is nil")
	}
	if candidate == nil {
		return fmt.Errorf("schedule hub candidate is nil")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.current != nil {
		if err := h.current.Deactivate(ctx); err != nil {
			return fmt.Errorf("deactivate previous scheduler: %w", err)
		}
	}
	if err := candidate.Activate(ctx); err != nil {
		return fmt.Errorf("activate candidate scheduler: %w", err)
	}
	h.current = candidate
	return nil
}

// Retire 只撤销仍是当前候选的准入；旧代在新代 Commit 时已经被关闭。
func (h *Hub) Retire(ctx context.Context, candidate Access) error {
	if ctx == nil {
		return fmt.Errorf("schedule hub retire context is nil")
	}
	if candidate == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.current != candidate {
		return nil
	}
	if err := candidate.Deactivate(ctx); err != nil {
		return fmt.Errorf("deactivate current scheduler: %w", err)
	}
	h.current = nil
	return nil
}

// Diagnostics 返回当前已提交 Generation 的调度快照。
func (h *Hub) Diagnostics(ctx context.Context) (pkgschedule.Diagnostics, error) {
	h.mu.RLock()
	current := h.current
	h.mu.RUnlock()
	if current == nil {
		return pkgschedule.Diagnostics{Ready: true}, nil
	}
	return current.Diagnostics(ctx)
}

// Stop 撤销遗留 current；正常路径应由 Generation Stop 先完成 Retire。
func (h *Hub) Stop(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("schedule hub stop context is nil")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.current == nil {
		return nil
	}
	err := h.current.Deactivate(ctx)
	h.current = nil
	return err
}
