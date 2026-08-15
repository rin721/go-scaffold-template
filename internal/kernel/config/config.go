// Package config 负责加载不可变配置快照，并聚合、编码和安全写入能力默认配置。
package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"gopkg.in/yaml.v3"

	"github.com/rin721/go-scaffold-template/pkg/secrets"
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
	seenSources := make(map[string]struct{}, len(l.sources))
	for index, source := range l.sources {
		if source == nil {
			return Snapshot{}, fmt.Errorf("config source %d is nil", index)
		}
		name := strings.TrimSpace(source.Name())
		if name == "" {
			return Snapshot{}, fmt.Errorf("config source %d name is required", index)
		}
		if _, exists := seenSources[name]; exists {
			return Snapshot{}, fmt.Errorf("config source name %q is duplicated", name)
		}
		seenSources[name] = struct{}{}
		if err := ctx.Err(); err != nil {
			return Snapshot{}, fmt.Errorf("load config source %s: %w", name, err)
		}
		values, err := source.Load(ctx)
		if err != nil {
			return Snapshot{}, fmt.Errorf("load config source %s: %w", name, err)
		}
		normalized, err := canonicalMap(values)
		if err != nil {
			return Snapshot{}, fmt.Errorf("normalize config source %s: %w", name, err)
		}
		if err := mergeMap(merged, normalized, ""); err != nil {
			return Snapshot{}, fmt.Errorf("merge config source %s: %w", name, err)
		}
		provenance = append(provenance, name)
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
	copied, err := canonicalMap(values)
	if err != nil {
		return Snapshot{}, fmt.Errorf("normalize config snapshot: %w", err)
	}
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
		ErrorUnused:      true,
		WeaklyTypedInput: false,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			strictStringScalarHook,
		),
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

func (s fileSource) Load(ctx context.Context) (map[string]any, error) {
	if ctx == nil {
		return nil, fmt.Errorf("file config source context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var values map[string]any
	switch strings.ToLower(filepath.Ext(s.path)) {
	case ".json":
		values, err = decodeJSONObject(data)
	case ".yaml", ".yml", "":
		err = yaml.Unmarshal(data, &values)
	default:
		return nil, fmt.Errorf("unsupported config file extension %q", filepath.Ext(s.path))
	}
	if err != nil {
		return nil, err
	}
	return values, nil
}

func decodeJSONObject(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	value, err := decodeJSONValue(decoder)
	if err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, fmt.Errorf("multiple JSON root values")
	}
	if token, err := decoder.Token(); err == nil || token != nil {
		return nil, fmt.Errorf("unexpected data after JSON root")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("JSON configuration root must be an object")
	}
	return object, nil
}

func decodeJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return token, nil
	}
	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("JSON object key is not a string")
			}
			if _, exists := object[key]; exists {
				return nil, fmt.Errorf("duplicate JSON configuration key %q", key)
			}
			value, err := decodeJSONValue(decoder)
			if err != nil {
				return nil, fmt.Errorf("JSON key %s: %w", key, err)
			}
			object[key] = value
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return object, nil
	case '[':
		var values []any
		for decoder.More() {
			value, err := decodeJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return values, nil
	default:
		return nil, fmt.Errorf("unsupported JSON delimiter %q", delimiter)
	}
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

func (s envSource) Load(ctx context.Context) (map[string]any, error) {
	return loadEnvironment(ctx, s.prefix, os.Environ())
}

type configPathErrorKind string

const (
	configPathErrorCaseCollision configPathErrorKind = "case-collision"
	configPathErrorDuplicate     configPathErrorKind = "duplicate-path"
	configPathErrorEmptySegment  configPathErrorKind = "empty-segment"
	configPathErrorShapeConflict configPathErrorKind = "shape-conflict"
)

type configPathError struct {
	kind configPathErrorKind
	path string
}

func (e *configPathError) Error() string {
	path := e.path
	if path == "" {
		path = "<root>"
	}
	switch e.kind {
	case configPathErrorCaseCollision:
		return fmt.Sprintf("config path %s has a case-insensitive sibling collision", path)
	case configPathErrorDuplicate:
		return fmt.Sprintf("config path %s is duplicated", path)
	case configPathErrorEmptySegment:
		return fmt.Sprintf("config path %s contains an empty segment", path)
	case configPathErrorShapeConflict:
		return fmt.Sprintf("config path %s changes between object and non-object", path)
	default:
		return fmt.Sprintf("config path %s is invalid", path)
	}
}

