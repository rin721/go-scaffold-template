// Package middleware 实现 Todo 模块拥有的 HTTP 入站中间件。
package middleware

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/rin721/go-scaffold-template/pkg/httpx"
)

const (
	contentTypeHeader = "Content-Type"
	jsonMediaType     = "application/json"
	safeJSONMessage   = "Todo 创建请求必须使用 application/json"

	// ReasonUnsupportedMediaType 是 Todo 创建请求 Content-Type 错误的稳定公开 reason。
	ReasonUnsupportedMediaType = "todo_unsupported_media_type"
)

var errJSONContentTypeRequired = errors.New("todo JSON content type is required")

// RequireJSONContentType 要求请求显式声明 JSON media type。
//
// 它只校验 HTTP 协议元数据，不读取 Body，也不承载 Todo 业务不变量。
func RequireJSONContentType() httpx.Middleware {
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx *httpx.Context) error {
			value := ctx.Request.Header.Get(contentTypeHeader)
			if value == "" {
				return unsupportedMediaType(errJSONContentTypeRequired)
			}
			mediaType, _, err := mime.ParseMediaType(value)
			if err != nil {
				return unsupportedMediaType(fmt.Errorf("parse Todo Content-Type: %w", err))
			}
			if !strings.EqualFold(mediaType, jsonMediaType) {
				return unsupportedMediaType(fmt.Errorf("Todo Content-Type %q is not JSON", mediaType))
			}
			return next(ctx)
		}
	}
}

func unsupportedMediaType(cause error) error {
	return &httpx.StatusError{
		StatusCode: http.StatusUnsupportedMediaType,
		Code:       ReasonUnsupportedMediaType,
		Message:    safeJSONMessage,
		Err:        cause,
	}
}
