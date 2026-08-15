// Command openapi-inventory 从唯一 OpenAPI authority 生成 operation inventory。
package main

import (
	"context"
	"flag"
	"fmt"
	"go/format"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

var operationIDPattern = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)

type operation struct {
	ID         string
	StrictName string
	Method     string
	Path       string
	Policy     string
	Scope      string
	Action     string
}

type policyExtension struct {
	Mode   string
	Scope  string
	Action string
}

func main() {
	input := flag.String("input", "", "OpenAPI 文件路径")
	output := flag.String("output", "", "生成的 Go 文件路径")
	packageName := flag.String("package", "api", "生成文件 package")
	flag.Parse()
	if err := run(*input, *output, *packageName); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input, output, packageName string) error {
	if input == "" || output == "" || packageName == "" {
		return fmt.Errorf("input, output and package are required")
	}
	loader := openapi3.NewLoader()
	specification, err := loader.LoadFromFile(input)
	if err != nil {
		return fmt.Errorf("load OpenAPI authority: %w", err)
	}
	if err := specification.Validate(context.Background()); err != nil {
		return fmt.Errorf("validate OpenAPI authority: %w", err)
	}
	operations, err := collectOperations(specification)
	if err != nil {
		return err
	}
	source, err := render(packageName, operations)
	if err != nil {
		return err
	}
	if err := os.WriteFile(output, source, 0o644); err != nil {
		return fmt.Errorf("write operation inventory: %w", err)
	}
	return nil
}

func collectOperations(specification *openapi3.T) ([]operation, error) {
	if specification == nil || specification.Paths == nil {
		return nil, fmt.Errorf("OpenAPI paths are required")
	}
	seen := make(map[string]string)
	result := make([]operation, 0)
	for path, item := range specification.Paths.Map() {
		for method, value := range item.Operations() {
			if !operationIDPattern.MatchString(value.OperationID) {
				return nil, fmt.Errorf("%s %s has invalid operationId %q", method, path, value.OperationID)
			}
			if previous, exists := seen[value.OperationID]; exists {
				return nil, fmt.Errorf("operationId %q is shared by %s and %s %s", value.OperationID, previous, method, path)
			}
			if value.Security == nil {
				return nil, fmt.Errorf("%s %s must declare operation security", method, path)
			}
			policy, err := decodePolicy(value.Extensions["x-policy"])
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", method, path, err)
			}
			if err := validateSecurity(value.Security, policy); err != nil {
				return nil, fmt.Errorf("%s %s: %w", method, path, err)
			}
			if value.Responses == nil || value.Responses.Default() == nil {
				return nil, fmt.Errorf("%s %s must declare a default Problem response", method, path)
			}
			seen[value.OperationID] = strings.ToUpper(method) + " " + path
			result = append(result, operation{
				ID: value.OperationID, StrictName: upperFirst(value.OperationID),
				Method: strings.ToUpper(method), Path: path, Policy: policy.Mode,
				Scope: policy.Scope, Action: policy.Action,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	if len(result) == 0 {
		return nil, fmt.Errorf("OpenAPI authority has no operations")
	}
	return result, nil
}

func decodePolicy(raw any) (policyExtension, error) {
	values, ok := raw.(map[string]any)
	if !ok {
		return policyExtension{}, fmt.Errorf("x-policy must be an object")
	}
	if len(values) != 3 {
		return policyExtension{}, fmt.Errorf("x-policy must declare exactly mode, scope and action")
	}
	policy := policyExtension{}
	var fields = []struct {
		name   string
		target *string
	}{{"mode", &policy.Mode}, {"scope", &policy.Scope}, {"action", &policy.Action}}
	for _, field := range fields {
		value, exists := values[field.name]
		text, valid := value.(string)
		if !exists || !valid {
			return policyExtension{}, fmt.Errorf("x-policy %s must be a string", field.name)
		}
		*field.target = strings.TrimSpace(text)
	}
	if policy.Mode != "public" && policy.Mode != "protected" {
		return policyExtension{}, fmt.Errorf("x-policy mode must be public or protected")
	}
	if policy.Mode == "public" && (policy.Scope != "" || policy.Action != "") {
		return policyExtension{}, fmt.Errorf("public x-policy cannot declare scope or action")
	}
	if policy.Mode == "protected" && (policy.Scope == "" || policy.Action == "") {
		return policyExtension{}, fmt.Errorf("protected x-policy requires scope and action")
	}
	return policy, nil
}

func validateSecurity(requirements *openapi3.SecurityRequirements, policy policyExtension) error {
	if requirements == nil {
		return fmt.Errorf("operation security is required")
	}
	if policy.Mode == "public" {
		if len(*requirements) != 0 {
			return fmt.Errorf("public operation security must be empty")
		}
		return nil
	}
	if len(*requirements) != 1 {
		return fmt.Errorf("protected operation must require exactly bearerAuth")
	}
	scopes, exists := (*requirements)[0]["bearerAuth"]
	if !exists || len((*requirements)[0]) != 1 || len(scopes) != 0 {
		return fmt.Errorf("protected operation must require bearerAuth without OAuth scopes")
	}
	return nil
}

func render(packageName string, operations []operation) ([]byte, error) {
	var source strings.Builder
	fmt.Fprintf(&source, "// Code generated by openapi-inventory; DO NOT EDIT.\n\npackage %s\n\n", packageName)
	source.WriteString("// OperationID 是公开契约中的稳定 operationId。\ntype OperationID string\n\n")
	source.WriteString("// Operation 描述公开路由的低基数身份与策略。\ntype Operation struct {\n\tID OperationID\n\tMethod string\n\tPath string\n\tPolicy string\n\tScope string\n\tAction string\n}\n\n")
	source.WriteString("const (\n")
	for _, item := range operations {
		fmt.Fprintf(&source, "\tOperation%s OperationID = %s\n", upperFirst(item.ID), strconv.Quote(item.ID))
	}
	source.WriteString(")\n\nvar operationInventory = [...]Operation{\n")
	for _, item := range operations {
		fmt.Fprintf(&source, "\t{ID: Operation%s, Method: %s, Path: %s, Policy: %s, Scope: %s, Action: %s},\n",
			upperFirst(item.ID), strconv.Quote(item.Method), strconv.Quote(item.Path), strconv.Quote(item.Policy), strconv.Quote(item.Scope), strconv.Quote(item.Action))
	}
	source.WriteString("}\n\n")
	source.WriteString("// Operations 返回与生成代码解耦的 inventory 副本。\nfunc Operations() []Operation {\n\treturn append([]Operation(nil), operationInventory[:]...)\n}\n\n")
	source.WriteString("// OperationForStrictName 把 strict middleware 名称映射回原始 operationId。\nfunc OperationForStrictName(name string) (Operation, bool) {\n\tswitch name {\n")
	for _, item := range operations {
		fmt.Fprintf(&source, "\tcase %s:\n\t\treturn Operation{ID: Operation%s, Method: %s, Path: %s, Policy: %s, Scope: %s, Action: %s}, true\n",
			strconv.Quote(item.StrictName), upperFirst(item.ID), strconv.Quote(item.Method), strconv.Quote(item.Path), strconv.Quote(item.Policy), strconv.Quote(item.Scope), strconv.Quote(item.Action))
	}
	source.WriteString("\tdefault:\n\t\treturn Operation{}, false\n\t}\n}\n")
	formatted, err := format.Source([]byte(source.String()))
	if err != nil {
		return nil, fmt.Errorf("format operation inventory: %w", err)
	}
	return formatted, nil
}

func upperFirst(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
