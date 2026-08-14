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

const modulePath = "github.com/rin721/go-scaffold2"

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
		{ImportPath: modulePath + "/cmd/app", Imports: []string{modulePath + "/internal/kernel/composition"}},
		{ImportPath: modulePath + "/internal/kernel/composition", Imports: []string{modulePath + "/internal/kernel/app/database"}},
		{ImportPath: modulePath + "/internal/kernel/app/database", Imports: []string{modulePath + "/pkg/database"}},
	}
	if err := validatePackageGraph(legal); err != nil {
		t.Fatalf("legal fixture error = %v", err)
	}
	for _, fixture := range [][]packageNode{
		{{ImportPath: modulePath + "/pkg/database", Imports: []string{modulePath + "/internal/kernel"}}},
		{{ImportPath: modulePath + "/internal/kernel/app/database", Imports: []string{modulePath + "/internal/kernel/composition"}}},
		{{ImportPath: modulePath + "/internal/feature", Imports: []string{modulePath + "/internal/kernel/composition"}}},
	} {
		if err := validatePackageGraph(fixture); err == nil {
			t.Fatalf("invalid fixture %#v passed", fixture)
		}
	}
}

func validatePackageGraph(graph []packageNode) error {
	for _, node := range graph {
		for _, imported := range node.Imports {
			if strings.HasPrefix(node.ImportPath, modulePath+"/pkg/") && strings.HasPrefix(imported, modulePath+"/internal/") {
				return fmt.Errorf("package %s imports forbidden internal package %s", node.ImportPath, imported)
			}
			if strings.HasPrefix(node.ImportPath, modulePath+"/internal/kernel/app/") &&
				(imported == modulePath+"/internal/kernel" || imported == modulePath+"/internal/kernel/composition") {
				return fmt.Errorf("component package %s imports upper owner %s", node.ImportPath, imported)
			}
			if imported == modulePath+"/internal/kernel/composition" && node.ImportPath != modulePath+"/cmd/app" {
				return fmt.Errorf("package %s bypasses the production composition root", node.ImportPath)
			}
		}
	}
	return nil
}
