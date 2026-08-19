// Package httptransport 负责把完整应用契约（模块声明）一次绑定为 HTTP routes。
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
	"github.com/getkin/kin-openapi/routers"
	"github.com/go-chi/chi/v5"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"github.com/rin721/go-scaffold-template/pkg/httpx/contract"
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

// Dispatcher 向 route binding 提供已聚合的模块契约、operation 表与运行期执行器。
type Dispatcher interface {
	// Modules 返回全部模块契约（用于渲染校验用 OpenAPI 文档）。
	Modules() []contract.Module
	// Operations 返回已注册的 operation 表（ID/method/path/policy/security）。
	Operations() []contract.Operation
	// Handler 返回 operation 对应的运行期执行器；不存在时返回 ok=false。
	Handler(operationID contract.OperationID) (contract.Handler, bool)
}

// NewRouteBinding 把完整契约、验证和 operation middleware 绑定为唯一业务路由树。
func NewRouteBinding(dispatcher Dispatcher, gate OperationGate) (http.Handler, error) {
	if dispatcher == nil {
		return nil, fmt.Errorf("HTTP operation dispatcher is nil")
	}
	specification, err := buildSpec(dispatcher.Modules())
	if err != nil {
		return nil, err
	}
	if nilInterface(gate) {
		return nil, fmt.Errorf("HTTP operation gate is nil")
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

	for _, operation := range dispatcher.Operations() {
		handler, ok := dispatcher.Handler(operation.ID)
		if !ok {
			return nil, fmt.Errorf("operation %q has no runtime handler", operation.ID)
		}
		bindOperation(router, specification, operation, handler, gate)
	}
	return router, nil
}

func buildSpec(modules []contract.Module) (*openapi3.T, error) {
	if len(modules) == 0 {
		return nil, fmt.Errorf("no module contracts provided")
	}
	info := contract.Info{Title: "application HTTP API", Version: "1.0.0-rc.1", Description: "generated from module contracts"}
	document, err := contract.BuildDocument(info, modules)
	if err != nil {
		return nil, fmt.Errorf("build contract document: %w", err)
	}
	yamlBytes, err := document.MarshalYAML()
	if err != nil {
		return nil, fmt.Errorf("marshal contract document: %w", err)
	}
	specification, err := openapi3.NewLoader().LoadFromData(yamlBytes)
	if err != nil {
		return nil, fmt.Errorf("load contract spec: %w", err)
	}
	return specification, nil
}

// bindOperation 注册一个 method/path 的 handler：校验请求、写 metadata、执行 gate、调用模块执行器。
func bindOperation(router chi.Router, specification *openapi3.T, operation contract.Operation, handler contract.Handler, gate OperationGate) {
	router.Method(string(operation.Method), operation.Path, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		pathParams := collectPathParams(operation, request)
		if err := validateRequest(specification, operation, request, pathParams, gate); err != nil {
			writeValidationError(writer, request, err)
			return
		}

		ctx := request.Context()
		operationID := string(operation.ID)
		ctx = httpx.WithOperationID(ctx, operationID)
		ctx = httpx.WithRequestLanguage(ctx, request.Header.Get(acceptLanguageHeader))
		request = request.WithContext(ctx)
		request = contract.WithPathValues(request, pathParams)

		if err := gate.Enforce(ctx, operationID); err != nil {
			writeGateError(writer, request, err)
			return
		}

		if err := handler.ServeHTTP(writer, request); err != nil {
			httpx.WriteProblem(writer, request, err)
		}
	}))
}

func collectPathParams(operation contract.Operation, request *http.Request) map[string]string {
	values := make(map[string]string)
	for _, param := range operation.Params {
		if param.Location != contract.ParamPath {
			continue
		}
		if value := chi.URLParam(request, param.Name); value != "" {
			values[param.Name] = value
		}
	}
	return values
}

