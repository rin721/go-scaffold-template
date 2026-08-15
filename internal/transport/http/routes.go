// Package httptransport 负责把完整应用 strict API 一次绑定为 HTTP routes。
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
	"reflect"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/oapi-codegen/nethttp-middleware"
	"github.com/rin721/go-scaffold-template/internal/transport/http/api"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
)

const (
	acceptLanguageHeader = "Accept-Language"
	bearerSecurityScheme = "bearerAuth"
)

var (
	// ErrUnauthenticated 表示应用路由没有可用的已验证主体。
	ErrUnauthenticated = errors.New("HTTP request is unauthenticated")
	// ErrPermissionDenied 表示当前主体无权访问目标 operation。
	ErrPermissionDenied = errors.New("HTTP operation is not authorized")
)

// OperationGate 是应用 route binding 使用的认证与 operation 授权窄端口。
type OperationGate interface {
	Authenticate(context.Context) error
	Enforce(context.Context, string) error
}

// NewRouteBinding 把完整 strict API、验证和 operation middleware 绑定为唯一业务路由树。
func NewRouteBinding(server api.StrictServerInterface, gate OperationGate) (http.Handler, error) {
	if nilInterface(server) {
		return nil, fmt.Errorf("strict API server is nil")
	}
	if nilInterface(gate) {
		return nil, fmt.Errorf("HTTP operation gate is nil")
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
				if input == nil || input.SecuritySchemeName != bearerSecurityScheme {
					return ErrUnauthenticated
				}
				return gate.Authenticate(ctx)
			},
		},
		ErrorHandlerWithOpts: func(_ context.Context, validationErr error, writer http.ResponseWriter, request *http.Request, options nethttpmiddleware.ErrorHandlerOpts) {
			status, code, message := requestValidationProblem(specification, request, validationErr, options.StatusCode)
			httpx.WriteProblem(writer, request, &httpx.StatusError{
				StatusCode: status, Code: code, Message: message, Err: validationErr,
			})
		},
	}))
	strict := api.NewStrictHandlerWithOptions(server, []api.StrictMiddlewareFunc{requestMetadata(gate)}, api.StrictHTTPServerOptions{
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

func requestMetadata(gate OperationGate) api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, strictName string) api.StrictHandlerFunc {
		return func(ctx context.Context, writer http.ResponseWriter, request *http.Request, input any) (any, error) {
			operation, ok := api.OperationForStrictName(strictName)
			if !ok {
				return nil, fmt.Errorf("strict operation %q is absent from generated inventory", strictName)
			}
			ctx = httpx.WithOperationID(ctx, string(operation.ID))
			ctx = httpx.WithRequestLanguage(ctx, request.Header.Get(acceptLanguageHeader))
			if err := gate.Enforce(ctx, string(operation.ID)); err != nil {
				switch {
				case errors.Is(err, ErrUnauthenticated):
					return nil, &httpx.StatusError{
						StatusCode: http.StatusUnauthorized, Code: "unauthenticated", Message: "valid bearer authentication is required", Err: err,
					}
				case errors.Is(err, ErrPermissionDenied):
					return nil, &httpx.StatusError{
						StatusCode: http.StatusForbidden, Code: "permission_denied", Message: "the authenticated principal is not authorized", Err: err,
					}
				default:
					return nil, err
				}
			}
			request = request.WithContext(ctx)
			return next(ctx, writer, request, input)
		}
	}
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
	case http.StatusUnauthorized:
		return suggestedStatus, "unauthenticated", "valid bearer authentication is required"
	case http.StatusForbidden:
		return suggestedStatus, "permission_denied", "the authenticated principal is not authorized"
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

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
