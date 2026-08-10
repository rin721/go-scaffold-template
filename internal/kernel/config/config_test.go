package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type databaseConfig struct {
	DSN         string        `mapstructure:"dsn"`
	PingTimeout time.Duration `mapstructure:"pingTimeout"`
}

func TestLoaderMergesSourcesAndRedactsSecrets(t *testing.T) {
	t.Setenv("APP_DATABASE__PINGTIMEOUT", "9s")
	loader := New(
		MapSource("defaults", map[string]any{
			"database": map[string]any{"dsn": "postgres://user:secret@example.invalid/app", "pingTimeout": "5s"},
		}),
		EnvSource("APP_"),
	)
	snapshot, err := loader.Load(t.Context())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	var decoded databaseConfig
	if err := snapshot.DecodeSection("database", &decoded); err != nil {
		t.Fatalf("DecodeSection() error = %v", err)
	}
	if decoded.PingTimeout != 9*time.Second {
		t.Fatalf("PingTimeout = %v, want 9s", decoded.PingTimeout)
	}
	redacted := snapshot.Redacted()["database"].(map[string]any)
	if redacted["dsn"] == "postgres://user:secret@example.invalid/app" {
		t.Fatal("Redacted() leaked database dsn")
	}
}

func TestFileSourceLoadsYAMLAndReportsPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("database:\n  pingTimeout: 3s\n"), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	loader := New(FileSource(path), FileSource(path))
	snapshot, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load(file) error = %v", err)
	}
	var decoded databaseConfig
	if err := snapshot.DecodeSection("database", &decoded); err != nil {
		t.Fatalf("DecodeSection() error = %v", err)
	}
	if decoded.PingTimeout != 3*time.Second {
		t.Fatalf("PingTimeout = %v, want 3s", decoded.PingTimeout)
	}
	if paths := loader.FilePaths(); len(paths) != 1 || paths[0] != filepath.Clean(path) {
		t.Fatalf("FilePaths() = %#v, want %q", paths, filepath.Clean(path))
	}
}

func TestSnapshotSectionDigestTracksOnlySection(t *testing.T) {
	first, err := New(MapSource("first", map[string]any{
		"database": map[string]any{"dsn": "one"},
		"server":   map[string]any{"addr": ":8080"},
	})).Load(t.Context())
	if err != nil {
		t.Fatalf("Load(first) error = %v", err)
	}
	second, err := New(MapSource("second", map[string]any{
		"database": map[string]any{"dsn": "one"},
		"server":   map[string]any{"addr": ":9090"},
	})).Load(t.Context())
	if err != nil {
		t.Fatalf("Load(second) error = %v", err)
	}
	firstDigest, err := first.SectionDigest("database")
	if err != nil {
		t.Fatalf("SectionDigest(first) error = %v", err)
	}
	secondDigest, err := second.SectionDigest("database")
	if err != nil {
		t.Fatalf("SectionDigest(second) error = %v", err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("database digest changed with unrelated section: %q != %q", firstDigest, secondDigest)
	}
}
