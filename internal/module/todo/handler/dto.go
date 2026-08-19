// Package handler 提供模块顶层 HTTP handler 层（031 分责）：HTTP 应用语义适配到 Todo UseCases。
//
// 职责：读 actor、组装 UseCases 命令/查询、DTO 映射与错误呈现。不创建 Router、不加载 OpenAPI、
// 不 import binding/** 或 internal/transport/**，也不使用第三方 HTTP 框架。
package handler

import (
	"time"

	"github.com/rin721/go-scaffold-template/internal/module/todo/model"
)

// TodoStatus 是 Todo 状态在 HTTP 边界的可枚举字符串。
type TodoStatus string

const (
	StatusPending   TodoStatus = "pending"
	StatusCompleted TodoStatus = "completed"
)

// CreateTodoRequest 是创建 Todo 的请求体。
type CreateTodoRequest struct {
	Title string `json:"title"`
}

// Todo 是 Todo 资源的 HTTP 表示。
type Todo struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Status      TodoStatus `json:"status"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// TodoList 是 Todo 分页列表的 HTTP 表示。
type TodoList struct {
	Items  []Todo `json:"items"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
	Total  int64  `json:"total"`
}

// ListTodosParams 是 listTodos 的查询参数。
type ListTodosParams struct {
	Status *TodoStatus `form:"status"`
	Offset *int        `form:"offset"`
	Limit  *int        `form:"limit"`
}

// todoDTO 把业务实体转换为 HTTP DTO。时间统一截断到毫秒，避免纳秒噪声进入契约。
func todoDTO(todo model.Todo) Todo {
	dto := Todo{
		ID:        todo.ID,
		Title:     todo.Title,
		Status:    TodoStatus(todo.Status),
		CreatedAt: todo.CreatedAt.Truncate(time.Millisecond),
		UpdatedAt: todo.UpdatedAt.Truncate(time.Millisecond),
	}
	if todo.CompletedAt != nil {
		completed := todo.CompletedAt.Truncate(time.Millisecond)
		dto.CompletedAt = &completed
	}
	return dto
}
