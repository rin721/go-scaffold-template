package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

// Format 是默认配置文件的编码格式。
type Format string

const (
	// FormatYAML 表示 YAML 配置格式。
	FormatYAML Format = "yaml"
	// FormatJSON 表示 JSON 配置格式。
	FormatJSON Format = "json"
)

// GenerateRequest 描述一次默认配置文件生成请求。
type GenerateRequest struct {
	Path  string
	Force bool
}

// GenerateResult 描述成功生成的目标和参与能力。
type GenerateResult struct {
	Path       string
	Format     Format
	Replaced   bool
	SectionIDs []string
}

// DefaultManager 聚合当前 composition 显式绑定的全部默认配置契约。
type DefaultManager interface {
	Generate(context.Context, GenerateRequest) (GenerateResult, error)
}

type defaultManager struct {
	bindings []Binding
}

// NewDefaultManager 校验并冻结按 composition 登记顺序传入的 Binding。
func NewDefaultManager(bindings ...Binding) (DefaultManager, error) {
	copied := append([]Binding(nil), bindings...)
	if err := ValidateBindings(copied...); err != nil {
		return nil, err
	}
	return &defaultManager{bindings: copied}, nil
}

// ValidateBindings 校验并冻结前置所需的 section ID、path 和契约唯一性。
func ValidateBindings(copied ...Binding) error {
	return validateBindings(true, copied...)
}

// ValidateRegistrations 校验运行期 section 契约；不要求该 section 贡献生成默认值。
func ValidateRegistrations(copied ...Binding) error {
	return validateBindings(false, copied...)
}

func validateBindings(requireDefaults bool, copied ...Binding) error {
	paths := make([][]string, 0, len(copied))
	ids := make(map[string]struct{}, len(copied))
	for index, binding := range copied {
		if strings.TrimSpace(binding.CapabilityID) == "" {
			return fmt.Errorf("default binding %d capability id is required", index)
		}
		if _, exists := ids[binding.CapabilityID]; exists {
			return fmt.Errorf("default binding capability %s is duplicated", binding.CapabilityID)
		}
		ids[binding.CapabilityID] = struct{}{}
		segments, err := splitConfigPath(binding.ConfigPath)
		if err != nil {
			return fmt.Errorf("default binding capability %s: %w", binding.CapabilityID, err)
		}
		if requireDefaults && isNilContract(binding.Contract) {
			return fmt.Errorf("default binding capability %s contract is nil", binding.CapabilityID)
		}
		if binding.Validate == nil {
			return fmt.Errorf("default binding capability %s validator is nil", binding.CapabilityID)
		}
		for previousIndex, previous := range paths {
			if pathsOverlap(previous, segments) {
				return fmt.Errorf("default binding paths %q and %q overlap", copied[previousIndex].ConfigPath, binding.ConfigPath)
			}
		}
		paths = append(paths, segments)
	}
	return nil
}

// ValidateCandidate 对完整候选执行未知 section 门禁和每个 owner 的严格校验。
func ValidateCandidate(snapshot Snapshot, bindings ...Binding) error {
	knownRoots := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		root := binding.ConfigPath
		if separator := strings.IndexByte(root, '.'); separator >= 0 {
			root = root[:separator]
		}
		knownRoots[root] = struct{}{}
		if binding.Validate == nil {
			return fmt.Errorf("config section %s validator is nil", binding.CapabilityID)
		}
		if err := binding.Validate(snapshot); err != nil {
			return fmt.Errorf("validate config section %s at %s: %w", binding.CapabilityID, binding.ConfigPath, err)
		}
	}
	for root := range snapshot.Data() {
		if _, exists := knownRoots[root]; !exists {
			return fmt.Errorf("unknown config section %q", root)
		}
	}
	return nil
}

