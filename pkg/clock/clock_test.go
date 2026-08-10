package clock

import (
	"testing"
	"time"
)

func TestFixedClockAndFormat(t *testing.T) {
	now := time.Date(2026, 8, 9, 1, 2, 3, 4_000_000, time.FixedZone("UTC+8", 8*60*60))
	clk := Fixed(now)
	if !clk.Now().Equal(now) {
		t.Fatal("fixed clock returned a different time")
	}
	if got := RFC3339Millis(now); got != "2026-08-08T17:02:03.004Z" {
		t.Fatalf("RFC3339Millis() = %q", got)
	}
}
