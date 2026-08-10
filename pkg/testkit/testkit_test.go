package testkit

import (
	"testing"
	"time"
)

func TestTempConfigFile(t *testing.T) {
	path := TempConfigFile(t, "server:\n  addr: :8080\n")
	if path == "" {
		t.Fatal("TempConfigFile() returned empty path")
	}
}

func TestClock(t *testing.T) {
	now := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	if !Clock(t, now).Now().Equal(now) {
		t.Fatal("test clock returned unexpected time")
	}
}