// validateRequest 用规范校验请求（含路径参数与认证）。
func validateRequest(specification *openapi3.T, operation contract.Operation, request *http.Request, pathParams map[string]string, gate OperationGate) error {
	pathItem := specification.Paths.Find(operation.Path)
	if pathItem == nil {
		return fmt.Errorf("contract path %q is absent from rendered OpenAPI", operation.Path)
	}
	specOperation := operationForMethod(pathItem, operation.Method)
	if specOperation == nil {
		return fmt.Errorf("contract operation %q is absent from rendered OpenAPI", operation.ID)
	}
	route := &routers.Route{
		Spec:      specification,
		Path:      operation.Path,
		PathItem:  pathItem,
		Method:    string(operation.Method),
		Operation: specOperation,
	}
	options := &openapi3filter.Options{
		AuthenticationFunc: func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
			if input == nil || input.SecuritySchemeName != bearerSecurityScheme {
				return ErrUnauthenticated
			}
			return gate.Authenticate(ctx)
		},
	}
	input := &openapi3filter.RequestValidationInput{
		Request:    request,
		PathParams: pathParams,
		Route:      route,
		Options:    options,
	}
	return openapi3filter.ValidateRequest(request.Context(), input)
}

func operationForMethod(pathItem *openapi3.PathItem, method contract.Method) *openapi3.Operation {
	switch method {
	case contract.MethodGet:
		return pathItem.Get
	case contract.MethodPost:
		return pathItem.Post
	case contract.MethodPut:
		return pathItem.Put
	case contract.MethodPatch:
		return pathItem.Patch
	case contract.MethodDelete:
		return pathItem.Delete
	default:
		return nil
	}
}

func writeValidationError(writer http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, ErrUnauthenticated):
		httpx.WriteProblem(writer, request, &httpx.StatusError{
			StatusCode: http.StatusUnauthorized, Code: "unauthenticated", Message: "valid bearer authentication is required", Err: err,
		})
	case errors.Is(err, ErrPermissionDenied):
		httpx.WriteProblem(writer, request, &httpx.StatusError{
			StatusCode: http.StatusForbidden, Code: "permission_denied", Message: "the authenticated principal is not authorized", Err: err,
		})
	case isRequestValidationError(err):
		status, code, message := requestValidationProblem(request, err)
		httpx.WriteProblem(writer, request, &httpx.StatusError{StatusCode: status, Code: code, Message: message, Err: err})
	default:
		httpx.WriteProblem(writer, request, &httpx.StatusError{
			StatusCode: http.StatusInternalServerError, Code: "internal_server_error", Message: "internal server error", Err: err,
		})
	}
}

func isRequestValidationError(err error) bool {
	var requestErr *openapi3filter.RequestError
	return errors.As(err, &requestErr)
}

func requestValidationProblem(request *http.Request, err error) (int, string, string) {
	var requestErr *openapi3filter.RequestError
	if errors.As(err, &requestErr) && strings.HasPrefix(requestErr.Reason, "header Content-Type has unexpected value") {
		return http.StatusUnsupportedMediaType, "unsupported_media_type", "request Content-Type is not supported"
	}
	return http.StatusBadRequest, "invalid_request", "request does not match the OpenAPI contract"
}

func writeGateError(writer http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, ErrUnauthenticated):
		httpx.WriteProblem(writer, request, &httpx.StatusError{
			StatusCode: http.StatusUnauthorized, Code: "unauthenticated", Message: "valid bearer authentication is required", Err: err,
		})
	case errors.Is(err, ErrPermissionDenied):
		httpx.WriteProblem(writer, request, &httpx.StatusError{
			StatusCode: http.StatusForbidden, Code: "permission_denied", Message: "the authenticated principal is not authorized", Err: err,
		})
	default:
		httpx.WriteProblem(writer, request, err)
	}
}

// requireSingleJSONDocument 拒绝首个 JSON 值后的尾随内容。
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
