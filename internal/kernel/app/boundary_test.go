package app_test

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

func TestAppLayerDoesNotImportKernelOrComposition(t *testing.T) {
	root := appRoot(t)
	modulePrefix := strings.Join([]string{"github.com", "rin721", "go-scaffold-template", "internal", "kernel"}, "/")
	forbidden := map[string]struct{}{
		modulePrefix:                  {},
		modulePrefix + "/composition": {},
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

func TestAppExportsHideThirdPartySelectors(t *testing.T) {
	root := appRoot(t)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		imports := make(map[string]string, len(file.Imports))
		for _, imported := range file.Imports {
			value, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			name := filepath.Base(value)
			if imported.Name != nil {
				name = imported.Name.Name
			}
			imports[name] = value
		}
		for _, declaration := range file.Decls {
			for _, contract := range appExportedContractNodes(declaration) {
				var leaked string
				ast.Inspect(contract, func(node ast.Node) bool {
					selector, ok := node.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					identifier, ok := selector.X.(*ast.Ident)
					if !ok {
						return true
					}
					importPath := imports[identifier.Name]
					if appThirdPartyImport(importPath) {
						leaked = importPath
						return false
					}
					return true
				})
				if leaked != "" {
					t.Fatalf("exported App contract in %s leaks third-party package %s", path, leaked)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk App exported API: %v", err)
	}
}

func appExportedContractNodes(declaration ast.Decl) []ast.Node {
	switch current := declaration.(type) {
	case *ast.FuncDecl:
		if current.Name.IsExported() && appExportedReceiver(current.Recv) {
			return []ast.Node{current.Type}
		}
	case *ast.GenDecl:
		var result []ast.Node
		for _, specification := range current.Specs {
			switch spec := specification.(type) {
			case *ast.TypeSpec:
				if !spec.Name.IsExported() {
					continue
				}
				switch value := spec.Type.(type) {
				case *ast.StructType:
					for _, field := range value.Fields.List {
						if len(field.Names) == 0 || field.Names[0].IsExported() {
							result = append(result, field.Type)
						}
					}
				case *ast.InterfaceType:
					for _, method := range value.Methods.List {
						if len(method.Names) == 0 || method.Names[0].IsExported() {
							result = append(result, method.Type)
						}
					}
				default:
					result = append(result, spec.Type)
				}
			}
		}
		return result
	}
	return nil
}

func appExportedReceiver(receiver *ast.FieldList) bool {
	if receiver == nil {
		return true
	}
	if len(receiver.List) != 1 {
		return false
	}
	typeNode := receiver.List[0].Type
	if pointer, ok := typeNode.(*ast.StarExpr); ok {
		typeNode = pointer.X
	}
	identifier, ok := typeNode.(*ast.Ident)
	return ok && identifier.IsExported()
}

func appThirdPartyImport(importPath string) bool {
	if importPath == "" || strings.HasPrefix(importPath, "github.com/rin721/go-scaffold-template/") {
		return false
	}
	first, _, _ := strings.Cut(importPath, "/")
	return strings.Contains(first, ".")
}

func appRoot(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return root
}
