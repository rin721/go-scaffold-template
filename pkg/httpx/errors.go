package httpx

import (
	"errors"
	"fmt"
	"net/http"
)

// StatusError 表示 HTTP 状态码不符合预期。
type StatusError struct {
	StatusCode  int
	Code        string
	Message     string
	RetryAfter  int
	BodySnippet string
	Err         error
}

func (e *StatusError) Error() string {
	if e == nil {
		return ""
	}
	message := e.Message
	if message == "" {
		message = http.StatusText(e.StatusCode)
	}
	if message == "" {
		message = "unexpected http status"
	}
	return fmt.Sprintf("http status %d: %s", e.StatusCode, message)
}

func (e *StatusError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func statusErrorCode(err *StatusError) string {
	if err == nil || err.Code == "" {
		return errorCodeHTTPStatus
	}
	return err.Code
}

func statusErrorMessage(err *StatusError) string {
	if err == nil {
		return http.StatusText(http.StatusInternalServerError)
	}
	if err.Message == "" {
		return http.StatusText(statusErrorStatusCode(err))
	}
	return err.Message
}

func statusErrorStatusCode(err *StatusError) int {
	if err == nil || err.StatusCode < http.StatusContinue || err.StatusCode > 999 {
		return http.StatusInternalServerError
	}
	return err.StatusCode
}

func asStatusError(err error) (*StatusError, bool) {
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		return statusErr, true
	}
	return nil, false
}
