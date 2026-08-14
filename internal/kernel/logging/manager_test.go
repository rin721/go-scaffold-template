package logging

import (
	"sync"
	"testing"
	"time"

	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
)

func TestManagerRequiresBaseline(t *testing.T) {
	if manager, err := New(nil); err == nil || manager != nil {
		t.Fatalf("New(nil) = %#v, %v; want nil, error", manager, err)
	}
}

func TestManagerLoggerViewDoesNotExposeReplacementControl(t *testing.T) {
	manager, err := New(pkglogger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	view := manager.Logger()
	if view == nil {
		t.Fatal("Logger() view is nil")
	}
	if _, exposed := view.(interface {
		Replace(pkglogger.Logger)
		Restore()
	}); exposed {
		t.Fatal("Logger() view exposes replacement control")
	}
	if view != manager.Logger() {
		t.Fatal("Logger() did not return a stable view")
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

func TestManagerReplacementWaitsForInFlightWrite(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	baseline := &blockingLogger{entered: entered, release: release}
	configured := &recordingLogger{}
	manager, err := New(baseline)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	writeDone := make(chan struct{})
	go func() {
		manager.Info("blocking")
		close(writeDone)
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("logger write did not enter baseline")
	}
	replaceDone := make(chan struct{})
	go func() {
		manager.Replace(configured)
		close(replaceDone)
	}()
	select {
	case <-replaceDone:
		t.Fatal("Replace() completed while baseline write was in flight")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-writeDone:
	case <-time.After(time.Second):
		t.Fatal("baseline write did not complete")
	}
	select {
	case <-replaceDone:
	case <-time.After(time.Second):
		t.Fatal("Replace() did not complete after baseline write")
	}
	manager.Info("configured")
	if got := configured.messages(); !equalStrings(got, []string{"configured"}) {
		t.Fatalf("configured messages = %#v", got)
	}
}

type blockingLogger struct {
	entered chan<- struct{}
	release <-chan struct{}
	once    sync.Once
}

func (l *blockingLogger) Debug(string, ...pkglogger.Field) { l.block() }
func (l *blockingLogger) Info(string, ...pkglogger.Field)  { l.block() }
func (l *blockingLogger) Warn(string, ...pkglogger.Field)  { l.block() }
func (l *blockingLogger) Error(string, ...pkglogger.Field) { l.block() }
func (l *blockingLogger) With(...pkglogger.Field) pkglogger.Logger {
	return l
}

func (l *blockingLogger) block() {
	l.once.Do(func() { close(l.entered) })
	<-l.release
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
