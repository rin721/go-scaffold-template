package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/rin721/go-scaffold-template/pkg/httpx/contract"
)

// TestGeneratedOpenAPIMatchesCommittedAuthority 校验 030 方向反转的兼容性：由模块契约生成的
// openapi.yaml 必须保留当前已提交 api/openapi.yaml 的公开语义（路径、方法、operationId）。
func TestGeneratedOpenAPIMatchesCommittedAuthority(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	committed, err := os.ReadFile(filepath.Join(repoRoot, "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read committed openapi.yaml: %v", err)
	}
	loader := openapi3.NewLoader()
	committedSpec, err := loader.LoadFromData(committed)
	if err != nil {
		t.Fatalf("load committed spec: %v", err)
	}

	document, err := contract.BuildDocument(contract.Info{Title: "t", Version: "1", Description: "d"}, registeredModules())
	if err != nil {
		t.Fatalf("build generated document: %v", err)
	}
	yamlBytes, err := document.MarshalYAML()
	if err != nil {
		t.Fatalf("marshal generated doc: %v", err)
	}
	generatedSpec, err := loader.LoadFromData(yamlBytes)
	if err != nil {
		t.Fatalf("load generated spec: %v", err)
	}

	generatedPaths := generatedSpec.Paths.Map()
	for path, committedItem := range committedSpec.Paths.Map() {
		generatedItem, ok := generatedPaths[path]
		if !ok {
			t.Errorf("generated spec is missing committed path %q", path)
			continue
		}
		for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
			committedOp := operationForMethod(committedItem, method)
			generatedOp := operationForMethod(generatedItem, method)
			if committedOp == nil && generatedOp == nil {
				continue
			}
			if committedOp == nil || generatedOp == nil {
				t.Errorf("path %s method %s presence differs (committed=%v generated=%v)", path, method, committedOp != nil, generatedOp != nil)
				continue
			}
			if committedOp.OperationID != generatedOp.OperationID {
				t.Errorf("path %s method %s operationId differs: committed=%q generated=%q", path, method, committedOp.OperationID, generatedOp.OperationID)
			}
		}
	}
}

func operationForMethod(item *openapi3.PathItem, method string) *openapi3.Operation {
	switch method {
	case "GET":
		return item.Get
	case "POST":
		return item.Post
	case "PUT":
		return item.Put
	case "PATCH":
		return item.Patch
	case "DELETE":
		return item.Delete
	default:
		return nil
	}
}
