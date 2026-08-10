package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchFilesDebouncesChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("value: one\n"), 0o600); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	changes := make(chan struct{}, 2)
	errors := make(chan error, 1)
	go func() {
		errors <- WatchFiles(ctx, []string{path}, 50*time.Millisecond, func() {
			changes <- struct{}{}
		})
	}()

	// 等待 watcher 完成目录注册，避免把测试夹具写入误判为实现错误。
	time.Sleep(100 * time.Millisecond)
	for _, value := range []string{"two", "three", "four"} {
		if err := os.WriteFile(path, []byte("value: "+value+"\n"), 0o600); err != nil {
			t.Fatalf("rewrite config: %v", err)
		}
	}

	select {
	case <-changes:
	case err := <-errors:
		t.Fatalf("WatchFiles() returned early: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for debounced change")
	}
	select {
	case <-changes:
		t.Fatal("WatchFiles() emitted more than one debounced change")
	case <-time.After(150 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-errors:
		if err != nil {
			t.Fatalf("WatchFiles() shutdown error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out stopping config watcher")
	}
}
