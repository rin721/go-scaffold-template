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
	if err := validateModuleExportBoundaries(root); err != nil {
		t.Fatal(err)
	}
	if err := validateKernelAppConfigOwnership(root); err != nil {
		t.Fatal(err)
	}
	if err := validateCompositionOwnership(root); err != nil {
		t.Fatal(err)
	}
	if err := validateLoggingSourceOwnership(root); err != nil {
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
		{ImportPath: modulePath + "/internal/kernel/app/messaging/rabbitmq", Imports: []string{"github.com/rabbitmq/amqp091-go"}},
		{ImportPath: modulePath + "/internal/module/todo/service", Imports: []string{modulePath + "/internal/module/todo/model"}},
		{ImportPath: modulePath + "/internal/module/todo/repo", Imports: []string{modulePath + "/pkg/database"}},
		{ImportPath: modulePath + "/internal/module/auth/adapter/jwt", Imports: []string{"github.com/lestrrat-go/jwx/v3/jwt"}},
		{ImportPath: modulePath + "/internal/module/todo/binding/http", Imports: []string{
			modulePath + "/internal/module/todo/service", modulePath + "/pkg/httpx", modulePath + "/pkg/httpx/contract",
		}},
		{ImportPath: modulePath + "/internal/transport/http", Imports: []string{
			modulePath + "/pkg/httpx", modulePath + "/pkg/httpx/contract", modulePath + "/internal/transport/http/api",
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
		{{ImportPath: modulePath + "/internal/module/todo/handler", Imports: []string{modulePath + "/internal/module/todo/binding/http"}}},
		{{ImportPath: modulePath + "/internal/module/todo/handler", Imports: []string{modulePath + "/internal/transport/http"}}},
		{{ImportPath: modulePath + "/internal/module/todo/handler", Imports: []string{"github.com/go-chi/chi/v5"}}},
		{{ImportPath: modulePath + "/internal/module/todo", Imports: []string{modulePath + "/internal/kernel/composition"}}},
		{{ImportPath: modulePath + "/internal/module/example/service", Imports: []string{"github.com/example/sdk"}}},
		{{ImportPath: modulePath + "/internal/module/example", Imports: []string{"github.com/example/sdk"}}},
		{{ImportPath: modulePath + "/internal/composition", Imports: []string{modulePath + "/internal/module/example/adapter/sdk"}}},
		{{ImportPath: modulePath + "/internal/composition", Imports: []string{"go.uber.org/zap"}}},
		{{ImportPath: modulePath + "/internal/module/example", Imports: []string{"github.com/rabbitmq/amqp091-go"}}},
	} {
		if err := validatePackageGraph(fixture); err == nil {
			t.Fatalf("invalid fixture %#v passed", fixture)
		}
	}
}

func TestLoggingSourceRulesRejectProductionBypasses(t *testing.T) {
	legalRoot := t.TempDir()
	writeModuleBoundaryFixture(t, legalRoot, "pkg/logger/adapter.go", `package logger
import "go.uber.org/zap"
func build() *zap.Logger { return zap.NewNop() }
`)
	if err := validateLoggingSourceOwnership(legalRoot); err != nil {
		t.Fatalf("legal logger implementation fixture error = %v", err)
	}

	noopRoot := t.TempDir()
	writeModuleBoundaryFixture(t, noopRoot, "internal/example/example.go", `package example
import projectlogger "github.com/rin721/go-scaffold-template/pkg/logger"
func build() projectlogger.Logger { return projectlogger.Noop() }
`)
	if err := validateLoggingSourceOwnership(noopRoot); err == nil {
		t.Fatal("production logger.Noop fixture passed")
	}

	zapRoot := t.TempDir()
	writeModuleBoundaryFixture(t, zapRoot, "internal/example/example.go", `package example
import "go.uber.org/zap"
func build() *zap.Logger { return zap.NewNop() }
`)
	if err := validateLoggingSourceOwnership(zapRoot); err == nil {
		t.Fatal("direct zap fixture passed")
	}

	globalRoot := t.TempDir()
	writeModuleBoundaryFixture(t, globalRoot, "internal/example/example.go", `package example
import "log"
func report() { log.Print("bypass") }
`)
	if err := validateLoggingSourceOwnership(globalRoot); err == nil {
		t.Fatal("global standard logger fixture passed")
	}

	rawErrorRoot := t.TempDir()
	writeModuleBoundaryFixture(t, rawErrorRoot, "internal/example/example.go", `package example
import (
	"errors"
	projectlogger "github.com/rin721/go-scaffold-template/pkg/logger"
)
func report(log projectlogger.Logger) {
	err := errors.New("unsafe detail")
	log.Warn("failed", projectlogger.String("error", err.Error()))
}
`)
	if err := validateLoggingSourceOwnership(rawErrorRoot); err == nil {
		t.Fatal("raw error string logging fixture passed")
	}
}

func TestModuleExportRulesAcceptPrivateImplementationAndRejectLeaks(t *testing.T) {
	legalRoot := t.TempDir()
	writeModuleBoundaryFixture(t, legalRoot, "internal/module/example/adapter/sdk/adapter.go", `package sdk
import third "github.com/example/sdk"
type Adapter struct { client *third.Client }
type Config struct{}
func New(Config) (*Adapter, error) { return nil, nil }
`)
	if err := validateModuleExportBoundaries(legalRoot); err != nil {
		t.Fatalf("legal module Adapter fixture error = %v", err)
	}

	thirdPartyLeak := t.TempDir()
	writeModuleBoundaryFixture(t, thirdPartyLeak, "internal/module/example/adapter/sdk/adapter.go", `package sdk
import third "github.com/example/sdk"
func Client() *third.Client { return nil }
`)
	if err := validateModuleExportBoundaries(thirdPartyLeak); err == nil {
		t.Fatal("third-party exported selector fixture passed")
	}

	adapterLeak := t.TempDir()
	writeModuleBoundaryFixture(t, adapterLeak, "internal/module/example/module.go", `package example
import sdk "github.com/rin721/go-scaffold-template/internal/module/example/adapter/sdk"
func Adapter() *sdk.Adapter { return nil }
`)
	if err := validateModuleExportBoundaries(adapterLeak); err == nil {
		t.Fatal("module root concrete Adapter fixture passed")
	}
}

func writeModuleBoundaryFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func validatePackageGraph(graph []packageNode) error {
	for _, node := range graph {
		for _, imported := range node.Imports {
			if imported == "github.com/rabbitmq/amqp091-go" &&
				node.ImportPath != modulePath+"/internal/kernel/app/messaging/rabbitmq" {
				return fmt.Errorf("package %s bypasses the RabbitMQ messaging adapter", node.ImportPath)
			}
			if strings.HasPrefix(imported, "go.uber.org/zap") && node.ImportPath != modulePath+"/pkg/logger" {
				return fmt.Errorf("package %s bypasses pkg/logger through %s", node.ImportPath, imported)
			}
			if sourceOwner, sourceIsModule := applicationModuleOwner(node.ImportPath); sourceIsModule &&
				thirdPartyImport(imported) && !moduleAdapterPackage(node.ImportPath) {
				return fmt.Errorf("application module %s package %s imports third-party package outside its adapter: %s", sourceOwner, node.ImportPath, imported)
			}
			if node.ImportPath == modulePath+"/internal/composition" && moduleAdapterPackage(imported) {
				return fmt.Errorf("application composition imports module adapter %s", imported)
			}
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
				(!sourceIsModule || sourceOwner != importedOwner) &&
				!contractGeneratorImportsModuleContract(node.ImportPath, imported) {
				return fmt.Errorf("package %s imports application module %s outside the composition root", node.ImportPath, imported)
			}
			if sourceIsModule && (imported == modulePath+"/internal/composition" ||
				imported == modulePath+"/internal/kernel/composition") {
				return fmt.Errorf("application module %s imports upper owner %s", sourceOwner, imported)
			}
			if moduleCorePackage(node.ImportPath) && forbiddenModuleCoreImport(imported) {
				return fmt.Errorf("module core package %s imports forbidden boundary %s", node.ImportPath, imported)
			}
			if moduleHandlerPackage(node.ImportPath) && (moduleBindingImport(imported) ||
				strings.HasPrefix(imported, modulePath+"/internal/transport/")) {
				return fmt.Errorf("module handler %s imports framework/binding boundary %s", node.ImportPath, imported)
			}
			if moduleHandlerPackage(node.ImportPath) && forbiddenModuleHTTPBindingImport(imported) {
				return fmt.Errorf("module handler %s imports application route infrastructure %s", node.ImportPath, imported)
			}
		}
	}
	return nil
}

func validateLoggingSourceOwnership(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".gen.go") {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("resolve logging source %s: %w", path, err)
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if len(parts) == 0 || parts[0] != "cmd" && parts[0] != "internal" && parts[0] != "pkg" {
			return nil
		}
		loggerImplementation := len(parts) >= 2 && parts[0] == "pkg" && parts[1] == "logger"
		fileset := token.NewFileSet()
		parsed, err := parser.ParseFile(fileset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse logging source %s: %w", path, err)
		}
		imports := make(map[string]string, len(parsed.Imports))
		for _, spec := range parsed.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("decode logging import in %s: %w", path, err)
			}
			name := filepath.Base(importPath)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			imports[name] = importPath
			if strings.HasPrefix(importPath, "go.uber.org/zap") && !loggerImplementation {
				return fmt.Errorf("production source %s bypasses pkg/logger through %s", path, importPath)
			}
			if importPath == "log" && (parts[0] == "cmd" || parts[0] == "internal") {
				return fmt.Errorf("production source %s imports global standard logger", path)
			}
		}
		var violation error
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if selector.Sel.Name == "Noop" && selectorImportPath(selector, imports) == modulePath+"/pkg/logger" {
				position := fileset.Position(selector.Pos())
				violation = fmt.Errorf("production source %s:%d uses logger.Noop", path, position.Line)
				return false
			}
			if logsRawErrorString(call, selector, imports) {
				position := fileset.Position(selector.Pos())
				violation = fmt.Errorf("production source %s:%d logs raw err.Error() through logger.String(\"error\", ...)", path, position.Line)
				return false
			}
			return true
		})
		return violation
	})
}

