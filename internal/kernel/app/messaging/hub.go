package messaging

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rin721/go-scaffold-template/pkg/health"
	pkgmessaging "github.com/rin721/go-scaffold-template/pkg/messaging"
)

// Hub 是进程稳定的 Application Generation 消息准入切换点。
type Hub struct {
	mu      sync.RWMutex
	current Control
}

// NewHub 创建空的进程级消息准入点。
func NewHub() *Hub { return &Hub{} }

// Commit 先关闭旧代准入再开放候选，确保同 Consumer ID 在单进程内不跨代重叠。
func (h *Hub) Commit(ctx context.Context, candidate Control) error {
	if ctx == nil {
		return fmt.Errorf("messaging hub commit context is nil")
	}
	if candidate == nil {
		return fmt.Errorf("messaging hub candidate is nil")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	previous := h.current
	if previous != nil {
		if err := previous.Deactivate(ctx); err != nil {
			joined := fmt.Errorf("deactivate previous messaging generation: %w", err)
			if restoreErr := previous.Activate(context.WithoutCancel(ctx)); restoreErr != nil {
				joined = errors.Join(joined, fmt.Errorf("restore previous messaging generation: %w", restoreErr))
			}
			return joined
		}
	}
	if err := candidate.Activate(ctx); err != nil {
		joined := fmt.Errorf("activate candidate messaging generation: %w", err)
		if previous != nil {
			if restoreErr := previous.Activate(context.WithoutCancel(ctx)); restoreErr != nil {
				joined = errors.Join(joined, fmt.Errorf("restore previous messaging generation: %w", restoreErr))
			}
		}
		return joined
	}
	h.current = candidate
	return nil
}

// Retire 只撤销仍为当前候选的消息准入。
func (h *Hub) Retire(ctx context.Context, candidate Control) error {
	if ctx == nil {
		return fmt.Errorf("messaging hub retire context is nil")
	}
	if candidate == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.current != candidate {
		return nil
	}
	err := candidate.Deactivate(ctx)
	h.current = nil
	return err
}

// Diagnostics 返回当前已提交 Generation 的消息快照。
func (h *Hub) Diagnostics(ctx context.Context) (pkgmessaging.Diagnostics, error) {
	h.mu.RLock()
	current := h.current
	h.mu.RUnlock()
	if current == nil {
		return pkgmessaging.Diagnostics{}, nil
	}
	return current.Diagnostics(ctx)
}

// Health 返回当前已提交 Generation 的聚合健康状态。
func (h *Hub) Health(ctx context.Context) (health.Result, error) {
	h.mu.RLock()
	current := h.current
	h.mu.RUnlock()
	if current == nil {
		return health.Result{Name: string(ID), Kind: health.KindReadiness, Status: health.StatusPass, Message: "messaging inactive"}, nil
	}
	return current.Health(ctx)
}

// Stop 撤销遗留 current；正常路径由 Generation Stop 先完成 Retire。
func (h *Hub) Stop(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.current == nil {
		return nil
	}
	err := h.current.Deactivate(ctx)
	h.current = nil
	return err
}
