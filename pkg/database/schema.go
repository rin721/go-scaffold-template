package database

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)
var defaultExpressionPattern = regexp.MustCompile(`(?i)^(NULL|TRUE|FALSE|CURRENT_TIMESTAMP|-?[0-9]+(?:\.[0-9]+)?|'(?:[^';\r\n]|'')*')$`)

// FieldType 表示 Schema 可移植字段类型。
type FieldType string

const (
	// FieldBool 表示布尔值。
	FieldBool FieldType = "bool"
	// FieldInt 表示平台 int。
	FieldInt FieldType = "int"
	// FieldInt64 表示有符号 64 位整数。
	FieldInt64 FieldType = "int64"
	// FieldUint 表示平台 uint。
	FieldUint FieldType = "uint"
	// FieldUint64 表示无符号 64 位整数。
	FieldUint64 FieldType = "uint64"
	// FieldFloat64 表示 64 位浮点数。
	FieldFloat64 FieldType = "float64"
	// FieldString 表示字符串。
	FieldString FieldType = "string"
	// FieldBytes 表示二进制数据。
	FieldBytes FieldType = "bytes"
	// FieldTime 表示时间。
	FieldTime FieldType = "time"
)

// Field 描述业务字段到数据库列的稳定映射。
type Field struct {
	Name          string
	Column        string
	Type          FieldType
	PrimaryKey    bool
	AutoIncrement bool
	Nullable      bool
	Length        int
	Default       string
}

// Index 描述由 Schema 所有的普通或唯一索引。
type Index struct {
	Name   string
	Fields []string
	Unique bool
}

// ReferenceAction 表示可移植的外键引用动作。零值表示使用数据库默认动作。
type ReferenceAction string

const (
	// ReferenceCascade 将目标行变更级联到引用行。
	ReferenceCascade ReferenceAction = "CASCADE"
	// ReferenceRestrict 在存在引用行时拒绝目标行变更。
	ReferenceRestrict ReferenceAction = "RESTRICT"
	// ReferenceSetNull 将引用列设为 NULL，只能用于 nullable 字段。
	ReferenceSetNull ReferenceAction = "SET NULL"
	// ReferenceNoAction 使用数据库的 NO ACTION 语义。
	ReferenceNoAction ReferenceAction = "NO ACTION"
)

// Reference 用两个 Schema 字段名描述单列外键关系。
//
// Table 是目标 Schema 的表名，ReferenceField 是该 Schema 中的字段名，
// 不是数据库列名。目标 Schema 必须与当前 Schema 一起传给 Migrate。
type Reference struct {
	Field          string
	Table          string
	ReferenceField string
	OnUpdate       ReferenceAction
	OnDelete       ReferenceAction
}

// Schema 描述表、字段、索引和仓储并发语义。
//
// VersionField 启用乐观锁；SoftDeleteField 指向可空时间字段。
type Schema struct {
	Table           string
	Fields          []Field
	Indexes         []Index
	References      []Reference
	VersionField    string
	SoftDeleteField string
}

type resolvedSchema struct {
	Schema
	fields map[string]Field
}

func validateReferences(schemas map[string]resolvedSchema) error {
	for _, schema := range schemas {
		for _, reference := range schema.References {
			source := schema.fields[reference.Field]
			target, exists := schemas[reference.Table]
			if !exists {
				return fmt.Errorf("%w: reference target schema %q is missing", ErrInvalidSchema, reference.Table)
			}
			targetField, exists := target.fields[reference.ReferenceField]
			if !exists {
				return fmt.Errorf("%w: reference target field %q.%q is missing", ErrInvalidSchema, reference.Table, reference.ReferenceField)
			}
			if source.Type != targetField.Type || source.Length != targetField.Length {
				return fmt.Errorf("%w: reference fields %q.%q and %q.%q have incompatible types", ErrInvalidSchema, schema.Table, reference.Field, reference.Table, reference.ReferenceField)
			}
			if !target.uniqueField(reference.ReferenceField) {
				return fmt.Errorf("%w: reference target %q.%q is not uniquely constrained", ErrInvalidSchema, reference.Table, reference.ReferenceField)
			}
			if (reference.OnUpdate == ReferenceSetNull || reference.OnDelete == ReferenceSetNull) && !source.Nullable {
				return fmt.Errorf("%w: SET NULL reference field %q.%q is not nullable", ErrInvalidSchema, schema.Table, reference.Field)
			}
		}
	}
	return nil
}