func logsRawErrorString(call *ast.CallExpr, selector *ast.SelectorExpr, imports map[string]string) bool {
	if selector.Sel.Name != "String" || selectorImportPath(selector, imports) != modulePath+"/pkg/logger" || len(call.Args) < 2 {
		return false
	}
	key, ok := call.Args[0].(*ast.BasicLit)
	if !ok || key.Kind != token.STRING {
		return false
	}
	unquoted, err := strconv.Unquote(key.Value)
	if err != nil || unquoted != "error" {
		return false
	}
	valueCall, ok := call.Args[1].(*ast.CallExpr)
	if !ok || len(valueCall.Args) != 0 {
		return false
	}
	valueSelector, ok := valueCall.Fun.(*ast.SelectorExpr)
	return ok && valueSelector.Sel.Name == "Error"
}

// validateKernelAppConfigOwnership 防止 application 层组件默认配置整体复用 pkg/* 默认配置。
// 032 门禁：kernel/app/* 组件的 default 配置来源不得直接调用 pkg/*.DefaultConfig()；
// 允许引用 pkg/* 的基础默认常量（如 redisstore.DefaultTagPrefix）作为未声明时的回退默认。
func validateKernelAppConfigOwnership(root string) error {
	kernelAppRoot := filepath.Join(root, "internal", "kernel", "app")
	return filepath.WalkDir(kernelAppRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read kernel app source %s: %w", path, err)
		}
		fileset := token.NewFileSet()
		parsed, err := parser.ParseFile(fileset, path, source, 0)
		if err != nil {
			return fmt.Errorf("parse kernel app source %s: %w", path, err)
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
		}
		var violation error
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "DefaultConfig" {
				return true
			}
			importPath := selectorImportPath(selector, imports)
			if !strings.HasPrefix(importPath, modulePath+"/pkg/") {
				return true
			}
			position := fileset.Position(selector.Pos())
			violation = fmt.Errorf("kernel app default config %s:%d reuses pkg default %s.DefaultConfig", path, position.Line, filepath.Base(importPath))
			return false
		})
		return violation
	})
}

