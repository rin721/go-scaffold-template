package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const problemContentType = "application/problem+json"

// Violation 描述一个不包含输入值的字段级协议错误。
type Violation struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// Problem 是项目公开的 RFC 9457 Problem Details 契约。
type Problem struct {
	Type       string      `json:"type"`
	Title      string      `json:"title"`
	Status     int         `json:"status"`
	Detail     string      `json:"detail,omitempty"`
	Instance   string      `json:"instance,omitempty"`
	Code       string      `json:"code"`
	Violations []Violation `json:"violations,omitempty"`
}

type responseStateWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseStateWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseStateWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *responseStateWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// ResponseCommitted 判断当前 response 是否已经提交 Header。
func ResponseCommitted(writer http.ResponseWriter) bool {
	state, ok := writer.(*responseStateWriter)
	return ok && state.status != 0
}

// WriteProblem 把项目错误写成 RFC 9457 响应；响应已提交时不会写第二份响应。
func WriteProblem(writer http.ResponseWriter, request *http.Request, err error) {
	if writer == nil || ResponseCommitted(writer) {
		return
	}
	problem, retryAfter := ProblemOf(request, err)
	writer.Header().Set("Content-Type", problemContentType)
	if retryAfter > 0 {
		writer.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
	}
	writer.WriteHeader(problem.Status)
	_ = json.NewEncoder(writer).Encode(problem)
}

// ProblemOf 把内部错误映射为不泄露 error chain 的稳定公开问题。
func ProblemOf(request *http.Request, err error) (Problem, int) {
	status := http.StatusInternalServerError
	code := errorCodeInternalServer
	detail := ""
	retryAfter := 0
	if statusErr, ok := asStatusError(err); ok {
		status = statusErrorStatusCode(statusErr)
		code = statusErrorCode(statusErr)
		detail = statusErrorMessage(statusErr)
		retryAfter = statusErr.RetryAfter
	} else {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			status, code, detail = http.StatusGatewayTimeout, "request_timeout", "request deadline exceeded"
		case errors.Is(err, context.Canceled):
			status, code, detail = http.StatusRequestTimeout, "request_canceled", "request canceled"
		}
	}
	if status < http.StatusBadRequest || status > 599 {
		status, code, detail = http.StatusInternalServerError, errorCodeInternalServer, ""
	}
	if !validProblemCode(code) {
		code = errorCodeInternalServer
	}
	problem := Problem{
		Type: "urn:go-scaffold-template:problem:" + code, Title: http.StatusText(status),
		Status: status, Detail: detail, Code: code,
	}
	if requestID, ok := RequestIDFromContext(requestContext(request)); ok {
		problem.Instance = "urn:request:" + url.PathEscape(requestID)
	}
	return problem, retryAfter
}

func validProblemCode(code string) bool {
	if code == "" || len(code) > 64 || code[0] < 'a' || code[0] > 'z' {
		return false
	}
	for _, character := range code[1:] {
		if character != '_' && (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}
	return !strings.Contains(code, "__")
}

func requestContext(request *http.Request) context.Context {
	if request == nil {
		return context.Background()
	}
	return request.Context()
}