func (s resolvedSchema) uniqueField(name string) bool {
	primaryCount := 0
	for _, field := range s.Fields {
		if field.PrimaryKey {
			primaryCount++
		}
	}
	if field := s.fields[name]; field.PrimaryKey && primaryCount == 1 {
		return true
	}
	for _, index := range s.Indexes {
		if index.Unique && len(index.Fields) == 1 && index.Fields[0] == name {
			return true
		}
	}
	return false
}

func resolveSchema(schema Schema) (resolvedSchema, error) {
	if !validIdentifier(schema.Table) {
		return resolvedSchema{}, fmt.Errorf("%w: invalid table %q", ErrInvalidSchema, schema.Table)
	}
	if len(schema.Fields) == 0 {
		return resolvedSchema{}, fmt.Errorf("%w: schema %q has no fields", ErrInvalidSchema, schema.Table)
	}
	resolved := resolvedSchema{Schema: cloneSchema(schema), fields: make(map[string]Field, len(schema.Fields))}
	columns := make(map[string]struct{}, len(schema.Fields))
	primaryKeys := 0
	autoIncrementFields := 0
	for _, field := range schema.Fields {
		if !validFieldName(field.Name) || !validIdentifier(field.Column) {
			return resolvedSchema{}, fmt.Errorf("%w: invalid field %q or column %q", ErrInvalidSchema, field.Name, field.Column)
		}
		if _, exists := resolved.fields[field.Name]; exists {
			return resolvedSchema{}, fmt.Errorf("%w: duplicate field %q", ErrInvalidSchema, field.Name)
		}
		if _, exists := columns[field.Column]; exists {
			return resolvedSchema{}, fmt.Errorf("%w: duplicate column %q", ErrInvalidSchema, field.Column)
		}
		if field.Length < 0 || !validFieldType(field.Type) {
			return resolvedSchema{}, fmt.Errorf("%w: invalid field %q metadata", ErrInvalidSchema, field.Name)
		}
		if field.Length > 0 && field.Type != FieldString {
			return resolvedSchema{}, fmt.Errorf("%w: length is only supported for string field %q", ErrInvalidSchema, field.Name)
		}
		if field.Default != "" && (!defaultExpressionPattern.MatchString(field.Default) || !validDefault(field)) {
			return resolvedSchema{}, fmt.Errorf("%w: invalid default for field %q", ErrInvalidSchema, field.Name)
		}
		if field.PrimaryKey {
			if field.Nullable {
				return resolvedSchema{}, fmt.Errorf("%w: primary key field %q cannot be nullable", ErrInvalidSchema, field.Name)
			}
			primaryKeys++
		}
		if field.AutoIncrement {
			autoIncrementFields++
			if !field.PrimaryKey || !isIntegerField(field.Type) || field.Default != "" {
				return resolvedSchema{}, fmt.Errorf("%w: auto increment field %q must be an integer primary key", ErrInvalidSchema, field.Name)
			}
		}
		resolved.fields[field.Name] = field
		columns[field.Column] = struct{}{}
	}
	if primaryKeys == 0 {
		return resolvedSchema{}, fmt.Errorf("%w: schema %q has no primary key", ErrInvalidSchema, schema.Table)
	}
	if primaryKeys > 1 && autoIncrementFields > 0 {
		return resolvedSchema{}, fmt.Errorf("%w: composite primary key cannot use auto increment", ErrInvalidSchema)
	}
	indexNames := make(map[string]struct{}, len(schema.Indexes))
	for _, index := range schema.Indexes {
		if !validIdentifier(index.Name) || len(index.Fields) == 0 {
			return resolvedSchema{}, fmt.Errorf("%w: invalid index %q", ErrInvalidSchema, index.Name)
		}
		if _, exists := indexNames[index.Name]; exists {
			return resolvedSchema{}, fmt.Errorf("%w: duplicate index %q", ErrInvalidSchema, index.Name)
		}
		indexNames[index.Name] = struct{}{}
		indexFields := make(map[string]struct{}, len(index.Fields))
		for _, name := range index.Fields {
			if _, exists := resolved.fields[name]; !exists {
				return resolvedSchema{}, fmt.Errorf("%w: index %q references field %q", ErrInvalidSchema, index.Name, name)
			}
			if _, exists := indexFields[name]; exists {
				return resolvedSchema{}, fmt.Errorf("%w: index %q repeats field %q", ErrInvalidSchema, index.Name, name)
			}
			indexFields[name] = struct{}{}
		}
	}
	referenceFields := make(map[string]struct{}, len(schema.References))
	for _, reference := range schema.References {
		if _, exists := resolved.fields[reference.Field]; !exists || !validIdentifier(reference.Table) || !validFieldName(reference.ReferenceField) {
			return resolvedSchema{}, fmt.Errorf("%w: invalid reference for field %q", ErrInvalidSchema, reference.Field)
		}
		if !validReferenceAction(reference.OnUpdate) || !validReferenceAction(reference.OnDelete) {
			return resolvedSchema{}, fmt.Errorf("%w: invalid reference action", ErrInvalidSchema)
		}
		if _, exists := referenceFields[reference.Field]; exists {
			return resolvedSchema{}, fmt.Errorf("%w: field %q has multiple references", ErrInvalidSchema, reference.Field)
		}
		referenceFields[reference.Field] = struct{}{}
		constraint := "fk_" + schema.Table + "_" + resolved.fields[reference.Field].Column
		if !validIdentifier(constraint) {
			return resolvedSchema{}, fmt.Errorf("%w: generated constraint %q is invalid", ErrInvalidSchema, constraint)
		}
	}
	if schema.VersionField != "" {
		field, exists := resolved.fields[schema.VersionField]
		if !exists || field.Nullable || field.PrimaryKey || field.AutoIncrement || field.Default != "" ||
			(field.Type != FieldInt && field.Type != FieldInt64 && field.Type != FieldUint && field.Type != FieldUint64) {
			return resolvedSchema{}, fmt.Errorf("%w: invalid version field %q", ErrInvalidSchema, schema.VersionField)
		}
	}
	if schema.SoftDeleteField != "" {
		field, exists := resolved.fields[schema.SoftDeleteField]
		if !exists || field.Type != FieldTime || !field.Nullable || field.PrimaryKey || field.AutoIncrement || field.Default != "" {
			return resolvedSchema{}, fmt.Errorf("%w: invalid soft delete field %q", ErrInvalidSchema, schema.SoftDeleteField)
		}
	}
	return resolved, nil
}

