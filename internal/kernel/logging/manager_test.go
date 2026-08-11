package logging

import (
	"sync"
	"testing"

	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

func TestManagerRequiresBaseline(t *testing.T) {
	if manager, err := New(nil); err == nil || manager != nil {
		t.Fatalf("New(nil) = %#v, %v; want nil, error", manager, err)
	}
}

func TestManagerBoundLoggerFollowsReplacementAndRestore(t *testing.T) {
	baseline := &recordingLogger{}
	configured := &recordingLogger{}
	manager, err := New(baseline)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	bound := manager.With(pkglogger.String("component", "kernel")).With(pkglogger.String("phase", "reload"))

	bound.Info("baseline")
	manager.Replace(configured)
	bound.Info("configured")
	manager.Restore()
	bound.Info("restored")

	if got := baseline.messages(); !equalStrings(got, []string{"baseline", "restored"}) {
		t.Fatalf("baseline messages = %#v", got)
	}
	if got := configured.messages(); !equalStrings(got, []string{"configured"}) {
		t.Fatalf("configured messages = %#v", got)
	}
	if got := configured.fieldCounts(); len(got) != 1 || got[0] != 2 {
		t.Fatalf("configured field counts = %#v, want [2]", got)
	}
}

func TestManagerConcurrentWriteAndReplacement(t *testing.T) {
	baseline := &recordingLogger{}
	configured := &recordingLogger{}
	manager, err := New(baseline)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	var group sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := 0; index < 200; index++ {
				manager.Info("concurrent")
			}
		}()
	}
	for index := 0; index < 200; index++ {
		manager.Replace(configured)
		manager.Restore()
	}
	group.Wait()
}

type recordingLogger struct {
	mu      sync.Mutex
	entries []recordingEntry
	bound   int
}

type recordingEntry struct {
	message string
	fields  int
}

func (l *recordingLogger) Debug(message string, fields ...pkglogger.Field) {
	l.add(message, len(fields))
}
func (l *recordingLogger) Info(message string, fields ...pkglogger.Field) {
	l.add(message, len(fields))
}
func (l *recordingLogger) Warn(message string, fields ...pkglogger.Field) {
	l.add(message, len(fields))
}
func (l *recordingLogger) Error(message string, fields ...pkglogger.Field) {
	l.add(message, len(fields))
}

func (l *recordingLogger) With(fields ...pkglogger.Field) pkglogger.Logger {
	return &recordingLoggerView{target: l, fields: l.bound + len(fields)}
}

func (l *recordingLogger) add(message string, fields int) {
	l.mu.Lock()
	l.entries = append(l.entries, recordingEntry{message: message, fields: l.bound + fields})
	l.mu.Unlock()
}

func (l *recordingLogger) messages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]string, 0, len(l.entries))
	for _, entry := range l.entries {
		result = append(result, entry.message)
	}
	return result
}

func (l *recordingLogger) fieldCounts() []int {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]int, 0, len(l.entries))
	for _, entry := range l.entries {
		result = append(result, entry.fields)
	}
	return result
}

type recordingLoggerView struct {
	target *recordingLogger
	fields int
}

func (l *recordingLoggerView) Debug(message string, fields ...pkglogger.Field) {
	l.target.add(message, l.fields+len(fields))
}
func (l *recordingLoggerView) Info(message string, fields ...pkglogger.Field) {
	l.target.add(message, l.fields+len(fields))
}
func (l *recordingLoggerView) Warn(message string, fields ...pkglogger.Field) {
	l.target.add(message, l.fields+len(fields))
}
func (l *recordingLoggerView) Error(message string, fields ...pkglogger.Field) {
	l.target.add(message, l.fields+len(fields))
}
func (l *recordingLoggerView) With(fields ...pkglogger.Field) pkglogger.Logger {
	return &recordingLoggerView{target: l.target, fields: l.fields + len(fields)}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
