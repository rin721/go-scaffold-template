package composition

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/rin721/go-scaffold-template"

type packageNode struct {
	ImportPath string
	Imports    []string
}

func TestProductionPackageGraphRespectsCompositionBoundaries(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	command := exec.Command("go", "list", "-json", "./...")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list package graph: %v", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var graph []packageNode
	for {
		var node packageNode
		if err := decoder.Decode(&node); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("decode package graph: %v", err)
		}
		graph = append(graph, node)
	}
	if err := validatePackageGraph(graph); err != nil {
		t.Fatal(err)
	}
	if err := validateHTTPSourceOwnership(root); err != nil {
		t.Fatal(err)
	}
}

func TestPackageGraphRulesAcceptLegalFixtureAndRejectViolations(t *testing.T) {
	legal := []packageNode{
		{ImportPath: modulePath + "/cmd/app", Imports: []string{modulePath + "/internal/composition"}},
		{ImportPath: modulePath + "/internal/composition", Imports: []string{
			modulePath + "/internal/kernel/composition", modulePath + "/internal/module/todo",
		}},
		{ImportPath: modulePath + "/internal/kernel/composition", Imports: []string{modulePath + "/internal/kernel/app/database"}},
		{ImportPath: modulePath + "/internal/kernel/app/database", Imports: []string{modulePath + "/pkg/database"}},
		{ImportPath: modulePath + "/internal/module/todo/service", Imports: []string{modulePath + "/internal/module/todo/model"}},
		{ImportPath: modulePath + "/internal/module/todo/repo", Imports: []string{modulePath + "/pkg/database"}},
		{ImportPath: modulePath + "/internal/module/todo/binding/http", Imports: []string{
			modulePath + "/internal/module/todo/service", modulePath + "/internal/transport/http/api", modulePath + "/pkg/httpx",
		}},
		{ImportPath: modulePath + "/internal/transport/http", Imports: []string{
			modulePath + "/internal/transport/http/api", modulePath + "/pkg/httpx",
		}},
	}
	if err := validatePackageGraph(legal); err != nil {
		t.Fatalf("legal fixture error = %v", err)
	}
	for _, fixture := range [][]packageNode{
		{{ImportPath: modulePath + "/pkg/database", Imports: []string{modulePath + "/internal/kernel"}}},
		{{ImportPath: modulePath + "/internal/kernel/app/database", Imports: []string{modulePath + "/internal/kernel/composition"}}},
		{{ImportPath: modulePath + "/internal/module/example", Imports: []string{modulePath + "/internal/kernel/composition"}}},
		{{ImportPath: modulePath + "/internal/module/example", Imports: []string{modulePath + "/internal/composition"}}},
		{{ImportPath: modulePath + "/internal/module/todo/service", Imports: []string{modulePath + "/internal/kernel"}}},
		{{ImportPath: modulePath + "/internal/module/todo/model", Imports: []string{modulePath + "/pkg/httpx"}}},
		{{ImportPath: modulePath + "/internal/module/todo/service", Imports: []string{modulePath + "/pkg/database"}}},
		{{ImportPath: modulePath + "/cmd/app", Imports: []string{modulePath + "/internal/module/ops/model"}}},
		{{ImportPath: modulePath + "/internal/transport/http", Imports: []string{modulePath + "/internal/module/todo/service"}}},
		{{ImportPath: modulePath + "/internal/module/todo/binding/http", Imports: []string{modulePath + "/internal/module/auth/model"}}},
		{{ImportPath: modulePath + "/internal/module/todo/binding/http", Imports: []string{"github.com/go-chi/chi/v5"}}},
		{{ImportPath: modulePath + "/internal/module/todo/binding/http", Imports: []string{"github.com/oapi-codegen/nethttp-middleware"}}},
		{{ImportPath: modulePath + "/internal/module/todo", Imports: []string{modulePath + "/internal/kernel/composition"}}},
	} {
		if err := validatePackageGraph(fixture); err == nil {
			t.Fatalf("invalid fixture %#v passed", fixture)
		}
	}
}

