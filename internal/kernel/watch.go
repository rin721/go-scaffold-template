package kernel

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
)

// Watch 持续监听 FileSource，并由唯一 Coordinator 加载每个候选。
func (c *Coordinator) Watch(ctx context.Context, onReloadError func(error)) error {
	return c.watch(ctx, onReloadError, nil)
}

func (c *Coordinator) watch(ctx context.Context, onReloadError func(error), ready chan<- struct{}) error {
	if ctx == nil {
		return ErrNilContext
	}
	if onReloadError == nil {
		return fmt.Errorf("kernel reload error callback is nil")
	}
	c.runtime.mu.Lock()
	state := c.runtime.state
	c.runtime.mu.Unlock()
	if state != kernelRunning {
		if state == kernelStopped {
			return ErrStopped
		}
		return ErrNotRunning
	}
	paths := c.loader.FilePaths()
	if len(paths) == 0 {
		return fmt.Errorf("kernel has no file config source")
	}

	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	changes := make(chan struct{}, 1)
	notifyChange := func() {
		select {
		case changes <- struct{}{}:
		default:
		}
	}
	notifyReady := func() {
		notifyChange()
		if ready != nil {
			close(ready)
			ready = nil
		}
	}
	watchDone := make(chan error, 1)
	go func() {
		watchDone <- config.WatchFiles(watchCtx, paths, c.runtime.options.Debounce, config.WatchCallbacks{
			OnReady:  notifyReady,
			OnChange: notifyChange,
		})
	}()

	for {
		select {
		case <-ctx.Done():
			cancel()
			if err := <-watchDone; err != nil {
				return err
			}
			return nil
		case err := <-watchDone:
			if err == nil {
				return fmt.Errorf("kernel config watcher stopped unexpectedly")
			}
			return err
		case <-changes:
			if _, err := c.Reload(ctx); err != nil {
				if ctx.Err() != nil {
					continue
				}
				onReloadError(err)
			}
		}
	}
}
