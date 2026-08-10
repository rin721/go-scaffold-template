package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNewManagerDisabled(t *testing.T) {
	manager, err := NewManager(context.Background(), &Config{Driver: DriverDisabled})
	if err != nil {
		t.Fatalf("NewManager(disabled) error = %v", err)
	}
	if manager.Primary() != nil || manager.Local != nil || manager.Object != nil {
		t.Fatalf("disabled manager should not create clients: %#v", manager)
	}
}

func TestLocalStorageClientExercise(t *testing.T) {
	manager, err := NewManager(context.Background(), &Config{
		Driver: DriverLocal,
		Local: LocalConfig{
			BasePath: t.TempDir(),
		},
	})
	if err != nil {
		t.Fatalf("NewManager(local) error = %v", err)
	}
	defer manager.Close()
	if manager.Local == nil || manager.Primary() == nil {
		t.Fatalf("local manager did not create local client: %#v", manager)
	}
	if err := ExerciseClient(context.Background(), manager.Local); err != nil {
		t.Fatalf("ExerciseClient(local) error = %v", err)
	}
}

func TestRemoteStorageRequiresConnectionFields(t *testing.T) {
	_, err := NewManager(context.Background(), &Config{Driver: DriverS3})
	if err == nil {
		t.Fatal("NewManager(r2 without object config) error = nil")
	}
	if !strings.Contains(err.Error(), "endpoint") {
		t.Fatalf("NewManager(r2) error = %v, want endpoint hint", err)
	}
}

func TestLocalRemoteModeCreatesLocalBeforeValidatingRemote(t *testing.T) {
	manager, err := NewManager(context.Background(), &Config{
		Driver: DriverLocalMinIO,
		Local: LocalConfig{
			BasePath: t.TempDir(),
		},
		MinIO: ObjectConfig{
			Endpoint:        "http://127.0.0.1:9000",
			Bucket:          "bucket",
			AccessKeyID:     "access",
			SecretAccessKey: "secret",
		},
	})
	if err != nil {
		t.Fatalf("NewManager(local+minio with config) error = %v", err)
	}
	defer manager.Close()
	if manager.Local == nil || manager.Object == nil {
		t.Fatalf("local+minio manager did not create both clients: %#v", manager)
	}
}

func TestStorageManagerCloseJoinsLocalAndObjectErrors(t *testing.T) {
	localErr := errors.New("local close failed")
	objectErr := errors.New("object close failed")
	localClosed := false
	objectClosed := false
	manager := &StorageManager{
		Local:  closeErrorStorageClient{err: localErr, closed: &localClosed},
		Object: closeErrorStorageClient{err: objectErr, closed: &objectClosed},
	}

	err := manager.Close()
	if err == nil {
		t.Fatal("Close() error = nil, want joined close errors")
	}
	if !localClosed || !objectClosed {
		t.Fatalf("Close() closed local=%v object=%v, want both true", localClosed, objectClosed)
	}
	if !errors.Is(err, localErr) {
		t.Fatalf("Close() error = %v, want local close error", err)
	}
	if !errors.Is(err, objectErr) {
		t.Fatalf("Close() error = %v, want object close error", err)
	}
	if !strings.Contains(err.Error(), "local storage client close") {
		t.Fatalf("Close() error = %v, want local close context", err)
	}
	if !strings.Contains(err.Error(), "object storage client close") {
		t.Fatalf("Close() error = %v, want object close context", err)
	}
}

type closeErrorStorageClient struct {
	StorageClient
	err    error
	closed *bool
}

func (c closeErrorStorageClient) Close() error {
	if c.closed != nil {
		*c.closed = true
	}
	return c.err
}