func validatePackageGraph(graph []packageNode) error {
	for _, node := range graph {
		for _, imported := range node.Imports {
			if moduleHTTPBindingPackage(node.ImportPath) && forbiddenModuleHTTPBindingImport(imported) {
				return fmt.Errorf("module HTTP binding %s imports application route infrastructure %s", node.ImportPath, imported)
			}
			if node.ImportPath == modulePath+"/cmd/app" && strings.HasPrefix(imported, modulePath+"/") &&
				imported != modulePath+"/internal/composition" {
				return fmt.Errorf("application entrypoint %s imports forbidden project package %s", node.ImportPath, imported)
			}
			if strings.HasPrefix(node.ImportPath, modulePath+"/pkg/") && strings.HasPrefix(imported, modulePath+"/internal/") {
				return fmt.Errorf("package %s imports forbidden internal package %s", node.ImportPath, imported)
			}
			if strings.HasPrefix(node.ImportPath, modulePath+"/internal/kernel/app/") &&
				(imported == modulePath+"/internal/kernel" || imported == modulePath+"/internal/kernel/composition") {
				return fmt.Errorf("component package %s imports upper owner %s", node.ImportPath, imported)
			}
			if imported == modulePath+"/internal/kernel/composition" && node.ImportPath != modulePath+"/internal/composition" {
				return fmt.Errorf("package %s bypasses the production composition root", node.ImportPath)
			}
			if imported == modulePath+"/internal/composition" && node.ImportPath != modulePath+"/cmd/app" {
				return fmt.Errorf("package %s bypasses the application composition root", node.ImportPath)
			}
			sourceOwner, sourceIsModule := applicationModuleOwner(node.ImportPath)
			importedOwner, importedIsModule := applicationModuleOwner(imported)
			if sourceIsModule && importedIsModule && sourceOwner != importedOwner {
				return fmt.Errorf("application module %s imports another module owner %s through %s", sourceOwner, importedOwner, imported)
			}
			if importedIsModule && node.ImportPath != modulePath+"/internal/composition" &&
				(!sourceIsModule || sourceOwner != importedOwner) {
				return fmt.Errorf("package %s imports application module %s outside the composition root", node.ImportPath, imported)
			}
			if sourceIsModule && (imported == modulePath+"/internal/composition" ||
				imported == modulePath+"/internal/kernel/composition") {
				return fmt.Errorf("application module %s imports upper owner %s", sourceOwner, imported)
			}
			if moduleCorePackage(node.ImportPath) && forbiddenModuleCoreImport(imported) {
				return fmt.Errorf("module core package %s imports forbidden boundary %s", node.ImportPath, imported)
			}
		}
	}
	return nil
}

