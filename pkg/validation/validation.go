package validation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// FieldError 表示单个字段的校验失败。
type FieldError struct {
	Field   string
	Rule    string
	Value   any
	Message string
}

// Error 聚合结构体校验错误。
type Error struct {
	Fields []FieldError
}

func (e *Error) Error() string {
	if e == nil || len(e.Fields) == 0 {
		return "validation failed"
	}
	parts := make([]string, 0, len(e.Fields))
	for _, field := range e.Fields {
		parts = append(parts, field.Field+":"+field.Rule)
	}
	return "validation failed: " + strings.Join(parts, ", ")
}

// Validator 是项目对外暴露的校验器契约。
type Validator interface {
	Struct(value any) error
}

type defaultValidator struct {
	validate *validator.Validate
}

// New 创建默认校验器。
func New() Validator {
	return &defaultValidator{validate: validator.New(validator.WithRequiredStructEnabled())}
}

func (v *defaultValidator) Struct(value any) error {
	if value == nil {
		return &Error{Fields: []FieldError{{Field: "$", Rule: "required", Message: "value is required"}}}
	}
	err := v.validate.Struct(value)
	if err == nil {
		return nil
	}
	var invalid *validator.InvalidValidationError
	if errors.As(err, &invalid) {
		return fmt.Errorf("invalid validation target: %w", err)
	}
	var fieldErrors validator.ValidationErrors
	if !errors.As(err, &fieldErrors) {
		return err
	}
	out := make([]FieldError, 0, len(fieldErrors))
	for _, field := range fieldErrors {
		out = append(out, FieldError{
			Field:   field.Namespace(),
			Rule:    field.Tag(),
			Value:   field.Value(),
			Message: field.Error(),
		})
	}
	return &Error{Fields: out}
}

// Struct 使用默认校验器校验结构体。
func Struct(value any) error {
	return New().Struct(value)
}
