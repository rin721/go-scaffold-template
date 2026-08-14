package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDefaultValueModelPreservesOrderAndEncodesAllKinds(t *testing.T) {
	number, err := Number("12.50")
	if err != nil {
		t.Fatalf("Number() error = %v", err)
	}
	object := Object{
		FieldOf("name", String("demo")),
		FieldOf("enabled", Bool(true)),
		FieldOf("count", number),
		FieldOf("timeout", Duration(5*time.Second)),
		FieldOf("optional", Null()),
		FieldOf("nested", ObjectValue(Object{FieldOf("value", String("x"))})),
		FieldOf("items", List(String("first"), Bool(false))),
	}

	jsonPayload, err := encodeDefaultDocument(object, FormatJSON)
	if err != nil {
		t.Fatalf("encode JSON error = %v", err)
	}
	wantJSON := "{\n" +
		"  \"name\": \"demo\",\n" +
		"  \"enabled\": true,\n" +
		"  \"count\": 12.50,\n" +
		"  \"timeout\": \"5s\",\n" +
		"  \"optional\": null,\n" +
		"  \"nested\": {\n" +
		"    \"value\": \"x\"\n" +
		"  },\n" +
		"  \"items\": [\n" +
		"    \"first\",\n" +
		"    false\n" +
		"  ]\n" +
		"}\n"
	if string(jsonPayload) != wantJSON {
		t.Fatalf("JSON payload:\n%s\nwant:\n%s", jsonPayload, wantJSON)
	}
}

func TestDefaultValueValidationRejectsInvalidNumbersAndStructures(t *testing.T) {
	for _, value := range []string{"", "01", ".5", "NaN", "1/2"} {
		if _, err := Number(value); !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("Number(%q) error = %v, want ErrInvalidValue", value, err)
		}
	}
	tests := []Object{
		{FieldOf("", String("x"))},
		{FieldOf("same", String("x")), FieldOf("same", String("y"))},
		{FieldOf("nil", nil)},
		{FieldOf("list", List(nil))},
	}
	for index, object := range tests {
		if err := validateObject(object); !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("validateObject case %d error = %v, want ErrInvalidValue", index, err)
		}
	}
}

func TestNewDefaultManagerRejectsInvalidAndOverlappingBindings(t *testing.T) {
	contract := fixedDefaults(Object{})
	tests := [][]Binding{
		{{ConfigPath: "database", Contract: contract}},
		{{CapabilityID: "database", ConfigPath: "", Contract: contract}},
		{{CapabilityID: "database", ConfigPath: "database", Contract: nil}},
		{{CapabilityID: "one", ConfigPath: "database", Contract: contract}, {CapabilityID: "one", ConfigPath: "cache", Contract: contract}},
		{{CapabilityID: "one", ConfigPath: "database", Contract: contract}, {CapabilityID: "two", ConfigPath: "database", Contract: contract}},
		{{CapabilityID: "one", ConfigPath: "database", Contract: contract}, {CapabilityID: "two", ConfigPath: "database.read", Contract: contract}},
		{{CapabilityID: "one", ConfigPath: "database..read", Contract: contract}},
	}
	for index, bindings := range tests {
		if manager, err := NewDefaultManager(bindings...); err == nil || manager != nil {
			t.Fatalf("NewDefaultManager case %d = %#v, %v; want nil, error", index, manager, err)
		}
	}
}

func TestDefaultManagerGeneratesOrderedYAMLAndJSON(t *testing.T) {
	database := Object{
		FieldOf("driver", String("sqlite")),
		FieldOf("timeout", Duration(5*time.Second)),
	}
	manager, err := NewDefaultManager(Binding{
		CapabilityID: "database",
		ConfigPath:   "database",
		Contract:     fixedDefaults(database),
		Validate:     acceptSnapshot,
	})
	if err != nil {
		t.Fatalf("NewDefaultManager() error = %v", err)
	}
	tests := []struct {
		name string
		ext  string
		want string
	}{
		{name: "yaml", ext: ".YAML", want: "database:\n  driver: sqlite\n  timeout: 5s\n"},
		{name: "json", ext: ".json", want: "{\n  \"database\": {\n    \"driver\": \"sqlite\",\n    \"timeout\": \"5s\"\n  }\n}\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "nested", "config"+test.ext)
			result, err := manager.Generate(t.Context(), GenerateRequest{Path: target})
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			payload, err := os.ReadFile(target)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			if string(payload) != test.want {
				t.Fatalf("payload:\n%s\nwant:\n%s", payload, test.want)
			}
			absolute, _ := filepath.Abs(target)
			if result.Path != absolute || !reflect.DeepEqual(result.SectionIDs, []string{"database"}) {
				t.Fatalf("Generate() result = %#v", result)
			}
			if runtime.GOOS != "windows" {
				if info, statErr := os.Stat(target); statErr != nil || info.Mode().Perm() != 0o600 {
					t.Fatalf("target mode = %v, error = %v; want 0600", info.Mode().Perm(), statErr)
				}
			}
		})
	}
}

