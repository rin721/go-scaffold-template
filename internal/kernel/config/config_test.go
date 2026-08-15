package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type databaseConfig struct {
	DSN         string        `mapstructure:"dsn"`
	PingTimeout time.Duration `mapstructure:"pingTimeout"`
}

func TestLoaderMergesSourcesAndRedactsSecrets(t *testing.T) {
	t.Setenv("GO_SCAFFOLD_TEST_DATABASE__PINGTIMEOUT", "9s")
	loader := New(
		MapSource("defaults", map[string]any{
			"database": map[string]any{"dsn": "postgres://user:secret@example.invalid/app", "pingTimeout": "5s"},
		}),
		EnvSource("GO_SCAFFOLD_TEST_"),
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
	t.Setenv("GO_SCAFFOLD_TEST_DATABASE__DSN", "environment.db")
	writeDatabaseDSN(t, path, "file-v1.db")
	loader := New(FileSource(path), EnvSource("GO_SCAFFOLD_TEST_"))
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

func TestEnvironmentSourceRejectsAmbiguousPathsDeterministically(t *testing.T) {
	tests := []struct {
		name     string
		entries  []string
		wantKind configPathErrorKind
		wantPath string
	}{
		{name: "scalar before object", entries: []string{"APP_DATABASE=value", "APP_DATABASE__DSN=db"}, wantKind: configPathErrorShapeConflict, wantPath: "database"},
		{name: "object before scalar", entries: []string{"APP_DATABASE__DSN=db", "APP_DATABASE=value"}, wantKind: configPathErrorShapeConflict, wantPath: "database"},
		{name: "three level ancestor first", entries: []string{"APP_CACHE__REDIS=value", "APP_CACHE__REDIS__ADDR=localhost"}, wantKind: configPathErrorShapeConflict, wantPath: "cache.redis"},
		{name: "three level descendant first", entries: []string{"APP_CACHE__REDIS__ADDR=localhost", "APP_CACHE__REDIS=value"}, wantKind: configPathErrorShapeConflict, wantPath: "cache.redis"},
		{name: "duplicate logical path", entries: []string{"APP_DATABASE__DSN=one", "APP_DATABASE__DSN=two"}, wantKind: configPathErrorDuplicate, wantPath: "database.dsn"},
		{name: "case collision", entries: []string{"APP_DATABASE__DSN=one", "APP_database__dsn=two"}, wantKind: configPathErrorCaseCollision, wantPath: "database.dsn"},
		{name: "leading empty segment", entries: []string{"APP___DATABASE=value"}, wantKind: configPathErrorEmptySegment, wantPath: ".database"},
		{name: "middle empty segment", entries: []string{"APP_DATABASE____DSN=value"}, wantKind: configPathErrorEmptySegment, wantPath: "database..dsn"},
		{name: "trailing empty segment", entries: []string{"APP_DATABASE__=value"}, wantKind: configPathErrorEmptySegment, wantPath: "database."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := loadEnvironment(t.Context(), "APP_", test.entries)
			requireConfigPathError(t, err, test.wantKind, test.wantPath)
		})
	}
}

func TestEnvironmentSourceReturnsSameFirstConflictAcrossEntryOrders(t *testing.T) {
	const secretValue = "env-secret-payload"
	orders := [][]string{
		{"APP_Z=" + secretValue, "APP_Z__CHILD=child", "APP_A=parent", "APP_A__CHILD=child"},
		{"APP_A__CHILD=child", "APP_A=parent", "APP_Z__CHILD=child", "APP_Z=" + secretValue},
		{"APP_Z__CHILD=child", "APP_A=parent", "APP_Z=" + secretValue, "APP_A__CHILD=child"},
	}
	for index, entries := range orders {
		_, err := loadEnvironment(t.Context(), "APP_", entries)
		requireConfigPathError(t, err, configPathErrorShapeConflict, "a")
		if strings.Contains(err.Error(), secretValue) {
			t.Fatalf("order %d error leaked environment value: %v", index, err)
		}
	}
}

func TestEnvironmentSourcePreservesEmptyAndEqualsValues(t *testing.T) {
	values, err := loadEnvironment(t.Context(), "APP_", []string{
		"UNRELATED=value",
		"APP_=ignored",
		"APP_DATABASE__DSN=",
		"APP_TOKEN=part=with=equals",
	})
	if err != nil {
		t.Fatalf("loadEnvironment() error = %v", err)
	}
	database, ok := values["database"].(map[string]any)
	if !ok || database["dsn"] != "" {
		t.Fatalf("database config = %#v, want explicit empty dsn", values["database"])
	}
	if values["token"] != "part=with=equals" {
		t.Fatalf("token = %#v, want full value after first equals", values["token"])
	}
	if _, exists := values[""]; exists {
		t.Fatal("prefix-only environment entry created an empty root key")
	}
}

func TestEnvironmentSourcePreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := loadEnvironment(ctx, "APP_", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("loadEnvironment(cancelled) error = %v, want context.Canceled", err)
	}
}

