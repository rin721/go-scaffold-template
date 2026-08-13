package pkg_test

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
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
	internalPrefix := strings.Join([]string{"github.com", "rin721", "go-scaffold2", "internal"}, "/") + "/"
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
	oldImport := strings.Join([]string{"github.com", "rin721", "go-scaffold2", "pkg", "config"}, "/")
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