func TestDefaultManagerRejectsExistingTargetAndForceReplacesAtomically(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("original\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	manager, err := NewDefaultManager(Binding{
		CapabilityID: "database",
		ConfigPath:   "database",
		Contract:     fixedDefaults(Object{FieldOf("driver", String("sqlite"))}),
		Validate:     acceptSnapshot,
	})
	if err != nil {
		t.Fatalf("NewDefaultManager() error = %v", err)
	}
	if _, err := manager.Generate(t.Context(), GenerateRequest{Path: target}); !errors.Is(err, ErrTargetExists) {
		t.Fatalf("Generate(existing) error = %v, want ErrTargetExists", err)
	}
	assertFileBytes(t, target, original)
	result, err := manager.Generate(t.Context(), GenerateRequest{Path: target, Force: true})
	if err != nil {
		t.Fatalf("Generate(force) error = %v", err)
	}
	if !result.Replaced || !reflect.DeepEqual(result.SectionIDs, []string{"database"}) {
		t.Fatalf("Generate(force) result = %#v", result)
	}
	assertFileBytes(t, target, []byte("database:\n  driver: sqlite\n"))
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".default-config-") {
			t.Fatalf("temporary file remains: %s", entry.Name())
		}
	}
}

func TestDefaultManagerRejectsSymlinkTarget(t *testing.T) {
	directory := t.TempDir()
	realTarget := filepath.Join(directory, "real.yaml")
	if err := os.WriteFile(realTarget, []byte("original\n"), 0o600); err != nil {
		t.Fatalf("write real target: %v", err)
	}
	symlinkTarget := filepath.Join(directory, "config.yaml")
	if err := os.Symlink(realTarget, symlinkTarget); err != nil {
		t.Skipf("current environment cannot create symlink: %v", err)
	}
	manager, err := NewDefaultManager(Binding{
		CapabilityID: "database", ConfigPath: "database",
		Contract: fixedDefaults(Object{FieldOf("driver", String("sqlite"))}),
		Validate: acceptSnapshot,
	})
	if err != nil {
		t.Fatalf("NewDefaultManager() error = %v", err)
	}
	if _, err := manager.Generate(t.Context(), GenerateRequest{Path: symlinkTarget, Force: true}); err == nil {
		t.Fatal("Generate(force symlink) error = nil")
	}
	assertFileBytes(t, realTarget, []byte("original\n"))
}

func TestDefaultManagerAbortStopsContractsAndPreservesTarget(t *testing.T) {
	cause := errors.New("operator rejected defaults")
	called := false
	manager, err := NewDefaultManager(
		Binding{CapabilityID: "one", ConfigPath: "one", Contract: fixedDefaults(Object{FieldOf("value", String("one"))}), Validate: acceptSnapshot},
		Binding{CapabilityID: "two", ConfigPath: "two", Contract: DefaultContractFunc(func(context.Context) (Object, Control, error) {
			return nil, Abort, cause
		}), Validate: acceptSnapshot},
		Binding{CapabilityID: "three", ConfigPath: "three", Contract: DefaultContractFunc(func(context.Context) (Object, Control, error) {
			called = true
			return Object{}, Continue, nil
		}), Validate: acceptSnapshot},
	)
	if err != nil {
		t.Fatalf("NewDefaultManager() error = %v", err)
	}
	target := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("original\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err = manager.Generate(t.Context(), GenerateRequest{Path: target, Force: true})
	if !errors.Is(err, ErrAborted) || !errors.Is(err, cause) {
		t.Fatalf("Generate() error = %v, want ErrAborted and cause", err)
	}
	var aborted *AbortedError
	if !errors.As(err, &aborted) || aborted.CapabilityID != "two" {
		t.Fatalf("Generate() error = %#v, want capability two", err)
	}
	if called {
		t.Fatal("contract after Abort was called")
	}
	assertFileBytes(t, target, original)
}

func TestDefaultManagerRejectsInvalidControlResultsWithoutOutput(t *testing.T) {
	tests := []struct {
		name     string
		contract DefaultContract
	}{
		{name: "continue error", contract: DefaultContractFunc(func(context.Context) (Object, Control, error) {
			return nil, Continue, errors.New("failed")
		})},
		{name: "abort without cause", contract: DefaultContractFunc(func(context.Context) (Object, Control, error) {
			return nil, Abort, nil
		})},
		{name: "unknown control", contract: DefaultContractFunc(func(context.Context) (Object, Control, error) {
			return nil, Control(99), errors.New("failed")
		})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager, err := NewDefaultManager(Binding{CapabilityID: "test", ConfigPath: "test", Contract: test.contract, Validate: acceptSnapshot})
			if err != nil {
				t.Fatalf("NewDefaultManager() error = %v", err)
			}
			target := filepath.Join(t.TempDir(), "config.yaml")
			if _, err := manager.Generate(t.Context(), GenerateRequest{Path: target}); err == nil {
				t.Fatal("Generate() error = nil")
			}
			if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("target Stat() error = %v, want not exist", err)
			}
		})
	}
}

