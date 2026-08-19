package execution

import (
	"context"
	"testing"
)

func TestBuildMemoryReturnsExecutor(t *testing.T) {
	resource, err := build(context.Background(), Config{
		Driver:           DriverMemory,
		RetryMaxAttempts: 2,
		RetryInitialWait: 10,
		RetryMaxWait:     50,
	}, struct{}{})
	if err != nil {
		t.Fatalf("build memory: %v", err)
	}
	if resource == nil || resource.driver != DriverMemory || resource.executor == nil {
		t.Fatalf("memory resource unexpected: %+v", resource)
	}
}

func TestBuildDisabledReturnsNilExecutor(t *testing.T) {
	resource, err := build(context.Background(), Config{Driver: DriverDisabled}, struct{}{})
	if err != nil {
		t.Fatalf("build disabled: %v", err)
	}
	if resource == nil || resource.executor != nil {
		t.Fatalf("disabled resource should have nil executor: %+v", resource)
	}
}

func TestBuildUnsupportedDriver(t *testing.T) {
	if _, err := build(context.Background(), Config{Driver: "bogus"}, struct{}{}); err == nil {
		t.Fatal("unsupported driver should error")
	}
}

func TestDefaultConfigDriverIsMemory(t *testing.T) {
	cfg := defaultConfig()
	if cfg.Driver != DriverMemory {
		t.Fatalf("default driver=%q want memory", cfg.Driver)
	}
}
