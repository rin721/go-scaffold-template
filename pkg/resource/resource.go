package resource

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

// Handle 表示一个有明确所有权的资源。
type Handle struct {
	Name   string
	Shared bool
	Close  func() error
}

// Registry 管理资源释放顺序。
type Registry struct {
	mu      sync.Mutex
	handles []Handle
	closed  bool
}

// NewRegistry 创建资源注册表。
func NewRegistry() *Registry {
	return &Registry{}
}

// Add 注册资源。共享资源默认禁止由本 Registry 关闭。
func (r *Registry) Add(handle Handle) error {
	if handle.Name == "" {
		return fmt.Errorf("resource name is required")
	}
	if handle.Close == nil {
		return fmt.Errorf("resource %s close function is nil", handle.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return fmt.Errorf("resource registry already closed")
	}
	r.handles = append(r.handles, handle)
	return nil
}

// AddCloser 注册 io.Closer。
func (r *Registry) AddCloser(name string, closer io.Closer) error {
	if closer == nil {
		return fmt.Errorf("resource %s closer is nil", name)
	}
	return r.Add(Handle{Name: name, Close: closer.Close})
}

// Close 按反向注册顺序关闭资源，并保留所有关闭错误。
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	var joined error
	for index := len(r.handles) - 1; index >= 0; index-- {
		handle := r.handles[index]
		if handle.Shared {
			continue
		}
		if err := handle.Close(); err != nil {
			joined = errors.Join(joined, fmt.Errorf("close resource %s: %w", handle.Name, err))
		}
	}
	return joined
}