// validateCompositionOwnership 防止 composition 内部文件反向 import 具体业务模块的 HTTP 契约包
// （如 ops.go/service.go 直接读 todohttp.ModuleContract）。契约知识只经装配汇总点
// applicationHTTPModules() 消费（034 WIRE-001）。
func validateCompositionOwnership(root string) error {
	compositionRoot := filepath.Join(root, "internal", "composition")
	return filepath.WalkDir(compositionRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
			return walkErr
		}
		// 只约束必须经 applicationHTTPModules() 汇总消费契约的装配文件（ops.go/service.go）；
		// http_api.go 负责对具体模块运行时 handler 装箱并对齐唯一契约来源，属合法装配点。
		if base := filepath.Base(path); base != "ops.go" && base != "service.go" {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read composition source %s: %w", path, err)
		}
		fileset := token.NewFileSet()
		parsed, err := parser.ParseFile(fileset, path, source, 0)
		if err != nil {
			return fmt.Errorf("parse composition source %s: %w", path, err)
		}
		for _, spec := range parsed.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("decode import in %s: %w", path, err)
			}
			prefix := modulePath + "/internal/module/"
			if strings.HasPrefix(importPath, prefix) && strings.Contains(importPath, "/todo/binding/http") {
				return fmt.Errorf("composition source %s imports module HTTP binding contract %s; use applicationHTTPModules()", path, importPath)
			}
		}
		return nil
	})
}