func (m *defaultManager) Generate(ctx context.Context, request GenerateRequest) (GenerateResult, error) {
	if ctx == nil {
		return GenerateResult{}, fmt.Errorf("default configuration context is nil")
	}
	target, format, err := resolveTarget(request.Path)
	if err != nil {
		return GenerateResult{}, err
	}

	root := &documentNode{}
	sectionIDs := make([]string, 0, len(m.bindings))
	for _, binding := range m.bindings {
		if err := ctx.Err(); err != nil {
			return GenerateResult{}, fmt.Errorf("generate default configuration: %w", err)
		}
		object, control, contractErr := binding.Contract.Defaults(ctx)
		switch control {
		case Continue:
			if contractErr != nil {
				return GenerateResult{}, fmt.Errorf("defaults for capability %s: %w", binding.CapabilityID, contractErr)
			}
		case Abort:
			if contractErr == nil {
				return GenerateResult{}, fmt.Errorf("defaults for capability %s returned Abort without a cause", binding.CapabilityID)
			}
			return GenerateResult{}, &AbortedError{CapabilityID: binding.CapabilityID, Cause: contractErr}
		default:
			return GenerateResult{}, fmt.Errorf("defaults for capability %s returned unknown control %d", binding.CapabilityID, control)
		}
		if err := validateObject(object); err != nil {
			return GenerateResult{}, fmt.Errorf("defaults for capability %s: %w", binding.CapabilityID, err)
		}
		root.insert(strings.Split(binding.ConfigPath, "."), object)
		sectionIDs = append(sectionIDs, binding.CapabilityID)
	}
	candidate, err := newSnapshot(objectMap(root.object()), []string{"generated-defaults"})
	if err != nil {
		return GenerateResult{}, fmt.Errorf("build default configuration candidate: %w", err)
	}
	for _, binding := range m.bindings {
		if err := binding.Validate(candidate); err != nil {
			return GenerateResult{}, fmt.Errorf("validate defaults for capability %s: %w", binding.CapabilityID, err)
		}
	}

	payload, err := encodeDefaultDocument(root.object(), format)
	if err != nil {
		return GenerateResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return GenerateResult{}, fmt.Errorf("write default configuration: %w", err)
	}
	replaced := request.Force && regularFileExists(target)
	if err := writeDefaultFile(target, payload, request.Force); err != nil {
		return GenerateResult{}, err
	}
	return GenerateResult{Path: target, Format: format, Replaced: replaced, SectionIDs: sectionIDs}, nil
}

func objectMap(object Object) map[string]any {
	values := make(map[string]any, len(object))
	for _, field := range object {
		values[field.Name] = defaultValueData(field.Value)
	}
	return values
}

func defaultValueData(value Value) any {
	concrete := value.(configValue)
	switch concrete.kind {
	case valueString, valueDuration:
		return concrete.text
	case valueBool:
		return concrete.boolean
	case valueNumber:
		if strings.ContainsAny(concrete.text, ".eE") {
			parsed, _ := strconv.ParseFloat(concrete.text, 64)
			return parsed
		}
		parsed, _ := strconv.ParseInt(concrete.text, 10, 64)
		return parsed
	case valueNull:
		return nil
	case valueObject:
		return objectMap(concrete.object)
	case valueList:
		values := make([]any, len(concrete.elements))
		for index, element := range concrete.elements {
			values[index] = defaultValueData(element)
		}
		return values
	default:
		panic("validated default value has unknown kind")
	}
}

func regularFileExists(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func resolveTarget(path string) (string, Format, error) {
	if strings.TrimSpace(path) == "" {
		return "", "", fmt.Errorf("default configuration path is required")
	}
	cleaned := filepath.Clean(path)
	absolute, err := filepath.Abs(cleaned)
	if err != nil {
		return "", "", fmt.Errorf("resolve default configuration target: %w", err)
	}
	switch strings.ToLower(filepath.Ext(cleaned)) {
	case ".yaml", ".yml":
		return absolute, FormatYAML, nil
	case ".json":
		return absolute, FormatJSON, nil
	default:
		return "", "", fmt.Errorf("unsupported default configuration extension %q", filepath.Ext(cleaned))
	}
}

func splitConfigPath(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("config path is required")
	}
	segments := strings.Split(path, ".")
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			return nil, fmt.Errorf("config path %q contains an empty segment", path)
		}
	}
	return segments, nil
}

func pathsOverlap(left, right []string) bool {
	limit := min(len(left), len(right))
	for index := range limit {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func isNilContract(contract DefaultContract) bool {
	if contract == nil {
		return true
	}
	value := reflect.ValueOf(contract)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

type documentNode struct {
	children []documentChild
	leaf     Object
}

type documentChild struct {
	name string
	node *documentNode
}

func (n *documentNode) insert(path []string, object Object) {
	if len(path) == 0 {
		n.leaf = append(Object(nil), object...)
		return
	}
	for _, child := range n.children {
		if child.name == path[0] {
			child.node.insert(path[1:], object)
			return
		}
	}
	child := documentChild{name: path[0], node: &documentNode{}}
	n.children = append(n.children, child)
	child.node.insert(path[1:], object)
}

func (n *documentNode) object() Object {
	if n.leaf != nil {
		return append(Object(nil), n.leaf...)
	}
	object := make(Object, 0, len(n.children))
	for _, child := range n.children {
		object = append(object, FieldOf(child.name, ObjectValue(child.node.object())))
	}
	return object
}
