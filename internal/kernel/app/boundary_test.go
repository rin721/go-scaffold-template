package app_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppLayerDoesNotImportKernelOrComposition(t *testing.T) {
	root := appRoot(t)
	modulePrefix := strings.Join([]string{"github.com", "rin721", "go-scaffold2", "internal", "kernel"}, "/")
	forbidden := map[string]struct{}{
		modulePrefix:                  {},
		modulePrefix + "/composition": {},
		modulePrefix + "/logging":     {},
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return walkErr
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			value := strings.Trim(imported.Path.Value, "\"")
			if strings.HasPrefix(value, modulePrefix+"/builtin") {
				t.Fatalf("app layer %s imports forbidden builtin package %s", path, value)
			}
			if _, exists := forbidden[value]; exists {
				t.Fatalf("app layer %s imports forbidden package %s", path, value)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk app imports: %v", err)
	}
}

func TestRemovedCapabilityDirectoryDoesNotReturn(t *testing.T) {
	oldPath := filepath.Join(filepath.Dir(appRoot(t)), "capability")
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("removed capability directory still exists: %s", oldPath)
	}
}

func appRoot(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return root
}