type environmentRecord struct {
	rawPath  string
	path     string
	parts    []string
	value    string
	hasEmpty bool
}

func loadEnvironment(ctx context.Context, prefix string, entries []string) (map[string]any, error) {
	if ctx == nil {
		return nil, fmt.Errorf("environment config source context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	records := make([]environmentRecord, 0)
	for _, item := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key, value, ok := strings.Cut(item, "=")
		if !ok || !strings.HasPrefix(key, prefix) {
			continue
		}
		trimmed := strings.TrimPrefix(key, prefix)
		if trimmed == "" {
			continue
		}
		rawParts := strings.Split(trimmed, "__")
		parts := make([]string, len(rawParts))
		hasEmpty := false
		for index, part := range rawParts {
			parts[index] = strings.ToLower(part)
			hasEmpty = hasEmpty || part == ""
		}
		records = append(records, environmentRecord{
			rawPath:  strings.Join(rawParts, "."),
			path:     strings.Join(parts, "."),
			parts:    parts,
			value:    value,
			hasEmpty: hasEmpty,
		})
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].path == records[j].path {
			return records[i].rawPath < records[j].rawPath
		}
		return records[i].path < records[j].path
	})
	if err := validateEnvironmentRecords(ctx, records); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := insertPath(out, record.parts, record.value, ""); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func validateEnvironmentRecords(ctx context.Context, records []environmentRecord) error {
	var first *configPathError
	consider := func(candidate *configPathError) {
		if first == nil || candidate.path < first.path || candidate.path == first.path && candidate.kind < first.kind {
			first = candidate
		}
	}
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if record.hasEmpty {
			consider(&configPathError{kind: configPathErrorEmptySegment, path: record.path})
		}
	}
	for left := 0; left < len(records); left++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if records[left].hasEmpty {
			continue
		}
		for right := left + 1; right < len(records); right++ {
			if records[right].hasEmpty {
				continue
			}
			common := commonPathPrefix(records[left].parts, records[right].parts)
			if common == 0 {
				continue
			}
			leftComplete := common == len(records[left].parts)
			rightComplete := common == len(records[right].parts)
			switch {
			case leftComplete && rightComplete:
				kind := configPathErrorDuplicate
				if records[left].rawPath != records[right].rawPath {
					kind = configPathErrorCaseCollision
				}
				consider(&configPathError{kind: kind, path: records[left].path})
			case leftComplete || rightComplete:
				path := records[left].path
				if rightComplete {
					path = records[right].path
				}
				consider(&configPathError{kind: configPathErrorShapeConflict, path: path})
			}
		}
	}
	if first == nil {
		return nil
	}
	return first
}

func commonPathPrefix(left, right []string) int {
	limit := min(len(left), len(right))
	for index := 0; index < limit; index++ {
		if !strings.EqualFold(left[index], right[index]) {
			return index
		}
	}
	return limit
}

func mergeMap(dst map[string]any, src map[string]any, parent string) error {
	for _, candidate := range orderedKeys(src) {
		value := src[candidate]
		key := candidate
		key = matchingKey(dst, key)
		path := joinConfigPath(parent, strings.ToLower(key))
		nested, incomingObject := value.(map[string]any)
		existing, exists := dst[key]
		existingObject, currentObject := existing.(map[string]any)
		if exists && incomingObject != currentObject {
			return &configPathError{kind: configPathErrorShapeConflict, path: path}
		}
		if incomingObject && exists {
			if err := mergeMap(existingObject, nested, path); err != nil {
				return err
			}
			continue
		}
		dst[key] = copyValue(value)
	}
	return nil
}

func matchingKey(values map[string]any, candidate string) string {
	for _, existing := range orderedKeys(values) {
		if strings.EqualFold(existing, candidate) {
			return existing
		}
	}
	return candidate
}

