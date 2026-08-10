package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Response 表示 HTTP 客户端响应。
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// DecodeJSON 将响应体解码为 JSON。
func (r *Response) DecodeJSON(out any) error {
	if r == nil {
		return fmt.Errorf("response is nil")
	}
	if out == nil {
		return fmt.Errorf("json decode target is nil")
	}
	if err := json.Unmarshal(r.Body, out); err != nil {
		return fmt.Errorf("decode response json: %w", err)
	}
	return nil
}

// String 返回响应体字符串。
func (r *Response) String() string {
	if r == nil {
		return ""
	}
	return string(r.Body)
}
