package config

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchFiles 监听配置文件集合，并在事件稳定后通知调用方重新加载。
//
// onChange 必须快速返回；耗时的配置事务应由调用方在独立的受控循环中执行。
func WatchFiles(ctx context.Context, paths []string, debounce time.Duration, onChange func()) error {
	if ctx == nil {
		return fmt.Errorf("config watch context is nil")
	}
	if len(paths) == 0 {
		return fmt.Errorf("config watch paths are empty")
	}
	if debounce <= 0 {
		return fmt.Errorf("config watch debounce must be positive")
	}
	if onChange == nil {
		return fmt.Errorf("config watch callback is nil")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create config watcher: %w", err)
	}
	defer watcher.Close()

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
		case event, ok := <-watcher.Events:
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
			onChange()
		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("config watcher error channel closed")
			}
			return fmt.Errorf("watch config files: %w", watchErr)
		}
	}
}