func validateModuleExportBoundaries(root string) error {
	moduleRoot := filepath.Join(root, "internal", "module")
	return filepath.WalkDir(moduleRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		fileset := token.NewFileSet()
		parsed, err := parser.ParseFile(fileset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse module boundary source %s: %w", path, err)
		}
		imports := make(map[string]string, len(parsed.Imports))
		for _, spec := range parsed.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("decode module import in %s: %w", path, err)
			}
			name := filepath.Base(importPath)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			imports[name] = importPath
		}
		for _, declaration := range parsed.Decls {
			for _, exported := range exportedContractNodes(declaration) {
				var violation error
				ast.Inspect(exported, func(node ast.Node) bool {
					selector, ok := node.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					importPath := selectorImportPath(selector, imports)
					if importPath == "" {
						return true
					}
					if thirdPartyImport(importPath) || moduleAdapterPackage(importPath) {
						position := fileset.Position(selector.Pos())
						violation = fmt.Errorf("module exported contract %s:%d leaks implementation package %s", path, position.Line, importPath)
						return false
					}
					return true
				})
				if violation != nil {
					return violation
				}
			}
		}
		return nil
	})
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

func validateHTTPSourceOwnership(root string) error {
	type assertion struct {
		path string
		line int
	}
	var moduleRouteCalls []assertion
	var routeBindings []assertion
	var dispatcherAssertions []assertion
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
			if moduleHTTPBindingSource(root, path) {
				if forbiddenModuleHTTPBindingImport(importPath) {
					return fmt.Errorf("module HTTP binding %s imports application route infrastructure %s", path, importPath)
				}
				if importPath == modulePath+"/internal/transport/http/api" {
					return fmt.Errorf("module HTTP binding %s imports generated contract package %s", path, importPath)
				}
			}
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			switch current := node.(type) {
			case *ast.FuncDecl:
				if current.Name.Name == "NewRouteBinding" {
					position := fileset.Position(current.Pos())
					routeBindings = append(routeBindings, assertion{path: path, line: position.Line})
				}
			case *ast.CallExpr:
				selector, ok := current.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if moduleHTTPBindingSource(root, path) && selectorImportPath(selector, imports) == modulePath+"/internal/transport/http/api" {
					position := fileset.Position(selector.Pos())
					moduleRouteCalls = append(moduleRouteCalls, assertion{path: path, line: position.Line})
				}
			case *ast.ValueSpec:
				if len(current.Names) != 1 || current.Names[0].Name != "_" {
					return true
				}
				selector, ok := current.Type.(*ast.SelectorExpr)
				if ok && selector.Sel.Name == "Dispatcher" {
					position := fileset.Position(selector.Pos())
					dispatcherAssertions = append(dispatcherAssertions, assertion{path: path, line: position.Line})
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
	if len(routeBindings) != 1 || !pathWithin(root, routeBindings[0].path, "internal", "transport", "http") {
		return fmt.Errorf("contract NewRouteBinding must exist once under internal/transport/http, got %#v", routeBindings)
	}
	if len(dispatcherAssertions) != 1 || !pathWithin(root, dispatcherAssertions[0].path, "internal", "composition") {
		return fmt.Errorf("dispatcher assertion must exist once under internal/composition, got %#v", dispatcherAssertions)
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

// contractGeneratorImportsModuleContract 允许 030 生成器 internal/tools/contract-gen 在构建期
// import 模块的 binding/http 契约包（只生产生成物，不进入 production 运行图）。
func contractGeneratorImportsModuleContract(importer, imported string) bool {
	const generator = "/internal/tools/contract-gen"
	if !strings.HasSuffix(importer, generator) {
		return false
	}
	return strings.HasPrefix(imported, modulePath+"/internal/module/") &&
		strings.Contains(imported, "/binding/http")
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

func moduleAdapterPackage(importPath string) bool {
	prefix := modulePath + "/internal/module/"
	if !strings.HasPrefix(importPath, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(importPath, prefix), "/")
	return len(parts) >= 3 && parts[1] == "adapter"
}

func thirdPartyImport(importPath string) bool {
	if strings.HasPrefix(importPath, modulePath+"/") {
		return false
	}
	first, _, _ := strings.Cut(importPath, "/")
	return strings.Contains(first, ".")
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

// moduleHandlerPackage 识别模块顶层 handler 层（internal/module/<name>/handler）。
func moduleHandlerPackage(importPath string) bool {
	prefix := modulePath + "/internal/module/"
	if !strings.HasPrefix(importPath, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(importPath, prefix), "/")
	return len(parts) >= 2 && parts[1] == "handler"
}

// moduleBindingImport 判断 imported 是否属于同一模块的 binding/**。
func moduleBindingImport(imported string) bool {
	prefix := modulePath + "/internal/module/"
	if !strings.HasPrefix(imported, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(imported, prefix), "/")
	return len(parts) >= 3 && parts[1] == "binding"
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