func cloneSchema(schema Schema) Schema {
	cloned := schema
	cloned.Fields = append([]Field(nil), schema.Fields...)
	cloned.References = append([]Reference(nil), schema.References...)
	cloned.Indexes = make([]Index, len(schema.Indexes))
	for index, value := range schema.Indexes {
		cloned.Indexes[index] = value
		cloned.Indexes[index].Fields = append([]string(nil), value.Fields...)
	}
	return cloned
}

func (s resolvedSchema) column(field string) (string, error) {
	metadata, exists := s.fields[field]
	if !exists {
		return "", fmt.Errorf("%w: unknown field %q", ErrInvalidQuery, field)
	}
	return metadata.Column, nil
}

func (s resolvedSchema) dynamicModel() any {
	fields := append([]Field(nil), s.Fields...)
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	structFields := make([]reflect.StructField, 0, len(fields))
	for _, field := range fields {
		structFields = append(structFields, reflect.StructField{
			Name: exportedName(field.Name), Type: fieldReflectType(field),
			Tag: reflect.StructTag(`gorm:"` + gormFieldTag(field, s.Indexes) + `"`),
		})
	}
	return reflect.New(reflect.StructOf(structFields)).Interface()
}

func gormFieldTag(field Field, indexes []Index) string {
	parts := []string{"column:" + field.Column}
	if field.PrimaryKey {
		parts = append(parts, "primaryKey")
	}
	if field.AutoIncrement {
		parts = append(parts, "autoIncrement")
	} else {
		parts = append(parts, "autoIncrement:false")
	}
	if !field.Nullable {
		parts = append(parts, "not null")
	}
	if field.Length > 0 {
		parts = append(parts, fmt.Sprintf("size:%d", field.Length))
	}
	if field.Default != "" {
		parts = append(parts, "default:"+field.Default)
	}
	for _, index := range indexes {
		for position, name := range index.Fields {
			if name != field.Name {
				continue
			}
			kind := "index"
			if index.Unique {
				kind = "uniqueIndex"
			}
			parts = append(parts, fmt.Sprintf("%s:%s,priority:%d", kind, index.Name, position+1))
		}
	}
	return strings.Join(parts, ";")
}

