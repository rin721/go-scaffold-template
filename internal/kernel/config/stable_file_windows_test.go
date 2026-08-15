//go:build windows

package config

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestStableFileReaderRetriesWindowsSharingViolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("value: stable\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	reads := 0
	reader := stableFileReader{
		readFile: func(path string) ([]byte, error) {
			reads++
			if reads == 1 {
				return nil, syscall.Errno(32)
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
