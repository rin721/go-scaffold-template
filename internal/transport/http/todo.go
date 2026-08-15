// Package httptransport 把生成的 OpenAPI transport 契约适配到项目 UseCases。
package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/oapi-codegen/nethttp-middleware"
	authmodel "github.com/rin721/go-scaffold-template/internal/module/auth/model"
	"github.com/rin721/go-scaffold-template/internal/module/todo/model"
	"github.com/rin721/go-scaffold-template/internal/module/todo/service"
	"github.com/rin721/go-scaffold-template/internal/transport/http/api"
	"github.com/rin721/go-scaffold-template/pkg/fault"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"github.com/rin721/go-scaffold-template/pkg/i18n"
)

const acceptLanguageHeader = "Accept-Language"

type requestLanguageContextKey struct{}

// OperationAuthorizer 是 transport 使用的 Auth module 最小授权端口。
type OperationAuthorizer interface {
	EnforceOperation(context.Context, authmodel.Principal, string) error
}

// TodoHandler 实现生成的 strict server interface，并只依赖项目 UseCases。
type TodoHandler struct {
	service    service.UseCases
	translator i18n.Translator
}

// NewTodoHandler 创建无 I/O 副作用的 Todo transport Adapter。
func NewTodoHandler(todoService service.UseCases, translator i18n.Translator) (*TodoHandler, error) {
	if todoService == nil {
		return nil, fmt.Errorf("todo HTTP service is nil")
	}
	if translator == nil {
		return nil, fmt.Errorf("todo HTTP translator is nil")
	}
	return &TodoHandler{service: todoService, translator: translator}, nil
}

// NewTodoHTTPHandler 组装生成 route、OpenAPI request validator 与统一错误边界。
func NewTodoHTTPHandler(todoService service.UseCases, translator i18n.Translator, authorizer OperationAuthorizer) (http.Handler, error) {
	handler, err := NewTodoHandler(todoService, translator)
	if err != nil {
		return nil, err
	}
	if authorizer == nil {
		return nil, fmt.Errorf("Todo HTTP operation authorizer is nil")
	}
	specification, err := api.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("load generated OpenAPI authority: %w", err)
	}
	specification.Servers = nil
	router := chi.NewRouter()
	router.NotFound(func(writer http.ResponseWriter, request *http.Request) {
		httpx.WriteProblem(writer, request, &httpx.StatusError{
			StatusCode: http.StatusNotFound, Code: "route_not_found", Message: "route not found",
		})
	})
	router.MethodNotAllowed(func(writer http.ResponseWriter, request *http.Request) {
		httpx.WriteProblem(writer, request, &httpx.StatusError{
			StatusCode: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: "method not allowed",
		})
	})
	router.Use(requireSingleJSONDocument)
	router.Use(nethttpmiddleware.OapiRequestValidatorWithOptions(specification, &nethttpmiddleware.Options{
		DoNotValidateServers: true,
		Options: openapi3filter.Options{
			AuthenticationFunc: func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
				if input == nil || input.SecuritySchemeName != "bearerAuth" {
					return authmodel.ErrUnauthenticated
				}
				if _, ok := authmodel.PrincipalFromContext(ctx); !ok {
					return authmodel.ErrUnauthenticated
				}
				return nil
			},
		},
		ErrorHandlerWithOpts: func(_ context.Context, validationErr error, writer http.ResponseWriter, request *http.Request, options nethttpmiddleware.ErrorHandlerOpts) {
			status, code, message := requestValidationProblem(specification, request, validationErr, options.StatusCode)
			httpx.WriteProblem(writer, request, &httpx.StatusError{
				StatusCode: status, Code: code, Message: message, Err: validationErr,
			})
		},
	}))
	strict := api.NewStrictHandlerWithOptions(handler, []api.StrictMiddlewareFunc{requestMetadata(authorizer)}, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(writer http.ResponseWriter, request *http.Request, err error) {
			httpx.WriteProblem(writer, request, &httpx.StatusError{
				StatusCode: http.StatusBadRequest, Code: "invalid_json", Message: "invalid JSON request body", Err: err,
			})
		},
		ResponseErrorHandlerFunc: httpx.WriteProblem,
	})
	return api.HandlerWithOptions(strict, api.ChiServerOptions{
		BaseRouter: router,
		ErrorHandlerFunc: func(writer http.ResponseWriter, request *http.Request, err error) {
			httpx.WriteProblem(writer, request, &httpx.StatusError{
				StatusCode: http.StatusBadRequest, Code: "invalid_parameter", Message: "invalid request parameter", Err: err,
			})
		},
	}), nil
}

func requestValidationProblem(specification *openapi3.T, request *http.Request, err error, suggestedStatus int) (int, string, string) {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		return http.StatusRequestEntityTooLarge, "request_body_too_large", "request body exceeds the configured limit"
	}
	var requestErr *openapi3filter.RequestError
	if errors.As(err, &requestErr) && strings.HasPrefix(requestErr.Reason, "header Content-Type has unexpected value") {
		return http.StatusUnsupportedMediaType, "unsupported_media_type", "request Content-Type is not supported"
	}
	switch suggestedStatus {
	case http.StatusNotFound:
		if specification != nil && specification.Paths != nil && request != nil {
			if pathItem := specification.Paths.Find(request.URL.Path); pathItem != nil && pathItem.GetOperation(request.Method) == nil {
				return http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed"
			}
		}
		return suggestedStatus, "route_not_found", "route not found"
	case http.StatusMethodNotAllowed:
		return suggestedStatus, "method_not_allowed", "method not allowed"
	default:
		return http.StatusBadRequest, "invalid_request", "request does not match the OpenAPI contract"
	}
}

