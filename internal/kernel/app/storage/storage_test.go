package storage

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	kernelconfig "github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgstorage "github.com/rin721/go-scaffold2/pkg/storage"
)

func TestDefinitionContributesStableStorageAccess(t *testing.T) {
	definition, err := Definition()
	if err != nil {
		t.Fatalf("Definition() error = %v", err)
	}
	plan := app.NewPlan()
	added, err := app.Add(plan, definition)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if added.Output == nil {
		t.Fatal("Storage Access is nil")
	}
	frozen, err := plan.Freeze()
	if err != nil {
		t.Fatalf("Freeze() error = %v", err)
	}
	bindings := frozen.Configurations()
	if len(bindings) != 1 || bindings[0].CapabilityID != string(ID) || bindings[0].ConfigPath != ConfigPath {
		t.Fatalf("Configurations() = %#v", bindings)
	}
	components := frozen.Components()
	if len(components) != 1 || components[0].Policy() != app.KernelInstanceSwap {
		t.Fatalf("Components() = %#v", components)
	}
	clientType := reflect.TypeOf((*Client)(nil)).Elem()
	if _, exposed := clientType.MethodByName("Close"); exposed {
		t.Fatal("Storage Client exposes shared Close ownership")
	}
}

func TestDisabledAccessReturnsTypedError(t *testing.T) {
	manager, err := pkgstorage.NewManager(t.Context(), &pkgstorage.Config{Driver: pkgstorage.DriverDisabled})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	storageAccess, err := newAccess(fakeLease{current: manager})
	if err != nil {
		t.Fatalf("newAccess() error = %v", err)
	}
	err = storageAccess.Use(t.Context(), RoutePrimary, func(Client) error { return nil })
	if !errors.Is(err, pkgstorage.ErrDisabled) {
		t.Fatalf("Use() error = %v, want ErrDisabled", err)
	}
}

func TestAccessRoutesAndInvalidatesBorrowedClient(t *testing.T) {
	manager, err := pkgstorage.NewManager(t.Context(), &pkgstorage.Config{
		Driver: pkgstorage.DriverLocal,
		Local:  pkgstorage.LocalConfig{BasePath: t.TempDir()},
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	storageAccess, err := newAccess(fakeLease{current: manager})
	if err != nil {
		t.Fatalf("newAccess() error = %v", err)
	}
	var escaped Client
	err = storageAccess.Use(t.Context(), RoutePrimary, func(client Client) error {
		escaped = client
		if _, exposed := client.(pkgstorage.StorageClient); exposed {
			t.Fatal("borrowed Client exposes StorageClient Close ownership")
		}
		if err := client.Put(t.Context(), "objects/value.txt", []byte("value"), pkgstorage.PutOptions{}); err != nil {
			return err
		}
		data, _, err := client.Get(t.Context(), "objects/value.txt")
		if err != nil {
			return err
		}
		if string(data) != "value" {
			t.Fatalf("Get() data = %q", data)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Use() error = %v", err)
	}
	if _, err := escaped.Exists(context.Background(), "objects/value.txt"); !errors.Is(err, pkgstorage.ErrClientUnavailable) {
		t.Fatalf("escaped Exists() error = %v, want ErrClientUnavailable", err)
	}
	if err := storageAccess.Use(t.Context(), RouteObject, func(Client) error { return nil }); !errors.Is(err, pkgstorage.ErrClientUnavailable) {
		t.Fatalf("Use(object) error = %v, want ErrClientUnavailable", err)
	}
	if err := storageAccess.Use(t.Context(), Route("unknown"), func(Client) error { return nil }); !errors.Is(err, pkgstorage.ErrInvalidRoute) {
		t.Fatalf("Use(unknown) error = %v, want ErrInvalidRoute", err)
	}
}

func TestBuildAndReadyLocalManager(t *testing.T) {
	cfg := defaultConfig()
	cfg.Local.BasePath = t.TempDir()
	manager, err := build(t.Context(), cfg, struct{}{})
	if err != nil {
		t.Fatalf("build() error = %v", err)
	}
	t.Cleanup(func() { _ = stop(context.Background(), manager) })
	if err := ready(t.Context(), manager); err != nil {
		t.Fatalf("ready() error = %v", err)
	}
}

func TestDecodeRejectsInvalidRemoteWithoutLeakingSecret(t *testing.T) {
	loader := kernelconfig.New(kernelconfig.MapSource("test", map[string]any{
		"storage": map[string]any{
			"driver": "s3",
			"s3": map[string]any{
				"accessKeyId":     "access",
				"secretAccessKey": "do-not-print",
			},
		},
	}))
	snapshot, err := loader.Load(t.Context())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	_, err = decode(snapshot)
	if err == nil {
		t.Fatal("decode() error = nil")
	}
	if strings.Contains(err.Error(), "do-not-print") {
		t.Fatalf("decode() leaked secret: %v", err)
	}
}

type fakeLease struct{ current *pkgstorage.StorageManager }

func (f fakeLease) Use(ctx context.Context, use func(*pkgstorage.StorageManager) error) error {
	if ctx == nil {
		return app.ErrNilContext
	}
	return use(f.current)
}
