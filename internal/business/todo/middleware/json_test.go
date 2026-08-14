package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rin721/go-scaffold2/pkg/httpx"
)

func TestRequireJSONContentTypeAllowsJSONAndPreservesDownstreamError(t *testing.T) {
	t.Parallel()

	downstreamErr := errors.New("downstream failed")
	for _, contentType := range []string{"application/json", "application/json; charset=utf-8", "Application/JSON"} {
		contentType := contentType
		t.Run(contentType, func(t *testing.T) {
			t.Parallel()
			calls := 0
			handler := RequireJSONContentType()(func(*httpx.Context) error {
				calls++
				return downstreamErr
			})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/todos", nil)
			request.Header.Set(contentTypeHeader, contentType)
			err := handler(&httpx.Context{ResponseWriter: httptest.NewRecorder(), Request: request})
			if calls != 1 || !errors.Is(err, downstreamErr) {
				t.Fatalf("handler calls = %d, error = %v", calls, err)
			}
		})
	}
}

func TestRequireJSONContentTypeRejectsMissingMalformedAndNonJSON(t *testing.T) {
	t.Parallel()

	for _, contentType := range []string{"", "application/json; charset", "text/plain"} {
		contentType := contentType
		t.Run(contentType, func(t *testing.T) {
			t.Parallel()
			calls := 0
			handler := RequireJSONContentType()(func(*httpx.Context) error {
				calls++
				return nil
			})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/todos", nil)
			if contentType != "" {
				request.Header.Set(contentTypeHeader, contentType)
			}
			err := handler(&httpx.Context{ResponseWriter: httptest.NewRecorder(), Request: request})
			var statusErr *httpx.StatusError
			if calls != 0 || !errors.As(err, &statusErr) {
				t.Fatalf("handler calls = %d, error = %v", calls, err)
			}
			if statusErr.StatusCode != http.StatusUnsupportedMediaType ||
				statusErr.Code != ReasonUnsupportedMediaType || statusErr.Message != safeJSONMessage ||
				statusErr.Err == nil {
				t.Fatalf("StatusError = %#v", statusErr)
			}
		})
	}
}