// requireSingleJSONDocument 拒绝首个 JSON 值后的尾随内容，避免生成解码器只消费首值。
func requireSingleJSONDocument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Body == nil || request.Body == http.NoBody {
			next.ServeHTTP(writer, request)
			return
		}
		mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			next.ServeHTTP(writer, request)
			return
		}
		payload, err := io.ReadAll(request.Body)
		if err != nil {
			httpx.WriteProblem(writer, request, &httpx.StatusError{
				StatusCode: http.StatusBadRequest, Code: "invalid_json", Message: "invalid JSON request body", Err: err,
			})
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(payload))
		decoder := json.NewDecoder(bytes.NewReader(payload))
		var value any
		if err := decoder.Decode(&value); err == nil {
			var trailing any
			if trailingErr := decoder.Decode(&trailing); !errors.Is(trailingErr, io.EOF) {
				httpx.WriteProblem(writer, request, &httpx.StatusError{
					StatusCode: http.StatusBadRequest, Code: "invalid_request", Message: "request must contain one JSON document", Err: trailingErr,
				})
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}

// ListTodos 把生成 query DTO 转换为稳定用例查询。
func (h *TodoHandler) ListTodos(ctx context.Context, request api.ListTodosRequestObject) (api.ListTodosResponseObject, error) {
	query := service.ListQuery{Actor: actorFromContext(ctx)}
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
func (h *TodoHandler) CreateTodo(ctx context.Context, request api.CreateTodoRequestObject) (api.CreateTodoResponseObject, error) {
	if request.Body == nil {
		return nil, &httpx.StatusError{StatusCode: http.StatusBadRequest, Code: "invalid_json", Message: "request body is required"}
	}
	created, err := h.service.Create(ctx, service.CreateCommand{Actor: actorFromContext(ctx), Title: request.Body.Title})
	if err != nil {
		return nil, h.present(ctx, err)
	}
	return api.CreateTodo201JSONResponse(todoResponse(created)), nil
}

// GetTodo 把生成 path DTO 转换为稳定用例查询。
func (h *TodoHandler) GetTodo(ctx context.Context, request api.GetTodoRequestObject) (api.GetTodoResponseObject, error) {
	todo, err := h.service.Get(ctx, service.GetQuery{Actor: actorFromContext(ctx), ID: request.Id})
	if err != nil {
		return nil, h.present(ctx, err)
	}
	return api.GetTodo200JSONResponse(todoResponse(todo)), nil
}

// CompleteTodo 把生成 path DTO 转换为稳定用例命令。
func (h *TodoHandler) CompleteTodo(ctx context.Context, request api.CompleteTodoRequestObject) (api.CompleteTodoResponseObject, error) {
	todo, err := h.service.Complete(ctx, service.CompleteCommand{Actor: actorFromContext(ctx), ID: request.Id})
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

func actorFromContext(ctx context.Context) service.Actor {
	principal, _ := authmodel.PrincipalFromContext(ctx)
	scopes := make([]string, len(principal.Scopes))
	for index, scope := range principal.Scopes {
		scopes[index] = string(scope)
	}
	return service.Actor{Subject: principal.Subject, Kind: string(principal.Kind), Scopes: scopes}
}

func (h *TodoHandler) present(ctx context.Context, err error) error {
	status, reason, message := errorContract(fault.CodeOf(err))
	if status == 0 {
		return err
	}
	language, _ := ctx.Value(requestLanguageContextKey{}).(string)
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

func requestMetadata(authorizer OperationAuthorizer) api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, strictName string) api.StrictHandlerFunc {
		return func(ctx context.Context, writer http.ResponseWriter, request *http.Request, input any) (any, error) {
			operation, ok := api.OperationForStrictName(strictName)
			if !ok {
				return nil, fmt.Errorf("strict operation %q is absent from generated inventory", strictName)
			}
			ctx = httpx.WithOperationID(ctx, string(operation.ID))
			ctx = context.WithValue(ctx, requestLanguageContextKey{}, request.Header.Get(acceptLanguageHeader))
			principal, authenticated := authmodel.PrincipalFromContext(ctx)
			if !authenticated {
				return nil, &httpx.StatusError{
					StatusCode: http.StatusUnauthorized, Code: "unauthenticated", Message: "valid bearer authentication is required", Err: authmodel.ErrUnauthenticated,
				}
			}
			if err := authorizer.EnforceOperation(ctx, principal, string(operation.ID)); err != nil {
				if errors.Is(err, authmodel.ErrPermissionDenied) {
					return nil, &httpx.StatusError{
						StatusCode: http.StatusForbidden, Code: "permission_denied", Message: "the authenticated principal is not authorized", Err: err,
					}
				}
				return nil, err
			}
			request = request.WithContext(ctx)
			return next(ctx, writer, request, input)
		}
	}
}

var _ api.StrictServerInterface = (*TodoHandler)(nil)
