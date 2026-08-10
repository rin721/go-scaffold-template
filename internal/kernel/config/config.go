// Package config 负责为 kernel 加载不可变配置快照。
package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"gopkg.in/yaml.v3"

	"github.com/rin721/go-scaffold2/pkg/secrets"
)

// Source 表示一个配置来源。
type Source interface {
	Name() string
	Load(context.Context) (map[string]any, error)
}

// Loader 按顺序加载配置来源，后加载的来源覆盖先加载的来源。
type Loader struct {
	sources []Source
}

// New 创建配置加载器。
func New(sources ...Source) *Loader {
	return &Loader{sources: append([]Source(nil), sources...)}
}

// Load 加载所有来源并生成不可变快照。
func (l *Loader) Load(ctx context.Context) (Snapshot, error) {
	if l == nil {
		return Snapshot{}, fmt.Errorf("config loader is nil")
	}
	if ctx == nil {
		return Snapshot{}, fmt.Errorf("config load context is nil")
	}
	merged := map[string]any{}
	var provenance []string
	for _, source := range l.sources {
		if source == nil {
			continue
		}
		values, err := source.Load(ctx)
		if err != nil {
			return Snapshot{}, fmt.Errorf("load config source %s: %w", source.Name(), err)
		}
		mergeMap(merged, normalizeMap(values))
		provenance = append(provenance, source.Name())
	}
	return newSnapshot(merged, provenance)
}

// FilePaths 返回所有 FileSource 对应的文件路径并去重。
func (l *Loader) FilePaths() []string {
	if l == nil {
		return nil
	}
	seen := make(map[string]struct{})
	paths := make([]string, 0)
	for _, source := range l.sources {
		file, ok := source.(fileSource)
		if !ok {
			continue
		}
		path := filepath.Clean(file.path)
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

// Snapshot 是一次配置加载的不可变结果。
type Snapshot struct {
	values     map[string]any
	redacted   map[string]any
	digest     string
	provenance []string
}

func newSnapshot(values map[string]any, provenance []string) (Snapshot, error) {
	copied := copyMap(values)
	payload, err := json.Marshal(copied)
	if err != nil {
		return Snapshot{}, fmt.Errorf("digest config snapshot: %w", err)
	}
	sum := sha256.Sum256(payload)
	return Snapshot{
		values:     copied,
		redacted:   secrets.RedactMap(copied),
		digest:     hex.EncodeToString(sum[:]),
		provenance: append([]string(nil), provenance...),
	}, nil
}

// Digest 返回快照内容摘要。
func (s Snapshot) Digest() string {
	return s.digest
}

// Data 返回配置深拷贝。
func (s Snapshot) Data() map[string]any {
	return copyMap(s.values)
}

// Redacted 返回脱敏后的配置深拷贝。
func (s Snapshot) Redacted() map[string]any {
	return copyMap(s.redacted)
}

// Provenance 返回参与本次快照的来源名称。
func (s Snapshot) Provenance() []string {
	return append([]string(nil), s.provenance...)
}

// Value 按点号路径读取配置值。
func (s Snapshot) Value(path string) (any, bool) {
	parts := strings.Split(strings.Trim(path, "."), ".")
	var current any = s.values
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
		values, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = values[part]
		if !ok {
			return nil, false
		}
	}
	return copyValue(current), true
}

// Section 返回指定路径对应的独立快照；缺失路径按空配置段处理。
func (s Snapshot) Section(path string) (Snapshot, error) {
	trimmed := strings.Trim(path, ".")
	if trimmed == "" {
		return newSnapshot(s.values, s.provenance)
	}
	value, exists := s.Value(trimmed)
	if !exists {
		return newSnapshot(map[string]any{}, s.provenance)
	}
	values, ok := value.(map[string]any)
	if !ok {
		return Snapshot{}, fmt.Errorf("config section %s is not an object", trimmed)
	}
	return newSnapshot(values, s.provenance)
}

// SectionDigest 返回指定配置段的内容摘要。
func (s Snapshot) SectionDigest(path string) (string, error) {
	section, err := s.Section(path)
	if err != nil {
		return "", err
	}
	return section.Digest(), nil
}

// Decode 将整个快照解码到调用方结构体。
func (s Snapshot) Decode(target any) error {
	if target == nil {
		return fmt.Errorf("config decode target is nil")
	}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          "mapstructure",
		Result:           target,
		WeaklyTypedInput: true,
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
	})
	if err != nil {
		return fmt.Errorf("create config decoder: %w", err)
	}
	if err := decoder.Decode(s.values); err != nil {
		return fmt.Errorf("decode config snapshot: %w", err)
	}
	return nil
}

