package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	permanent := os.ErrPermission
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

func TestStableFileReaderRejectsContinuouslyChangingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("value: a\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	reads := 0
	reader := stableFileReader{
		readFile: func(string) ([]byte, error) {
			reads++
			if reads%2 == 0 {
				return []byte("value: b\n"), nil
			}
			return []byte("value: a\n"), nil
		},
		stat: os.Stat, attempts: 4, interval: time.Millisecond,
	}
	if _, err := reader.read(t.Context(), path); err == nil || !strings.Contains(err.Error(), "did not stabilize") {
		t.Fatalf("read() error = %v, want unstable candidate", err)
	}
}

func TestStableFileReaderReadsAtomicallyReplacedFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.yaml")
	replacement := filepath.Join(directory, "config.next.yaml")
	if err := os.WriteFile(path, []byte("value: old\n"), 0o600); err != nil {
		t.Fatalf("write old config: %v", err)
	}
	if err := os.WriteFile(replacement, []byte("value: new\n"), 0o600); err != nil {
		t.Fatalf("write replacement config: %v", err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatalf("replace config: %v", err)
	}
	data, err := readStableFile(t.Context(), path)
	if err != nil {
		t.Fatalf("readStableFile() error = %v", err)
	}
	if string(data) != "value: new\n" {
		t.Fatalf("readStableFile() = %q", data)
	}
}

func TestStableFileReaderRejectsNonRegularTarget(t *testing.T) {
	path := t.TempDir()
	reader := stableFileReader{readFile: func(string) ([]byte, error) { return nil, nil }, stat: os.Stat, attempts: 2, interval: time.Millisecond}
	if _, err := reader.read(t.Context(), path); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("read() error = %v", err)
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
