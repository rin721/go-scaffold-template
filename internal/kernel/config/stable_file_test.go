package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStableFileReaderRetriesTransientFailureAndRequiresTwoEqualSamples(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("value: stable\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var mu sync.Mutex
	reads := 0
	reader := stableFileReader{
		readFile: func(path string) ([]byte, error) {
			mu.Lock()
			defer mu.Unlock()
			reads++
			if reads == 1 {
				return nil, os.ErrNotExist
			}
			return os.ReadFile(path)
		},
		stat: os.Stat, attempts: 4, interval: time.Millisecond,
	}
	data, err := reader.read(t.Context(), path)
	if err != nil {
		t.Fatalf("read() error = %v", err)
	}
	if string(data) != "value: stable\n" || reads != 3 {
		t.Fatalf("read() = %q after %d reads", data, reads)
	}
}

func TestStableFileReaderRejectsPermanentFailureWithoutRetry(t *testing.T) {
	permanent := errors.New("permanent read failure")
	reads := 0
	reader := stableFileReader{
		readFile: func(string) ([]byte, error) { reads++; return nil, permanent },
		stat:     os.Stat, attempts: 4, interval: time.Millisecond,
	}
	_, err := reader.read(t.Context(), "config.yaml")
	if !errors.Is(err, permanent) || reads != 1 {
		t.Fatalf("read() error = %v after %d reads", err, reads)
	}
}

func TestStableFileReaderHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	reader := stableFileReader{readFile: os.ReadFile, stat: os.Stat, attempts: 2, interval: time.Second}
	_, err := reader.read(ctx, "config.yaml")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("read() error = %v, want canceled", err)
	}
}