func fieldReflectType(field Field) reflect.Type {
	var value any
	switch field.Type {
	case FieldBool:
		value = false
	case FieldInt:
		value = int(0)
	case FieldInt64:
		value = int64(0)
	case FieldUint:
		value = uint(0)
	case FieldUint64:
		value = uint64(0)
	case FieldFloat64:
		value = float64(0)
	case FieldBytes:
		value = []byte(nil)
	case FieldTime:
		value = time.Time{}
	default:
		value = ""
	}
	typeOf := reflect.TypeOf(value)
	if field.Nullable {
		typeOf = reflect.PointerTo(typeOf)
	}
	return typeOf
}

func exportedName(name string) string {
	return name
}

func validIdentifier(value string) bool { return identifierPattern.MatchString(value) }

func validFieldName(value string) bool {
	return validIdentifier(value) && value[0] >= 'A' && value[0] <= 'Z'
}

func isIntegerField(value FieldType) bool {
	switch value {
	case FieldInt, FieldInt64, FieldUint, FieldUint64:
		return true
	default:
		return false
	}
}

func validFieldType(value FieldType) bool {
	switch value {
	case FieldBool, FieldInt, FieldInt64, FieldUint, FieldUint64, FieldFloat64, FieldString, FieldBytes, FieldTime:
		return true
	default:
		return false
	}
}

func validDefault(field Field) bool {
	value := field.Default
	switch strings.ToUpper(value) {
	case "NULL":
		return field.Nullable
	case "TRUE", "FALSE":
		return field.Type == FieldBool
	case "CURRENT_TIMESTAMP":
		return field.Type == FieldTime
	}
	if strings.HasPrefix(value, "'") {
		return field.Type == FieldString
	}
	var err error
	switch field.Type {
	case FieldInt:
		_, err = strconv.ParseInt(value, 10, strconv.IntSize)
	case FieldInt64:
		_, err = strconv.ParseInt(value, 10, 64)
	case FieldUint:
		_, err = strconv.ParseUint(value, 10, strconv.IntSize)
	case FieldUint64:
		_, err = strconv.ParseUint(value, 10, 64)
	case FieldFloat64:
		_, err = strconv.ParseFloat(value, 64)
	default:
		return false
	}
	return err == nil
}

func validReferenceAction(value ReferenceAction) bool {
	switch value {
	case "", ReferenceCascade, ReferenceRestrict, ReferenceSetNull, ReferenceNoAction:
		return true
	default:
		return false
	}
}