func TestDefaultManagerRejectsNilAndCancelledContextBeforeContract(t *testing.T) {
	called := false
	manager, err := NewDefaultManager(Binding{CapabilityID: "test", ConfigPath: "test", Contract: DefaultContractFunc(func(context.Context) (Object, Control, error) {
		called = true
		return Object{}, Continue, nil
	}), Validate: acceptSnapshot})
	if err != nil {
		t.Fatalf("NewDefaultManager() error = %v", err)
	}
	if _, err := manager.Generate(nil, GenerateRequest{Path: "config.yaml"}); err == nil {
		t.Fatal("Generate(nil) error = nil")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := manager.Generate(ctx, GenerateRequest{Path: "config.yaml"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Generate(cancelled) error = %v, want context.Canceled", err)
	}
	if called {
		t.Fatal("contract was called after context cancellation")
	}
}

func acceptSnapshot(Snapshot) error { return nil }

func TestDefaultFileTransactionPreservesPrimaryAndCleanupErrors(t *testing.T) {
	writeErr := errors.New("write failed")
	closeErr := errors.New("close failed")
	cleanupErr := errors.New("cleanup failed")
	operations := &fakeDefaultFileOperations{
		file:      &fakeTemporaryDefaultFile{writeErr: writeErr, closeErr: closeErr},
		removeErr: cleanupErr,
	}
	err := writeDefaultFileWithOperations("target.yaml", []byte("payload"), false, operations)
	for _, want := range []error{writeErr, closeErr, cleanupErr} {
		if !errors.Is(err, want) {
			t.Fatalf("write transaction error = %v, want chain containing %v", err, want)
		}
	}
}

func TestDefaultFileTransactionReportsSyncCloseReplaceAndExclusiveCreateErrors(t *testing.T) {
	tests := []struct {
		name       string
		force      bool
		file       *fakeTemporaryDefaultFile
		operations *fakeDefaultFileOperations
		want       []error
	}{
		{
			name: "sync and close",
			file: &fakeTemporaryDefaultFile{syncErr: errors.New("sync failed"), closeErr: errors.New("close failed")},
		},
		{
			name:  "close",
			file:  &fakeTemporaryDefaultFile{closeErr: errors.New("close failed")},
			force: true,
		},
		{
			name:       "replace",
			file:       &fakeTemporaryDefaultFile{},
			force:      true,
			operations: &fakeDefaultFileOperations{replaceErr: errors.New("replace failed")},
		},
		{
			name:       "exclusive target",
			file:       &fakeTemporaryDefaultFile{},
			operations: &fakeDefaultFileOperations{linkErr: os.ErrExist},
			want:       []error{ErrTargetExists},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			operations := test.operations
			if operations == nil {
				operations = &fakeDefaultFileOperations{}
			}
			operations.file = test.file
			err := writeDefaultFileWithOperations("target.yaml", []byte("payload"), test.force, operations)
			if err == nil {
				t.Fatal("write transaction error = nil")
			}
			wants := test.want
			if len(wants) == 0 {
				for _, candidate := range []error{test.file.syncErr, test.file.closeErr, operations.replaceErr} {
					if candidate != nil {
						wants = append(wants, candidate)
					}
				}
			}
			for _, want := range wants {
				if !errors.Is(err, want) {
					t.Fatalf("write transaction error = %v, want chain containing %v", err, want)
				}
			}
		})
	}
}

func fixedDefaults(object Object) DefaultContract {
	return DefaultContractFunc(func(context.Context) (Object, Control, error) {
		return object, Continue, nil
	})
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("file bytes = %q, want %q", got, want)
	}
}

type fakeTemporaryDefaultFile struct {
	chmodErr error
	writeErr error
	syncErr  error
	closeErr error
}

func (*fakeTemporaryDefaultFile) Name() string              { return "temporary.yaml" }
func (f *fakeTemporaryDefaultFile) Chmod(os.FileMode) error { return f.chmodErr }
func (f *fakeTemporaryDefaultFile) Write(payload []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(payload), nil
}
func (f *fakeTemporaryDefaultFile) Sync() error  { return f.syncErr }
func (f *fakeTemporaryDefaultFile) Close() error { return f.closeErr }

type fakeDefaultFileOperations struct {
	file       temporaryDefaultFile
	mkdirErr   error
	createErr  error
	linkErr    error
	removeErr  error
	replaceErr error
}

func (f *fakeDefaultFileOperations) MkdirAll(string, os.FileMode) error { return f.mkdirErr }
func (f *fakeDefaultFileOperations) CreateTemp(string, string) (temporaryDefaultFile, error) {
	return f.file, f.createErr
}
func (f *fakeDefaultFileOperations) Link(string, string) error    { return f.linkErr }
func (f *fakeDefaultFileOperations) Remove(string) error          { return f.removeErr }
func (f *fakeDefaultFileOperations) Replace(string, string) error { return f.replaceErr }