func insertPath(values map[string]any, parts []string, value any, parent string) error {
	if len(parts) == 0 {
		return &configPathError{kind: configPathErrorEmptySegment, path: parent}
	}
	path := joinConfigPath(parent, parts[0])
	if parts[0] == "" {
		return &configPathError{kind: configPathErrorEmptySegment, path: path}
	}
	if len(parts) == 1 {
		if _, exists := values[parts[0]]; exists {
			return &configPathError{kind: configPathErrorDuplicate, path: path}
		}
		values[parts[0]] = value
		return nil
	}
	next, exists := values[parts[0]].(map[string]any)
	if _, present := values[parts[0]]; present && !exists {
		return &configPathError{kind: configPathErrorShapeConflict, path: path}
	}
	if !exists {
		next = map[string]any{}
		values[parts[0]] = next
	}
	return insertPath(next, parts[1:], value, path)
}

func canonicalMap(values map[string]any) (map[string]any, error) {
	return canonicalMapAt(values, "")
}

func canonicalMapAt(values map[string]any, parent string) (map[string]any, error) {
	if values == nil {
		return map[string]any{}, nil
	}
	out := make(map[string]any, len(values))
	keys := orderedKeys(values)
	for index, key := range keys {
		if strings.TrimSpace(key) == "" {
			return nil, &configPathError{kind: configPathErrorEmptySegment, path: parent}
		}
		for previous := 0; previous < index; previous++ {
			if strings.EqualFold(keys[previous], key) {
				return nil, &configPathError{
					kind: configPathErrorCaseCollision,
					path: joinConfigPath(parent, strings.ToLower(key)),
				}
			}
		}
	}
	for _, key := range keys {
		path := joinConfigPath(parent, strings.ToLower(key))
		normalized, err := canonicalValueAt(values[key], path)
		if err != nil {
			return nil, fmt.Errorf("config key %s: %w", key, err)
		}
		out[key] = normalized
	}
	return out, nil
}

func canonicalValueAt(value any, path string) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		return canonicalMapAt(typed, path)
	case map[any]any:
		values := make(map[string]any, len(typed))
		for key, nestedValue := range typed {
			name, ok := key.(string)
			if !ok || strings.TrimSpace(name) == "" {
				return nil, fmt.Errorf("configuration object key must be a non-empty string")
			}
			values[name] = nestedValue
		}
		return canonicalMapAt(values, path)
	case []any:
		out := make([]any, len(typed))
		for index, nestedValue := range typed {
			normalized, err := canonicalValueAt(nestedValue, fmt.Sprintf("%s[%d]", path, index))
			if err != nil {
				return nil, fmt.Errorf("config list element %d: %w", index, err)
			}
			out[index] = normalized
		}
		return out, nil
	case nil, string, bool, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, float32, float64, json.Number:
		return typed, nil
	default:
		return nil, fmt.Errorf("unsupported configuration value type %T", value)
	}
}

func orderedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left := strings.ToLower(keys[i])
		right := strings.ToLower(keys[j])
		if left == right {
			return keys[i] < keys[j]
		}
		return left < right
	})
	return keys
}

func joinConfigPath(parent, segment string) string {
	if parent == "" {
		return segment
	}
	return parent + "." + segment
}

func strictStringScalarHook(from reflect.Type, to reflect.Type, data any) (any, error) {
	if from.Kind() != reflect.String {
		return data, nil
	}
	value := data.(string)
	switch to.Kind() {
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("parse %q as bool: %w", value, err)
		}
		return parsed, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(value, 10, to.Bits())
		if err != nil {
			return nil, fmt.Errorf("parse %q as signed integer: %w", value, err)
		}
		return reflect.ValueOf(parsed).Convert(to).Interface(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(value, 10, to.Bits())
		if err != nil {
			return nil, fmt.Errorf("parse %q as unsigned integer: %w", value, err)
		}
		return reflect.ValueOf(parsed).Convert(to).Interface(), nil
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(value, to.Bits())
		if err != nil {
			return nil, fmt.Errorf("parse %q as decimal: %w", value, err)
		}
		return reflect.ValueOf(parsed).Convert(to).Interface(), nil
	default:
		return data, nil
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
