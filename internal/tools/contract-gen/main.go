// Command contract-gen 从模块契约声明生成 OpenAPI 文档与 operation inventory。
//
// 输入是编译期注册的模块契约包（registry.go），输出：
//   - api/openapi.yaml
//   - internal/transport/http/api/operation_inventory.gen.go
//
// 这是 030 计划中的 "契约文件由代码生成"：Go 代码是唯一 authority，openapi.yaml 与
// operation inventory 都是由本项目生成器从代码渲染的产物，不再由 openapi.yaml 生成代码。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	todohttp "github.com/rin721/go-scaffold-template/internal/module/todo/binding/http"
	"github.com/rin721/go-scaffold-template/pkg/httpx/contract"
)

func main() {
	outputOpenAPI := flag.String("output-openapi", "api/openapi.yaml", "生成的 OpenAPI 文件路径")
	outputInventory := flag.String("output-inventory", "internal/transport/http/api/operation_inventory.gen.go", "生成的 operation inventory Go 文件路径")
	packageName := flag.String("package", "api", "生成 inventory 的 package")
	flag.Parse()

	if err := run(*outputOpenAPI, *outputInventory, *packageName); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(outputOpenAPI, outputInventory, packageName string) error {
	modules := registeredModules()

	info := contract.Info{
		Title:   "go-scaffold-template HTTP API",
		Version: "1.0.0-rc.1",
		Description: "当前模板的公开 HTTP 契约。所有失败使用 RFC 9457 Problem Details；" +
			"Todo operation 使用 bearer JWT，并由生成的 policy inventory 驱动授权。",
	}
	document, err := contract.BuildDocument(info, modules)
	if err != nil {
		return fmt.Errorf("build OpenAPI document: %w", err)
	}
	if err := document.Validate(); err != nil {
		return fmt.Errorf("validate generated OpenAPI document: %w", err)
	}
	openAPIYAML, err := document.MarshalYAML()
	if err != nil {
		return fmt.Errorf("marshal OpenAPI document: %w", err)
	}
	if err := writeFile(outputOpenAPI, openAPIYAML); err != nil {
		return fmt.Errorf("write OpenAPI document: %w", err)
	}

	inventory, err := contract.GenerateOperationsGo(packageName, modules)
	if err != nil {
		return fmt.Errorf("generate operation inventory: %w", err)
	}
	if err := writeFile(outputInventory, inventory); err != nil {
		return fmt.Errorf("write operation inventory: %w", err)
	}
	return nil
}

func writeFile(path string, content []byte) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output dir %s: %w", dir, err)
		}
	}
	// #nosec G306 -- 生成的 YAML/Go 源码不含 secret，0644 保持跨平台可读。
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// registeredModules 返回所有参与生成的模块契约。生成工具 import 模块契约包是 030 计划允许的：
// 生成器独立于 production 运行图，且后续新增模块只需在此追加一行并扩展 ModuleContract 注册。
func registeredModules() []contract.Module {
	return []contract.Module{
		todohttp.ModuleContract(),
	}
}
