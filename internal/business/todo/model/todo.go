// Package model 定义 Todo 的业务状态与不变量。
package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrInvalidID 表示 Todo ID 缺失。
	ErrInvalidID = errors.New("todo id is invalid")
	// ErrInvalidTitle 表示 Todo 标题不满足当前业务策略。
	ErrInvalidTitle = errors.New("todo title is invalid")
	// ErrInvalidStatus 表示持久化或调用方提供了未知状态。
	ErrInvalidStatus = errors.New("todo status is invalid")
	// ErrInvalidTime 表示 Todo 时间字段不满足状态不变量。
	ErrInvalidTime = errors.New("todo time is invalid")
)

// Status 是 Todo 的有限状态集合。
type Status string

const (
	// StatusPending 表示 Todo 尚未完成。
	StatusPending Status = "pending"
	// StatusCompleted 表示 Todo 已完成。
	StatusCompleted Status = "completed"
)

// Todo 是不携带 HTTP、CLI 或 ORM 标签的业务实体。
type Todo struct {
	ID          string
	Title       string
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
	Version     uint64
}

// NormalizeTitle 按当前策略规范化并验证标题。
func NormalizeTitle(title string, maxRunes int) (string, error) {
	normalized := strings.TrimSpace(title)
	if maxRunes <= 0 || normalized == "" || len([]rune(normalized)) > maxRunes {
		return "", ErrInvalidTitle
	}
	return normalized, nil
}

// ParseStatus 把协议或持久化字符串转换为有限状态。
func ParseStatus(value string) (Status, error) {
	status := Status(value)
	if status != StatusPending && status != StatusCompleted {
		return "", fmt.Errorf("%w: %q", ErrInvalidStatus, value)
	}
	return status, nil
}

// New 创建一个待完成 Todo。
func New(id, title string, now time.Time) (Todo, error) {
	if strings.TrimSpace(id) == "" {
		return Todo{}, ErrInvalidID
	}
	if title == "" {
		return Todo{}, ErrInvalidTitle
	}
	now = now.UTC()
	if now.IsZero() {
		return Todo{}, ErrInvalidTime
	}
	return Todo{ID: id, Title: title, Status: StatusPending, CreatedAt: now, UpdatedAt: now}, nil
}

// Restore 从持久化字段恢复并校验 Todo。
func Restore(todo Todo) (Todo, error) {
	if strings.TrimSpace(todo.ID) == "" {
		return Todo{}, ErrInvalidID
	}
	if todo.Title == "" {
		return Todo{}, ErrInvalidTitle
	}
	if _, err := ParseStatus(string(todo.Status)); err != nil {
		return Todo{}, err
	}
	if todo.CreatedAt.IsZero() || todo.UpdatedAt.IsZero() || todo.Version == 0 {
		return Todo{}, ErrInvalidTime
	}
	if todo.Status == StatusPending && todo.CompletedAt != nil {
		return Todo{}, ErrInvalidTime
	}
	if todo.Status == StatusCompleted && (todo.CompletedAt == nil || todo.CompletedAt.IsZero()) {
		return Todo{}, ErrInvalidTime
	}
	todo.CreatedAt = todo.CreatedAt.UTC()
	todo.UpdatedAt = todo.UpdatedAt.UTC()
	if todo.CompletedAt != nil {
		completed := todo.CompletedAt.UTC()
		todo.CompletedAt = &completed
	}
	return todo, nil
}

// Complete 把待完成 Todo 转成已完成；重复调用保持原完成时间。
func (t *Todo) Complete(now time.Time) (bool, error) {
	if t == nil {
		return false, ErrInvalidID
	}
	if t.Status == StatusCompleted {
		return false, nil
	}
	if t.Status != StatusPending || now.IsZero() {
		return false, ErrInvalidTime
	}
	completed := now.UTC()
	t.Status = StatusCompleted
	t.CompletedAt = &completed
	t.UpdatedAt = completed
	return true, nil
}
