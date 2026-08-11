package capability_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCapabilityDefinitionsHaveNoRegistrationSideEffects(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("get capability root: %v", err)
	}
	kernelImport := strings.Join([]string{"github.com", "rin721", "go-scaffold2", "internal", "kernel"}, "/")
	discoveryCalls := map[string]map[string]struct{}{
		"os":            {"ReadDir": {}},
		"path/filepath": {"Glob": {}, "Walk": {}, "WalkDir": {}},
		"io/fs":         {"Glob": {}, "WalkDir": {}},
	}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		kernelAliases := make(map[string]struct{})
		discoveryAliases := make(map[string]map[string]struct{})
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if importPath == "reflect" {
				t.Fatalf("capability %s imports reflect; automatic discovery is forbidden", path)
			}
			if importPath == kernelImport {
				name := "kernel"
				if imported.Name != nil {
					name = imported.Name.Name
				}
				kernelAliases[name] = struct{}{}
			}
			if calls, forbidden := discoveryCalls[importPath]; forbidden {
				name := filepath.Base(importPath)
				if imported.Name != nil {
					name = imported.Name.Name
				}
				discoveryAliases[name] = calls
			}
		}

		file, err = parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.FuncDecl:
				if value.Name.Name == "init" {
					t.Fatalf("capability %s declares init; composition must own registration", path)
				}
			case *ast.CallExpr:
				selector, ok := value.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				qualifier, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				if selector.Sel.Name == "Register" {
					if _, forbidden := kernelAliases[qualifier.Name]; forbidden {
						t.Fatalf("capability %s calls kernel.Register; composition must own registration", path)
					}
				}
				if calls, forbidden := discoveryAliases[qualifier.Name]; forbidden {
					if _, discovery := calls[selector.Sel.Name]; discovery {
						t.Fatalf("capability %s calls %s.%s; automatic discovery is forbidden", path, qualifier.Name, selector.Sel.Name)
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk capabilities: %v", err)
	}
}