func validateHTTPSourceOwnership(root string) error {
	type assertion struct {
		path string
		line int
	}
	var moduleRouteCalls []assertion
	var strictInterfaceAssertions []assertion
	var handlerBindings []assertion
	var strictBindings []assertion
	internalRoot := filepath.Join(root, "internal")
	err := filepath.WalkDir(internalRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".gen.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read architecture source %s: %w", path, err)
		}
		fileset := token.NewFileSet()
		parsed, err := parser.ParseFile(fileset, path, source, 0)
		if err != nil {
			return fmt.Errorf("parse architecture source %s: %w", path, err)
		}
		imports := make(map[string]string, len(parsed.Imports))
		for _, spec := range parsed.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("decode import in %s: %w", path, err)
			}
			name := filepath.Base(importPath)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			imports[name] = importPath
			if moduleHTTPBindingSource(root, path) && forbiddenModuleHTTPBindingImport(importPath) {
				return fmt.Errorf("module HTTP binding %s imports application route infrastructure %s", path, importPath)
			}
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			switch current := node.(type) {
			case *ast.CallExpr:
				selector, ok := current.Fun.(*ast.SelectorExpr)
				if !ok || selectorImportPath(selector, imports) != modulePath+"/internal/transport/http/api" {
					return true
				}
				position := fileset.Position(selector.Pos())
				item := assertion{path: path, line: position.Line}
				switch selector.Sel.Name {
				case "GetSwagger", "Handler", "HandlerFromMux", "HandlerFromMuxWithBaseURL", "HandlerWithOptions", "NewStrictHandler", "NewStrictHandlerWithOptions":
					if moduleHTTPBindingSource(root, path) {
						moduleRouteCalls = append(moduleRouteCalls, item)
					}
				}
				if selector.Sel.Name == "HandlerWithOptions" {
					handlerBindings = append(handlerBindings, item)
				}
				if selector.Sel.Name == "NewStrictHandlerWithOptions" {
					strictBindings = append(strictBindings, item)
				}
			case *ast.ValueSpec:
				if len(current.Names) != 1 || current.Names[0].Name != "_" {
					return true
				}
				selector, ok := current.Type.(*ast.SelectorExpr)
				if ok && selector.Sel.Name == "StrictServerInterface" &&
					selectorImportPath(selector, imports) == modulePath+"/internal/transport/http/api" {
					position := fileset.Position(selector.Pos())
					strictInterfaceAssertions = append(strictInterfaceAssertions, assertion{path: path, line: position.Line})
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		return err
	}
	if len(moduleRouteCalls) > 0 {
		return fmt.Errorf("module HTTP binding owns generated application route/server at %#v", moduleRouteCalls)
	}
	if len(handlerBindings) != 1 || !pathWithin(root, handlerBindings[0].path, "internal", "transport", "http") {
		return fmt.Errorf("generated HandlerWithOptions binding must exist once under internal/transport/http, got %#v", handlerBindings)
	}
	if len(strictBindings) != 1 || !pathWithin(root, strictBindings[0].path, "internal", "transport", "http") {
		return fmt.Errorf("generated strict binding must exist once under internal/transport/http, got %#v", strictBindings)
	}
	if len(strictInterfaceAssertions) != 1 || !pathWithin(root, strictInterfaceAssertions[0].path, "internal", "composition") {
		return fmt.Errorf("complete StrictServerInterface assertion must exist once under internal/composition, got %#v", strictInterfaceAssertions)
	}
	return nil
}

func selectorImportPath(selector *ast.SelectorExpr, imports map[string]string) string {
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return imports[identifier.Name]
}

func moduleHTTPBindingSource(root, path string) bool {
	relative, err := filepath.Rel(filepath.Join(root, "internal", "module"), path)
	if err != nil || strings.HasPrefix(relative, "..") {
		return false
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	return len(parts) >= 4 && parts[1] == "binding" && parts[2] == "http"
}

func pathWithin(root, path string, parts ...string) bool {
	target := filepath.Join(append([]string{root}, parts...)...)
	relative, err := filepath.Rel(target, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func applicationModuleOwner(importPath string) (string, bool) {
	prefix := modulePath + "/internal/module/"
	if !strings.HasPrefix(importPath, prefix) {
		return "", false
	}
	relative := strings.TrimPrefix(importPath, prefix)
	owner, _, _ := strings.Cut(relative, "/")
	return owner, owner != ""
}

func moduleCorePackage(importPath string) bool {
	prefix := modulePath + "/internal/module/"
	if !strings.HasPrefix(importPath, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(importPath, prefix), "/")
	return len(parts) >= 2 && (parts[1] == "model" || parts[1] == "service")
}

func moduleHTTPBindingPackage(importPath string) bool {
	prefix := modulePath + "/internal/module/"
	if !strings.HasPrefix(importPath, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(importPath, prefix), "/")
	return len(parts) >= 3 && parts[1] == "binding" && parts[2] == "http"
}

func forbiddenModuleHTTPBindingImport(importPath string) bool {
	return importPath == "github.com/go-chi/chi/v5" ||
		strings.HasPrefix(importPath, "github.com/getkin/kin-openapi") ||
		importPath == "github.com/oapi-codegen/nethttp-middleware"
}

func forbiddenModuleCoreImport(importPath string) bool {
	return strings.HasPrefix(importPath, modulePath+"/internal/kernel") ||
		importPath == modulePath+"/pkg/httpx" ||
		importPath == modulePath+"/pkg/cli" ||
		importPath == modulePath+"/pkg/database"
}
