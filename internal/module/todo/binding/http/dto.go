package httpbinding

import (
	"time"

	"github.com/rin721/go-scaffold-template/internal/module/todo/model"
)

// 模块自有 HTTP DTO（030：契约属于模块，由代码生成 api/openapi.yaml；Handler 只在 HTTP 边界
// 使用这些 wire 类型，不进入 model/service）。

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
