package pkg_test

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestNoOldStorageModuleImportsRemain(t *testing.T) {
	root := packageRoot(t)
	oldModule := strings.Join([]string{"github.com", "open-console", "console-platform"}, "/")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return walkErr
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), oldModule) {
			t.Fatalf("%s still references old storage module", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk pkg: %v", err)
	}
}

func TestPackageLayerDoesNotImportInternal(t *testing.T) {
	root := packageRoot(t)
	internalPrefix := strings.Join([]string{"github.com", "rin721", "go-scaffold-template", "internal"}, "/") + "/"
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
			if strings.HasPrefix(value, internalPrefix) {
				t.Fatalf("pkg layer %s imports internal package %s", path, value)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk pkg imports: %v", err)
	}
}

func TestOldConfigPackageIsRemoved(t *testing.T) {
	oldDirectory := filepath.Join(packageRoot(t), "config")
	if _, err := os.Stat(oldDirectory); !os.IsNotExist(err) {
		t.Fatalf("old config directory still exists: %s", oldDirectory)
	}

	root := repoRoot(t)
	oldImport := strings.Join([]string{"github.com", "rin721", "go-scaffold-template", "pkg", "config"}, "/")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !(strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".md")) {
			return walkErr
		}
		if filepath.Base(path) == "boundary_test.go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), oldImport) {
			t.Fatalf("%s still references old config import", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk old config references: %v", err)
	}
}

func TestStorageReloadEntryIsRemoved(t *testing.T) {
	path := filepath.Join(packageRoot(t), "storage", "fileservice.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse storage API: %v", err)
	}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, spec := range general.Specs {
			typeSpec := spec.(*ast.TypeSpec)
			if typeSpec.Name.Name != "Storage" {
				continue
			}
			iface := typeSpec.Type.(*ast.InterfaceType)
			for _, method := range iface.Methods.List {
				for _, name := range method.Names {
					if name.Name == "Reload" {
						t.Fatal("Storage interface still exposes Reload")
					}
				}
			}
		}
	}
}

func TestStoragePublicAPIHidesSelectedThirdPartyTypes(t *testing.T) {
	path := filepath.Join(packageRoot(t), "storage", "fileservice.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse storage API: %v", err)
	}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, spec := range general.Specs {
			typeSpec := spec.(*ast.TypeSpec)
			if !typeSpec.Name.IsExported() {
				continue
			}
			rendered := renderNode(typeSpec.Type)
			for _, forbidden := range []string{"afero.", "excelize.", "imaging."} {
				if strings.Contains(rendered, forbidden) {
					t.Fatalf("exported storage type %s leaks %s in %s", typeSpec.Name.Name, forbidden, rendered)
				}
			}
		}
	}
}

func TestExportedPackageAPIHidesSelectedThirdPartyTypes(t *testing.T) {
	root := packageRoot(t)
	forbidden := []string{"tea.ProgramOption", "afero.", "excelize.", "imaging.", "gorm."}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, declaration := range file.Decls {
			switch decl := declaration.(type) {
			case *ast.GenDecl:
				if decl.Tok != token.TYPE {
					continue
				}
				for _, spec := range decl.Specs {
					typeSpec := spec.(*ast.TypeSpec)
					if typeSpec.Name.IsExported() {
						assertNoForbiddenPublicType(t, path, typeSpec.Name.Name, renderNode(typeSpec.Type), forbidden)
					}
				}
			case *ast.FuncDecl:
				if decl.Name.IsExported() {
					assertNoForbiddenPublicType(t, path, decl.Name.Name, renderNode(decl.Type), forbidden)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk exported package API: %v", err)
	}
}

func TestExportedPackageAPIHidesThirdPartySelectors(t *testing.T) {
	root := packageRoot(t)
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
			for _, contract := range exportedContractNodes(declaration) {
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
					if thirdPartyImport(importPath) {
						leaked = importPath
						return false
					}
					return true
				})
				if leaked != "" {
					t.Fatalf("exported contract in %s leaks third-party package %s", path, leaked)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk exported package API: %v", err)
	}
}

func exportedContractNodes(declaration ast.Decl) []ast.Node {
	switch current := declaration.(type) {
	case *ast.FuncDecl:
		if current.Name.IsExported() && exportedReceiver(current.Recv) {
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
			case *ast.ValueSpec:
				for _, name := range spec.Names {
					if name.IsExported() && spec.Type != nil {
						result = append(result, spec.Type)
						break
					}
				}
			}
		}
		return result
	}
	return nil
}

func exportedReceiver(receiver *ast.FieldList) bool {
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

func thirdPartyImport(importPath string) bool {
	if importPath == "" || strings.HasPrefix(importPath, "github.com/rin721/go-scaffold-template/") {
		return false
	}
	first, _, _ := strings.Cut(importPath, "/")
	return strings.Contains(first, ".")
}

func assertNoForbiddenPublicType(t *testing.T, path string, symbol string, rendered string, forbidden []string) {
	t.Helper()
	for _, value := range forbidden {
		if strings.Contains(rendered, value) {
			t.Fatalf("exported symbol %s in %s leaks %s in %s", symbol, path, value, rendered)
		}
	}
}

func renderNode(node ast.Node) string {
	var builder strings.Builder
	_ = printer.Fprint(&builder, token.NewFileSet(), node)
	return builder.String()
}

func packageRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return wd
}

func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Dir(packageRoot(t))
}
