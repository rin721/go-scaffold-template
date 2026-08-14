package config

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"time"
)

// Control 表示默认配置契约是否继续参与聚合。
type Control uint8

const (
	// Continue 表示接受当前能力的默认配置并继续聚合。
	Continue Control = iota
	// Abort 表示当前能力主动中止整次默认配置生成。
	Abort
)

var (
	// ErrInvalidValue 标识默认配置文档包含非法结构或值。
	ErrInvalidValue = errors.New("invalid default configuration value")
	// ErrAborted 标识能力契约主动中止了默认配置生成。
	ErrAborted = errors.New("default configuration generation aborted")
	// ErrTargetExists 标识生成目标已经存在且调用方未允许替换。
	ErrTargetExists = errors.New("default configuration target exists")
)

// DefaultContract 由能力实现，用于提供其配置路径下的有序默认配置。
type DefaultContract interface {
	Defaults(context.Context) (Object, Control, error)
}

// DefaultContractFunc 把函数适配为 DefaultContract。
type DefaultContractFunc func(context.Context) (Object, Control, error)

// Defaults 调用被适配的默认配置函数。
func (f DefaultContractFunc) Defaults(ctx context.Context) (Object, Control, error) {
	return f(ctx)
}

// Binding 把成功登记的能力归属信息与其默认配置契约绑定。
type Binding struct {
	CapabilityID string
	ConfigPath   string
	Contract     DefaultContract
	Validate     func(Snapshot) error
}

// Object 是保持字段声明顺序的配置对象。
type Object []Field

// Field 是有序配置对象中的一个命名字段。
type Field struct {
	Name  string
	Value Value
}

// Value 是只能通过本包构造的格式无关配置值。
type Value interface {
	isConfigValue()
}

type valueKind uint8

const (
	valueString valueKind = iota
	valueBool
	valueNumber
	valueDuration
	valueNull
	valueObject
	valueList
)

type configValue struct {
	kind     valueKind
	text     string
	boolean  bool
	object   Object
	elements []Value
}

func (configValue) isConfigValue() {}

var decimalPattern = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)

// String 创建字符串配置值。
func String(value string) Value { return configValue{kind: valueString, text: value} }

// Bool 创建布尔配置值。
func Bool(value bool) Value { return configValue{kind: valueBool, boolean: value} }

// Number 创建保持原始十进制表示的数字配置值。
func Number(value string) (Value, error) {
	if !decimalPattern.MatchString(value) {
		return nil, fmt.Errorf("%w: %q is not a decimal number", ErrInvalidValue, value)
	}
	if _, _, err := big.ParseFloat(value, 10, 256, big.ToNearestEven); err != nil {
		return nil, fmt.Errorf("%w: parse number %q: %v", ErrInvalidValue, value, err)
	}
	return configValue{kind: valueNumber, text: value}, nil
}

// Duration 创建按 time.Duration 可读格式编码的字符串配置值。
func Duration(value time.Duration) Value {
	return configValue{kind: valueDuration, text: value.String()}
}

// Null 创建空配置值。
func Null() Value { return configValue{kind: valueNull} }

// ObjectValue 创建嵌套有序对象配置值。
func ObjectValue(value Object) Value {
	return configValue{kind: valueObject, object: append(Object(nil), value...)}
}

// List 创建保持元素顺序的列表配置值。
func List(values ...Value) Value {
	return configValue{kind: valueList, elements: append([]Value(nil), values...)}
}

// FieldOf 创建一个有序对象字段。
func FieldOf(name string, value Value) Field { return Field{Name: name, Value: value} }

func validateObject(object Object) error {
	seen := make(map[string]struct{}, len(object))
	for _, field := range object {
		if field.Name == "" {
			return fmt.Errorf("%w: field name is empty", ErrInvalidValue)
		}
		if _, exists := seen[field.Name]; exists {
			return fmt.Errorf("%w: duplicate field %q", ErrInvalidValue, field.Name)
		}
		seen[field.Name] = struct{}{}
		if err := validateValue(field.Value); err != nil {
			return fmt.Errorf("field %q: %w", field.Name, err)
		}
	}
	return nil
}

func validateValue(value Value) error {
	if value == nil {
		return fmt.Errorf("%w: value is nil", ErrInvalidValue)
	}
	concrete, ok := value.(configValue)
	if !ok {
		return fmt.Errorf("%w: unknown value implementation", ErrInvalidValue)
	}
	switch concrete.kind {
	case valueString, valueBool, valueDuration, valueNull:
		return nil
	case valueNumber:
		if _, err := Number(concrete.text); err != nil {
			return err
		}
		return nil
	case valueObject:
		return validateObject(concrete.object)
	case valueList:
		for index, element := range concrete.elements {
			if err := validateValue(element); err != nil {
				return fmt.Errorf("list element %d: %w", index, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown value kind %d", ErrInvalidValue, concrete.kind)
	}
}

// AbortedError 保留主动中止的能力归属和原始原因。
type AbortedError struct {
	CapabilityID string
	Cause        error
}

// Error 返回包含能力归属的中止说明。
func (e *AbortedError) Error() string {
	return fmt.Sprintf("%s: capability %s: %v", ErrAborted, e.CapabilityID, e.Cause)
}

// Unwrap 保留能力提供的原始原因链。
func (e *AbortedError) Unwrap() error { return e.Cause }

// Is 同时支持按默认配置中止类别识别错误。
func (e *AbortedError) Is(target error) bool { return target == ErrAborted }
