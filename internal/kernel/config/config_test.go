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
	loader := New(FileSource(path))
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
	if _, err := New(FileSource(path), FileSource(path)).Load(t.Context()); err == nil {
		t.Fatal("Load(duplicate source names) error = nil")
	}
}

func TestFileSourceRejectsDuplicateJSONAndYAMLKeys(t *testing.T) {
	for _, test := range []struct {
		name    string
		ext     string
		content string
	}{
		{name: "json", ext: ".json", content: `{"database":{"dsn":"one","dsn":"two"}}`},
		{name: "yaml", ext: ".yaml", content: "database:\n  dsn: one\n  dsn: two\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config"+test.ext)
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatalf("write duplicate fixture: %v", err)
			}
			if _, err := New(FileSource(path)).Load(t.Context()); err == nil {
				t.Fatal("Load(duplicate keys) error = nil")
			}
		})
	}
}

func TestSnapshotDecodeRejectsUnknownAndCrossTypeValues(t *testing.T) {
	snapshot, err := New(MapSource("strict", map[string]any{
		"database": map[string]any{"dsn": "db", "unknown": true},
	})).Load(t.Context())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	var decoded databaseConfig
	if err := snapshot.DecodeSection("database", &decoded); err == nil {
		t.Fatal("DecodeSection(unknown field) error = nil")
	}

	snapshot, err = New(MapSource("strict", map[string]any{
		"database": map[string]any{"dsn": false},
	})).Load(t.Context())
	if err != nil {
		t.Fatalf("Load(cross type) error = %v", err)
	}
	if err := snapshot.DecodeSection("database", &decoded); err == nil {
		t.Fatal("DecodeSection(bool to string) error = nil")
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

func TestEnvironmentOverrideKeepsEffectiveSectionDigestStableAcrossFileChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("APP_DATABASE__DSN", "environment.db")
	writeDatabaseDSN(t, path, "file-v1.db")
	loader := New(FileSource(path), EnvSource("APP_"))
	first, err := loader.Load(t.Context())
	if err != nil {
		t.Fatalf("Load(first) error = %v", err)
	}

	writeDatabaseDSN(t, path, "file-v2.db")
	second, err := loader.Load(t.Context())
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
		t.Fatalf("effective database digest changed under env override: %q != %q", firstDigest, secondDigest)
	}
	var decoded databaseConfig
	if err := second.DecodeSection("database", &decoded); err != nil {
		t.Fatalf("DecodeSection() error = %v", err)
	}
	if decoded.DSN != "environment.db" {
		t.Fatalf("DSN = %q, want environment override", decoded.DSN)
	}
}

func writeDatabaseDSN(t *testing.T, path, dsn string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("database:\n  dsn: "+dsn+"\n"), 0o600); err != nil {
		t.Fatalf("write database config: %v", err)
	}
}
