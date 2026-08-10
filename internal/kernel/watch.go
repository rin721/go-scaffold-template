package kernel

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

// Watch 持续监听 FileSource，并把每次失败交给调用方后继续等待新配置。
func (k *Kernel) Watch(ctx context.Context, onReloadError func(error)) error {
	if ctx == nil {
		return ErrNilContext
	}
	if onReloadError == nil {
		return fmt.Errorf("kernel reload error callback is nil")
	}
	k.mu.Lock()
	state := k.state
	k.mu.Unlock()
	if state != kernelRunning {
		if state == kernelStopped {
			return ErrStopped
		}
		return ErrNotRunning
	}
	paths := k.loader.FilePaths()
	if len(paths) == 0 {
		return fmt.Errorf("kernel has no file config source")
	}

	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	changes := make(chan struct{}, 1)
	watchDone := make(chan error, 1)
	go func() {
		watchDone <- config.WatchFiles(watchCtx, paths, k.options.Debounce, func() {
			select {
			case changes <- struct{}{}:
			default:
			}
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
			if _, err := k.Reload(ctx); err != nil {
				if ctx.Err() != nil {
					continue
				}
				onReloadError(err)
			}
		}
	}
}
