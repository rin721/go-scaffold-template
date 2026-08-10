package logger

import (
	"context"
	"testing"
)

func TestTestLoggerRecordsEntries(t *testing.T) {
	log := NewTestLogger()
	log.Info("hello", AuditField("action", "create"))
	if len(log.Entries()) != 1 {
		t.Fatalf("entries = %d", len(log.Entries()))
	}
}

func TestWithContextAddsFields(t *testing.T) {
	log := WithContext(context.Background(), NewTestLogger(), String("correlation_id", "req"))
	log.Info("ok")
	if len(log.(*TestLogger).Entries()) != 2 {
		t.Fatal("context logger did not record entry")
	}
}
