package composition

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
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

func forbiddenModuleCoreImport(importPath string) bool {
	return strings.HasPrefix(importPath, modulePath+"/internal/kernel") ||
		importPath == modulePath+"/pkg/httpx" ||
		importPath == modulePath+"/pkg/cli" ||
		importPath == modulePath+"/pkg/database"
}