// DecodeSection 将指定配置段解码到已经预填默认值的目标结构体。
func (s Snapshot) DecodeSection(path string, target any) error {
	section, err := s.Section(path)
	if err != nil {
		return err
	}
	if err := section.Decode(target); err != nil {
		return fmt.Errorf("decode config section %s: %w", path, err)
	}
	return nil
}

// MapSource 从内存 map 提供配置，适合默认值和测试。
func MapSource(name string, values map[string]any) Source {
	return mapSource{name: name, values: copyMap(values)}
}

type mapSource struct {
	name   string
	values map[string]any
}

func (s mapSource) Name() string {
	if s.name == "" {
		return "map"
	}
	return s.name
}

func (s mapSource) Load(context.Context) (map[string]any, error) {
	return copyMap(s.values), nil
}

// FileSource 从 JSON/YAML 文件加载配置。
func FileSource(path string) Source {
	return fileSource{path: path}
}

type fileSource struct {
	path string
}

func (s fileSource) Name() string {
	return "file:" + s.path
}

func (s fileSource) Load(context.Context) (map[string]any, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var values map[string]any
	switch strings.ToLower(filepath.Ext(s.path)) {
	case ".json":
		err = json.Unmarshal(data, &values)
	case ".yaml", ".yml", "":
		err = yaml.Unmarshal(data, &values)
	default:
		return nil, fmt.Errorf("unsupported config file extension %q", filepath.Ext(s.path))
	}
	if err != nil {
		return nil, err
	}
	return normalizeMap(values), nil
}

// EnvSource 从环境变量读取配置，双下划线表示嵌套路径。
func EnvSource(prefix string) Source {
	return envSource{prefix: prefix}
}

type envSource struct {
	prefix string
}

func (s envSource) Name() string {
	return "env:" + s.prefix
}

func (s envSource) Load(context.Context) (map[string]any, error) {
	out := map[string]any{}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok || !strings.HasPrefix(key, s.prefix) {
			continue
		}
		trimmed := strings.TrimPrefix(key, s.prefix)
		if trimmed == "" {
			continue
		}
		parts := strings.Split(strings.ToLower(trimmed), "__")
		setNested(out, parts, value)
	}
	return out, nil
}

func mergeMap(dst map[string]any, src map[string]any) {
	for key, value := range src {
		key = matchingKey(dst, key)
		if nested, ok := value.(map[string]any); ok {
			if existing, ok := dst[key].(map[string]any); ok {
				mergeMap(existing, nested)
				continue
			}
		}
		dst[key] = copyValue(value)
	}
}

func matchingKey(values map[string]any, candidate string) string {
	for existing := range values {
		if strings.EqualFold(existing, candidate) {
			return existing
		}
	}
	return candidate
}

func setNested(values map[string]any, parts []string, value any) {
	if len(parts) == 0 {
		return
	}
	if len(parts) == 1 {
		values[parts[0]] = value
		return
	}
	next, _ := values[parts[0]].(map[string]any)
	if next == nil {
		next = map[string]any{}
		values[parts[0]] = next
	}
	setNested(next, parts[1:], value)
}

func normalizeMap(values map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = normalizeValue(value)
	}
	return out
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return normalizeMap(typed)
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, nestedValue := range typed {
			out[fmt.Sprint(key)] = normalizeValue(nestedValue)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, nestedValue := range typed {
			out[index] = normalizeValue(nestedValue)
		}
		return out
	default:
		return value
	}
}

func copyMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = copyValue(value)
	}
	return out
}

func copyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return copyMap(typed)
	case []any:
		out := make([]any, len(typed))
		for index, nestedValue := range typed {
			out[index] = copyValue(nestedValue)
		}
		return out
	default:
		return value
	}
}
