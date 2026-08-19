package contract

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// Handler 是 Operation 的运行期执行器：绑定 HTTP 请求、调用模块 typed 实现、编码 typed 响应。
// 由模块通过 JSONBody/Query 等泛型构造器生成，transport 单一 binder 只调用它。
type Handler interface {
	// ServeHTTP 处理一次已完成认证/授权的请求。返回的 error 由 transport 统一呈现为 Problem。
	ServeHTTP(w http.ResponseWriter, r *http.Request) error
}

// JSONBody 构造一个解码请求体、调用 typed 函数、以给定状态码写 JSON 响应的 Handler。
func JSONBody[Req, Resp any](handle func(context.Context, Req) (Resp, error), successStatus int) Handler {
	return &jsonBodyHandler[Req, Resp]{handle: handle, status: successStatus}
}

type jsonBodyHandler[Req, Resp any] struct {
	handle func(context.Context, Req) (Resp, error)
	status int
}

func (h *jsonBodyHandler[Req, Resp]) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	var request Req
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return newBindingError("decode JSON body", err)
	}
	resp, err := h.handle(r.Context(), request)
	if err != nil {
		return err
	}
	if err := EncodeJSONResponse(w, h.status, resp); err != nil {
		return err
	}
	return nil
}

// Query 构造一个从查询字符串/路径参数绑定 Params、调用 typed 函数、以给定状态码写 JSON 响应的
// Handler。Params 必须是指针类型，字段通过 form/json tag 绑定。
func Query[Params, Resp any](handle func(context.Context, Params) (Resp, error), successStatus int) Handler {
	return &queryHandler[Params, Resp]{handle: handle, status: successStatus}
}

type queryHandler[Params, Resp any] struct {
	handle func(context.Context, Params) (Resp, error)
	status int
}

func (h *queryHandler[Params, Resp]) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	var params Params
	value := reflect.ValueOf(&params)
	if err := BindQueryMap(r.URL.Query(), value.Interface()); err != nil {
		return newBindingError("bind query params", err)
	}
	resp, err := h.handle(r.Context(), params)
	if err != nil {
		return err
	}
	if err := EncodeJSONResponse(w, h.status, resp); err != nil {
		return err
	}
	return nil
}

// PathValuesKey 是请求上下文中路径参数的键（由 transport binder 写入）。使用不可导出的键类型避免
// 与业务共享上下文冲突。
type pathValuesKey struct{}

// WithPathValues 把路径参数写入请求上下文。
func WithPathValues(r *http.Request, values map[string]string) *http.Request {
	if values == nil {
		values = map[string]string{}
	}
	return r.WithContext(context.WithValue(r.Context(), pathValuesKey{}, values))
}

// PathValuesFromContext 读取请求上下文中的路径参数。
func PathValuesFromContext(ctx context.Context) map[string]string {
	values, _ := ctx.Value(pathValuesKey{}).(map[string]string)
	return values
}

// Path 构造一个读取路径参数、调用 typed 函数、写 JSON 响应的 Handler。
func Path[Resp any](paramName string, handle func(context.Context, string) (Resp, error), successStatus int) Handler {
	return &pathHandler[Resp]{paramName: paramName, handle: handle, status: successStatus}
}

type pathHandler[Resp any] struct {
	paramName string
	handle    func(context.Context, string) (Resp, error)
	status    int
}

func (h *pathHandler[Resp]) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	id, ok := PathValuesFromContext(r.Context())[h.paramName]
	if !ok || id == "" {
		return newBindingError("path param "+h.paramName, fmt.Errorf("missing"))
	}
	resp, err := h.handle(r.Context(), id)
	if err != nil {
		return err
	}
	if err := EncodeJSONResponse(w, h.status, resp); err != nil {
		return err
	}
	return nil
}

// EncodeJSONResponse 写 JSON 响应。
func EncodeJSONResponse(w http.ResponseWriter, status int, value any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return fmt.Errorf("encode JSON response: %w", err)
	}
	return nil
}

// newBindingError 构造请求绑定错误（400 语义由 transport 呈现）。
func newBindingError(reason string, err error) error {
	return fmt.Errorf("%s: %w", reason, err)
}

// BindQueryMap 把查询字符串绑定到带 form/json tag 的结构字段。仅支持基础类型与指针。
func BindQueryMap(values map[string][]string, target any) error {
	reflected := reflect.ValueOf(target)
	if reflected.Kind() != reflect.Ptr || reflected.IsNil() {
		return fmt.Errorf("query bind target must be a non-nil pointer")
	}
	element := reflected.Elem()
	if element.Kind() != reflect.Struct {
		return nil
	}
	typeOf := element.Type()
	for index := 0; index < element.NumField(); index++ {
		field := element.Field(index)
		fieldType := typeOf.Field(index)
		name, ok := queryFieldName(fieldType)
		if !ok {
			continue
		}
		raw, exists := values[name]
		if !exists || len(raw) == 0 {
			continue
		}
		if err := assignScalar(field, raw[0]); err != nil {
			return fmt.Errorf("bind query parameter %q: %w", name, err)
		}
	}
	return nil
}

func queryFieldName(field reflect.StructField) (string, bool) {
	if tag, ok := field.Tag.Lookup("form"); ok {
		name := strings.Split(tag, ",")[0]
		if name != "" && name != "-" {
			return name, true
		}
	}
	if tag, ok := field.Tag.Lookup("json"); ok {
		name := strings.Split(tag, ",")[0]
		if name != "" && name != "-" {
			return name, true
		}
	}
	return "", false
}

func assignScalar(field reflect.Value, value string) error {
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return assignScalar(field.Elem(), value)
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("expected integer")
		}
		field.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("expected unsigned integer")
		}
		field.SetUint(parsed)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("expected boolean")
		}
		field.SetBool(parsed)
	default:
		return fmt.Errorf("unsupported query field type %s", field.Kind())
	}
	return nil
}
