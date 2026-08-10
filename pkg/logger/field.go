package logger

import (
	"time"

	"go.uber.org/zap"
)

// Field 表示一项结构化日志字段。
type Field struct {
	key   string
	value any
	kind  fieldKind
}

type fieldKind string

const (
	fieldKindString   fieldKind = "string"
	fieldKindInt      fieldKind = "int"
	fieldKindInt64    fieldKind = "int64"
	fieldKindBool     fieldKind = "bool"
	fieldKindError    fieldKind = "error"
	fieldKindAny      fieldKind = "any"
	fieldKindDuration fieldKind = "duration"
)

// String 构造字符串字段。
func String(key string, value string) Field {
	return Field{key: key, value: value, kind: fieldKindString}
}

// Int 构造整数字段。
func Int(key string, value int) Field {
	return Field{key: key, value: value, kind: fieldKindInt}
}

// Int64 构造 int64 字段。
func Int64(key string, value int64) Field {
	return Field{key: key, value: value, kind: fieldKindInt64}
}

// Bool 构造布尔字段。
func Bool(key string, value bool) Field {
	return Field{key: key, value: value, kind: fieldKindBool}
}

// Error 构造错误字段。
func Error(err error) Field {
	return Field{key: encoderErrorKey, value: err, kind: fieldKindError}
}

// Any 构造任意值字段，适合低频或调试场景。
func Any(key string, value any) Field {
	return Field{key: key, value: value, kind: fieldKindAny}
}

// Duration 构造耗时字段。
func Duration(key string, value time.Duration) Field {
	return Field{key: key, value: value, kind: fieldKindDuration}
}

func toZapFields(fields []Field) []zap.Field {
	if len(fields) == 0 {
		return nil
	}

	zapFields := make([]zap.Field, 0, len(fields))
	for _, field := range fields {
		zapFields = append(zapFields, field.toZapField())
	}
	return zapFields
}

func (f Field) toZapField() zap.Field {
	switch f.kind {
	case fieldKindString:
		value, _ := f.value.(string)
		return zap.String(f.key, value)
	case fieldKindInt:
		value, _ := f.value.(int)
		return zap.Int(f.key, value)
	case fieldKindInt64:
		value, _ := f.value.(int64)
		return zap.Int64(f.key, value)
	case fieldKindBool:
		value, _ := f.value.(bool)
		return zap.Bool(f.key, value)
	case fieldKindError:
		value, _ := f.value.(error)
		return zap.Error(value)
	case fieldKindDuration:
		value, _ := f.value.(time.Duration)
		return zap.Duration(f.key, value)
	case fieldKindAny:
		fallthrough
	default:
		return zap.Any(f.key, f.value)
	}
}
