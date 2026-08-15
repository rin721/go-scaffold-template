package config

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type watchBackend interface {
	Add(string) error
	Events() <-chan fsnotify.Event
	Errors() <-chan error
	Close() error
}

type fsnotifyBackend struct{ watcher *fsnotify.Watcher }

func (b fsnotifyBackend) Add(path string) error         { return b.watcher.Add(path) }
func (b fsnotifyBackend) Events() <-chan fsnotify.Event { return b.watcher.Events }
func (b fsnotifyBackend) Errors() <-chan error          { return b.watcher.Errors }
func (b fsnotifyBackend) Close() error                  { return b.watcher.Close() }

type watchFactory func() (watchBackend, error)

func newWatchBackend() (watchBackend, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return fsnotifyBackend{watcher: watcher}, nil
}

// WatchCallbacks 定义 watcher 注册完成和文件变化后的快速通知。
//
// 两个回调都不得执行配置事务；Kernel 会在自身串行循环中重新加载候选。
type WatchCallbacks struct {
	OnReady  func()
	OnChange func()
}

// WatchFiles 监听配置文件集合，并在目录注册完成和事件稳定后通知调用方。
func WatchFiles(ctx context.Context, paths []string, debounce time.Duration, callbacks WatchCallbacks) error {
	return watchFiles(ctx, paths, debounce, callbacks, newWatchBackend)
}

func watchFiles(ctx context.Context, paths []string, debounce time.Duration, callbacks WatchCallbacks, create watchFactory) (resultErr error) {
	if ctx == nil {
		return fmt.Errorf("config watch context is nil")
	}
	if len(paths) == 0 {
		return fmt.Errorf("config watch paths are empty")
	}
	if debounce <= 0 {
		return fmt.Errorf("config watch debounce must be positive")
	}
	if callbacks.OnReady == nil {
		return fmt.Errorf("config watch ready callback is nil")
	}
	if callbacks.OnChange == nil {
		return fmt.Errorf("config watch callback is nil")
	}

	watcher, err := create()
	if err != nil {
		return fmt.Errorf("create config watcher: %w", err)
	}
	defer func() {
		if closeErr := watcher.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close config watcher: %w", closeErr))
		}
	}()

	targets := make(map[string]struct{}, len(paths))
	directories := make(map[string]struct{})
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve config watch path: %w", err)
		}
		target := filepath.Clean(absolute)
		targets[target] = struct{}{}
		directories[filepath.Dir(target)] = struct{}{}
	}
	for directory := range directories {
		if err := watcher.Add(directory); err != nil {
			return fmt.Errorf("watch config directory %s: %w", directory, err)
		}
	}
	// 注册完成后立即要求调用方重新加载一次，封闭初始 Load 与 watcher ready
	// 之间的变化窗口。回调只投递通知，不在 fsnotify goroutine 中应用配置。
	callbacks.OnReady()

	var timer *time.Timer
	var timerEvents <-chan time.Time
	stopTimer := func() {
		if timer == nil {
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
	defer stopTimer()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.Events():
			if !ok {
				return fmt.Errorf("config watcher event channel closed")
			}
			absolute, err := filepath.Abs(event.Name)
			if err != nil {
				continue
			}
			if _, tracked := targets[filepath.Clean(absolute)]; !tracked {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}
			if timer == nil {
				timer = time.NewTimer(debounce)
			} else {
				stopTimer()
				timer.Reset(debounce)
			}
			timerEvents = timer.C
		case <-timerEvents:
			timerEvents = nil
			callbacks.OnChange()
		case watchErr, ok := <-watcher.Errors():
			if !ok {
				return fmt.Errorf("config watcher error channel closed")
			}
			return fmt.Errorf("watch config files: %w", watchErr)
		}
	}
}
