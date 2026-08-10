package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Context 封装一次入站 HTTP 请求和响应。
type Context struct {
	ResponseWriter http.ResponseWriter
	Request        *http.Request
}

// JSON 写入 JSON 响应。
func (c *Context) JSON(statusCode int, value any) error {
	c.ResponseWriter.Header().Set(headerContentType, contentTypeJSON)
	c.ResponseWriter.WriteHeader(statusCode)
	if value == nil {
		return nil
	}
	if err := json.NewEncoder(c.ResponseWriter).Encode(value); err != nil {
		return fmt.Errorf("encode response json: %w", err)
	}
	return nil
}

// Text 写入文本响应。
func (c *Context) Text(statusCode int, value string) error {
	c.ResponseWriter.Header().Set(headerContentType, contentTypeText)
	c.ResponseWriter.WriteHeader(statusCode)
	if _, err := c.ResponseWriter.Write([]byte(value)); err != nil {
		return fmt.Errorf("write text response: %w", err)
	}
	return nil
}

// NoContent 写入无响应体状态。
func (c *Context) NoContent(statusCode int) error {
	c.ResponseWriter.WriteHeader(statusCode)
	return nil
}

// BindJSON 将请求体绑定到目标对象。
func (c *Context) BindJSON(out any) error {
	if out == nil {
		return &StatusError{StatusCode: http.StatusBadRequest, Code: errorCodeInvalidJSON, Message: "json decode target is nil"}
	}
	if err := json.NewDecoder(c.Request.Body).Decode(out); err != nil {
		return &StatusError{StatusCode: http.StatusBadRequest, Code: errorCodeInvalidJSON, Message: "invalid json request body", Err: err}
	}
	return nil
}

// Param 读取路径参数。
func (c *Context) Param(name string) string {
	return chi.URLParam(c.Request, name)
}

// Query 读取查询参数。
func (c *Context) Query(name string) string {
	return c.Request.URL.Query().Get(name)
}

// Header 读取请求头。
func (c *Context) Header(name string) string {
	return c.Request.Header.Get(name)
}
