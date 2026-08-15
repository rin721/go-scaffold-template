package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteProblemUsesRFC9457AndRedactsInternalError(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/private?id=secret", nil)
	request = request.WithContext(context.WithValue(request.Context(), requestIDContextKey{}, "request-123"))
	recorder := httptest.NewRecorder()

	WriteProblem(recorder, request, errors.New("password=secret sql=select *"))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != problemContentType {
		t.Fatalf("Content-Type = %q", contentType)
	}
	var problem Problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode Problem: %v", err)
	}
	if problem.Code != errorCodeInternalServer || problem.Instance != "urn:request:request-123" || problem.Status != recorder.Code {
		t.Fatalf("problem = %#v", problem)
	}
	if problem.Detail != "" {
		t.Fatalf("internal detail leaked: %q", problem.Detail)
	}
}

func TestProblemOfPreservesSafeStatusContractAndRetryAfter(t *testing.T) {
	problem, retryAfter := ProblemOf(nil, &StatusError{
		StatusCode: http.StatusTooManyRequests, Code: "rate_limited",
		Message: "request quota exceeded", RetryAfter: 3, Err: errors.New("private limiter state"),
	})
	if problem.Status != http.StatusTooManyRequests || problem.Code != "rate_limited" ||
		problem.Detail != "request quota exceeded" || retryAfter != 3 {
		t.Fatalf("ProblemOf() = %#v, retry=%d", problem, retryAfter)
	}
}

func TestProblemOfPreservesCancellationClass(t *testing.T) {
	problem, _ := ProblemOf(nil, context.Canceled)
	if problem.Status != http.StatusRequestTimeout || problem.Code != "request_canceled" {
		t.Fatalf("ProblemOf(context.Canceled) = %#v", problem)
	}
}

func TestDefaultRouterUsesProblemForNotFoundAndMethodNotAllowed(t *testing.T) {
	router := NewRouter(nil)
	router.Handle(MethodGet, "/known", func(ctx *Context) error { return ctx.NoContent(http.StatusNoContent) })
	tests := []struct {
		method string
		path   string
		status int
		code   string
	}{
		{method: http.MethodGet, path: "/missing", status: http.StatusNotFound, code: "route_not_found"},
		{method: http.MethodPost, path: "/known", status: http.StatusMethodNotAllowed, code: "method_not_allowed"},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		var problem Problem
		if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
			t.Fatalf("decode %s %s Problem: %v", test.method, test.path, err)
		}
		if recorder.Code != test.status || problem.Code != test.code {
			t.Fatalf("%s %s = status %d problem %#v", test.method, test.path, recorder.Code, problem)
		}
	}
}

func TestWriteProblemDoesNotOverwriteCommittedResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	state := &responseStateWriter{ResponseWriter: recorder}
	state.WriteHeader(http.StatusAccepted)
	_, _ = state.Write([]byte("accepted"))
	WriteProblem(state, httptest.NewRequest(http.MethodGet, "/", nil), errors.New("late"))
	if recorder.Code != http.StatusAccepted || recorder.Body.String() != "accepted" {
		t.Fatalf("committed response changed: status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
