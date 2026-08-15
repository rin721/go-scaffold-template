// Package httpbinding 把生成的 OpenAPI transport 契约适配到 Todo UseCases。
package httpbinding

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rin721/go-scaffold-template/internal/module/todo/model"
	"github.com/rin721/go-scaffold-template/internal/module/todo/service"
	"github.com/rin721/go-scaffold-template/internal/transport/http/api"
	"github.com/rin721/go-scaffold-template/pkg/fault"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"github.com/rin721/go-scaffold-template/pkg/i18n"
)

// ActorAccess 是 Todo HTTP Handler 读取当前业务主体的窄端口。
type ActorAccess interface {
	Actor(context.Context) (service.Actor, bool)
}

// Operations 只声明 Todo 模块拥有的 HTTP operation，不扩张为整份应用 API。
type Operations interface {
	ListTodos(context.Context, api.ListTodosRequestObject) (api.ListTodosResponseObject, error)
	CreateTodo(context.Context, api.CreateTodoRequestObject) (api.CreateTodoResponseObject, error)
	GetTodo(context.Context, api.GetTodoRequestObject) (api.GetTodoResponseObject, error)
	CompleteTodo(context.Context, api.CompleteTodoRequestObject) (api.CompleteTodoResponseObject, error)
}

// Handler 把 Todo 生成 DTO 适配到项目 UseCases，不创建 Router 或绑定路由。
type Handler struct {
	service    service.UseCases
	translator i18n.Translator
	actors     ActorAccess
}

// NewHandler 创建无 I/O 副作用的 Todo operation Handler。
func NewHandler(todoService service.UseCases, translator i18n.Translator, actors ActorAccess) (*Handler, error) {
	if todoService == nil {
		return nil, fmt.Errorf("todo HTTP service is nil")
	}
	if translator == nil {
		return nil, fmt.Errorf("todo HTTP translator is nil")
	}
	if actors == nil {
		return nil, fmt.Errorf("Todo HTTP actor access is nil")
	}
	return &Handler{service: todoService, translator: translator, actors: actors}, nil
}

// ListTodos 把生成 query DTO 转换为稳定用例查询。
func (h *Handler) ListTodos(ctx context.Context, request api.ListTodosRequestObject) (api.ListTodosResponseObject, error) {
	actor, err := h.actor(ctx)
	if err != nil {
		return nil, err
	}
	query := service.ListQuery{Actor: actor}
	if request.Params.Status != nil {
		query.Status = string(*request.Params.Status)
	}
	if request.Params.Offset != nil {
		query.Offset = *request.Params.Offset
	}
	if request.Params.Limit != nil {
		query.Limit = *request.Params.Limit
	}
	result, err := h.service.List(ctx, query)
	if err != nil {
		return nil, h.present(ctx, err)
	}
	items := make([]api.Todo, len(result.Items))
	for index, todo := range result.Items {
		items[index] = todoResponse(todo)
	}
	return api.ListTodos200JSONResponse{
		Items: items, Offset: result.Offset, Limit: result.Limit, Total: result.Total,
	}, nil
}

// CreateTodo 把生成 request body 转换为稳定用例命令。
func (h *Handler) CreateTodo(ctx context.Context, request api.CreateTodoRequestObject) (api.CreateTodoResponseObject, error) {
	if request.Body == nil {
		return nil, &httpx.StatusError{StatusCode: http.StatusBadRequest, Code: "invalid_json", Message: "request body is required"}
	}
	actor, err := h.actor(ctx)
	if err != nil {
		return nil, err
	}
	created, err := h.service.Create(ctx, service.CreateCommand{Actor: actor, Title: request.Body.Title})
	if err != nil {
		return nil, h.present(ctx, err)
	}
	return api.CreateTodo201JSONResponse(todoResponse(created)), nil
}

// GetTodo 把生成 path DTO 转换为稳定用例查询。
func (h *Handler) GetTodo(ctx context.Context, request api.GetTodoRequestObject) (api.GetTodoResponseObject, error) {
	actor, err := h.actor(ctx)
	if err != nil {
		return nil, err
	}
	todo, err := h.service.Get(ctx, service.GetQuery{Actor: actor, ID: request.Id})
	if err != nil {
		return nil, h.present(ctx, err)
	}
	return api.GetTodo200JSONResponse(todoResponse(todo)), nil
}

// CompleteTodo 把生成 path DTO 转换为稳定用例命令。
func (h *Handler) CompleteTodo(ctx context.Context, request api.CompleteTodoRequestObject) (api.CompleteTodoResponseObject, error) {
	actor, err := h.actor(ctx)
	if err != nil {
		return nil, err
	}
	todo, err := h.service.Complete(ctx, service.CompleteCommand{Actor: actor, ID: request.Id})
	if err != nil {
		return nil, h.present(ctx, err)
	}
	return api.CompleteTodo200JSONResponse(todoResponse(todo)), nil
}

func todoResponse(todo model.Todo) api.Todo {
	response := api.Todo{
		Id: todo.ID, Title: todo.Title, Status: api.TodoStatus(todo.Status),
		CreatedAt: todo.CreatedAt.Truncate(time.Millisecond), UpdatedAt: todo.UpdatedAt.Truncate(time.Millisecond),
	}
	if todo.CompletedAt != nil {
		completed := todo.CompletedAt.Truncate(time.Millisecond)
		response.CompletedAt = &completed
	}
	return response
}

func (h *Handler) actor(ctx context.Context) (service.Actor, error) {
	actor, ok := h.actors.Actor(ctx)
	if !ok {
		return service.Actor{}, &httpx.StatusError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthenticated",
			Message:    "valid bearer authentication is required",
		}
	}
	return actor, nil
}

func (h *Handler) present(ctx context.Context, err error) error {
	status, reason, message := errorContract(fault.CodeOf(err))
	if status == 0 {
		return err
	}
	language := httpx.RequestLanguageFromContext(ctx)
	translated, translateErr := h.translator.Translate(language, i18n.Text(
		"todo.error."+reason,
		i18n.WithDefault(message),
	))
	if translateErr != nil {
		return fmt.Errorf("translate todo HTTP error: %w", translateErr)
	}
	if contextErr := ctx.Err(); contextErr != nil {
		err = contextErr
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
	case fault.CodePermissionDenied:
		return http.StatusForbidden, "permission_denied", "当前主体无权执行该操作"
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

var _ Operations = (*Handler)(nil)
