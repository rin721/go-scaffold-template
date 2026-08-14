// Package handler 实现 Todo 的 HTTP 入站 Adapter。
package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/rin721/go-scaffold2/internal/business/todo/model"
	"github.com/rin721/go-scaffold2/internal/business/todo/service"
	"github.com/rin721/go-scaffold2/pkg/clock"
	"github.com/rin721/go-scaffold2/pkg/fault"
	"github.com/rin721/go-scaffold2/pkg/httpx"
	"github.com/rin721/go-scaffold2/pkg/i18n"
)

const acceptLanguageHeader = "Accept-Language"

// Handler 把 HTTP DTO 转换为 Todo UseCases。
type Handler struct {
	service    service.UseCases
	translator i18n.Translator
}

// New 创建无 I/O 副作用的 Todo HTTP Handler。
func New(todoService service.UseCases, translator i18n.Translator) (*Handler, error) {
	if todoService == nil {
		return nil, fmt.Errorf("todo HTTP service is nil")
	}
	if translator == nil {
		return nil, fmt.Errorf("todo HTTP translator is nil")
	}
	return &Handler{service: todoService, translator: translator}, nil
}

type createRequest struct {
	Title string `json:"title"`
}

type todoResponse struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
	CompletedAt *string `json:"completedAt"`
}

type listResponse struct {
	Items  []todoResponse `json:"items"`
	Offset int            `json:"offset"`
	Limit  int            `json:"limit"`
	Total  int64          `json:"total"`
}

// Create 处理 POST /api/v1/todos。
func (h *Handler) Create(ctx *httpx.Context) error {
	var request createRequest
	if err := ctx.BindJSON(&request); err != nil {
		return err
	}
	created, err := h.service.Create(ctx.Request.Context(), service.CreateCommand{Title: request.Title})
	if err != nil {
		return h.present(ctx.Request.Context(), ctx.Header(acceptLanguageHeader), err)
	}
	return ctx.JSON(http.StatusCreated, responseOf(created))
}

// Get 处理 GET /api/v1/todos/{id}。
func (h *Handler) Get(ctx *httpx.Context) error {
	todo, err := h.service.Get(ctx.Request.Context(), service.GetQuery{ID: ctx.Param("id")})
	if err != nil {
		return h.present(ctx.Request.Context(), ctx.Header(acceptLanguageHeader), err)
	}
	return ctx.JSON(http.StatusOK, responseOf(todo))
}

// List 处理 GET /api/v1/todos。
func (h *Handler) List(ctx *httpx.Context) error {
	offset, err := optionalInt(ctx.Query("offset"))
	if err != nil {
		return h.present(ctx.Request.Context(), ctx.Header(acceptLanguageHeader), fault.Wrap(err, fault.CodeInvalidArgument, "todo.http.offset", false))
	}
	limit, err := optionalInt(ctx.Query("limit"))
	if err != nil {
		return h.present(ctx.Request.Context(), ctx.Header(acceptLanguageHeader), fault.Wrap(err, fault.CodeInvalidArgument, "todo.http.limit", false))
	}
	result, err := h.service.List(ctx.Request.Context(), service.ListQuery{
		Status: ctx.Query("status"), Offset: offset, Limit: limit,
	})
	if err != nil {
		return h.present(ctx.Request.Context(), ctx.Header(acceptLanguageHeader), err)
	}
	items := make([]todoResponse, len(result.Items))
	for index, todo := range result.Items {
		items[index] = responseOf(todo)
	}
	return ctx.JSON(http.StatusOK, listResponse{
		Items: items, Offset: result.Offset, Limit: result.Limit, Total: result.Total,
	})
}

// Complete 处理 PATCH /api/v1/todos/{id}/complete。
func (h *Handler) Complete(ctx *httpx.Context) error {
	todo, err := h.service.Complete(ctx.Request.Context(), service.CompleteCommand{ID: ctx.Param("id")})
	if err != nil {
		return h.present(ctx.Request.Context(), ctx.Header(acceptLanguageHeader), err)
	}
	return ctx.JSON(http.StatusOK, responseOf(todo))
}

func optionalInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse integer %q: %w", value, err)
	}
	return parsed, nil
}

func responseOf(todo model.Todo) todoResponse {
	response := todoResponse{
		ID: todo.ID, Title: todo.Title, Status: string(todo.Status),
		CreatedAt: clock.RFC3339Millis(todo.CreatedAt), UpdatedAt: clock.RFC3339Millis(todo.UpdatedAt),
	}
	if todo.CompletedAt != nil {
		completed := clock.RFC3339Millis(*todo.CompletedAt)
		response.CompletedAt = &completed
	}
	return response
}

func (h *Handler) present(ctx context.Context, language string, err error) error {
	code := fault.CodeOf(err)
	status, reason, message := errorContract(code)
	if status == 0 {
		return err
	}
	translated, translateErr := h.translator.Translate(language, i18n.Text(
		"todo.error."+reason,
		i18n.WithDefault(message),
	))
	if translateErr != nil {
		return fmt.Errorf("translate todo HTTP error: %w", translateErr)
	}
	if ctx != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			err = contextErr
		}
	}
	return &httpx.StatusError{StatusCode: status, Code: reason, Message: translated, Err: err}
}

func errorContract(code fault.Code) (int, string, string) {
	switch code {
	case fault.CodeInvalidArgument:
		return http.StatusBadRequest, "todo_invalid_argument", "Todo 输入无效"
	case fault.CodeNotFound:
		return http.StatusNotFound, "todo_not_found", "Todo 不存在"
	case fault.CodeConflict:
		return http.StatusConflict, "todo_conflict", "Todo 已被其他操作修改"
	case fault.CodeUnavailable:
		return http.StatusServiceUnavailable, "todo_unavailable", "Todo 服务暂不可用"
	case fault.CodeTimeout:
		return http.StatusGatewayTimeout, "todo_timeout", "Todo 请求已超时"
	case fault.CodeCanceled:
		return http.StatusRequestTimeout, "todo_canceled", "Todo 请求已取消"
	default:
		return 0, "", ""
	}
}