func TestLoaderRejectsObjectNonObjectShapeChanges(t *testing.T) {
	object := map[string]any{"child": "value"}
	tests := []struct {
		name string
		low  any
		high any
	}{
		{name: "object to scalar", low: object, high: "top-secret-value"},
		{name: "object to array", low: object, high: []any{"value"}},
		{name: "object to null", low: object, high: nil},
		{name: "scalar to object", low: "value", high: object},
		{name: "array to object", low: []any{"value"}, high: object},
		{name: "null to object", low: nil, high: object},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(
				MapSource("base", map[string]any{"setting": test.low}),
				MapSource("override", map[string]any{"setting": test.high}),
			).Load(t.Context())
			requireConfigPathError(t, err, configPathErrorShapeConflict, "setting")
			if !strings.Contains(err.Error(), "merge config source override") {
				t.Fatalf("Load() error = %v, want override source identity", err)
			}
			if strings.Contains(err.Error(), "top-secret-value") {
				t.Fatalf("Load() error leaked config value: %v", err)
			}
		})
	}
}

func TestLoaderAllowsDeterministicSameShapeOverrides(t *testing.T) {
	t.Run("recursive object merge preserves base spelling", func(t *testing.T) {
		snapshot, err := New(
			MapSource("file", map[string]any{"Database": map[string]any{"DSN": "file.db", "pool": 4}}),
			MapSource("env", map[string]any{"database": map[string]any{"dsn": "env.db", "timeout": "5s"}}),
		).Load(t.Context())
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		data := snapshot.Data()
		if _, exists := data["database"]; exists {
			t.Fatal("merge created a second differently-cased database key")
		}
		database := data["Database"].(map[string]any)
		if database["DSN"] != "env.db" || database["pool"] != 4 || database["timeout"] != "5s" {
			t.Fatalf("merged database = %#v", database)
		}
		if got := snapshot.Provenance(); !reflect.DeepEqual(got, []string{"file", "env"}) {
			t.Fatalf("Provenance() = %#v", got)
		}
	})

	for _, test := range []struct {
		name string
		low  any
		high any
	}{
		{name: "scalar to array", low: "value", high: []any{"next"}},
		{name: "array to null", low: []any{"value"}, high: nil},
		{name: "null to bool", low: nil, high: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, err := New(
				MapSource("base", map[string]any{"setting": test.low}),
				MapSource("override", map[string]any{"setting": test.high}),
			).Load(t.Context())
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			got, exists := snapshot.Value("setting")
			if !exists || !reflect.DeepEqual(got, test.high) {
				t.Fatalf("setting = %#v, want %#v", got, test.high)
			}
		})
	}
}

func TestLoaderRejectsCaseFoldSiblingCollision(t *testing.T) {
	for _, test := range []struct {
		name     string
		values   map[string]any
		wantPath string
	}{
		{name: "root", values: map[string]any{
			"Database": map[string]any{"dsn": "one"},
			"database": map[string]any{"dsn": "two"},
		}, wantPath: "database"},
		{name: "nested", values: map[string]any{
			"database": map[string]any{"DSN": "one", "dsn": "two"},
		}, wantPath: "database.dsn"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(MapSource("ambiguous", test.values)).Load(t.Context())
			requireConfigPathError(t, err, configPathErrorCaseCollision, test.wantPath)
			if !strings.Contains(err.Error(), "normalize config source ambiguous") {
				t.Fatalf("Load() error = %v, want ambiguous source identity", err)
			}
		})
	}
}

func TestLoaderReportsSameFirstConflictAcrossMapIterations(t *testing.T) {
	for attempt := 0; attempt < 100; attempt++ {
		_, err := New(
			MapSource("base", map[string]any{
				"z": map[string]any{"child": "value"},
				"a": map[string]any{"child": "value"},
			}),
			MapSource("override", map[string]any{"z": "value", "a": "value"}),
		).Load(t.Context())
		requireConfigPathError(t, err, configPathErrorShapeConflict, "a")
	}
}

func requireConfigPathError(t *testing.T, err error, wantKind configPathErrorKind, wantPath string) {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil")
	}
	var pathErr *configPathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("error = %v, want configPathError", err)
	}
	if pathErr.kind != wantKind || pathErr.path != wantPath {
		t.Fatalf("configPathError = {%q %q}, want {%q %q}", pathErr.kind, pathErr.path, wantKind, wantPath)
	}
}

func writeDatabaseDSN(t *testing.T, path, dsn string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("database:\n  dsn: "+dsn+"\n"), 0o600); err != nil {
		t.Fatalf("write database config: %v", err)
	}
}
