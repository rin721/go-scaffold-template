package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClientWithNilConfig(t *testing.T) {
	client, err := NewClient(nil)
	if err != nil {
		t.Fatalf("NewClient(nil) returned error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClient(nil) returned nil client")
	}
	client.CloseIdleConnections()
}

func TestClientSendsGETWithHeadersAndQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != string(MethodGet) {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Query().Get("name") != "rin" {
			t.Fatalf("query name = %q, want rin", r.URL.Query().Get("name"))
		}
		if r.Header.Get("X-Trace-ID") != "trace-1" {
			t.Fatalf("trace header = %q, want trace-1", r.Header.Get("X-Trace-ID"))
		}

		w.Header().Set(headerContentType, contentTypeJSON)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := mustNewClient(t, nil)
	var out struct {
		OK bool `json:"ok"`
	}
	resp, err := client.JSON(context.Background(), Request{
		URL:     server.URL + "/search",
		Headers: map[string]string{"X-Trace-ID": "trace-1"},
		Query:   map[string]string{"name": "rin"},
	}, &out)
	if err != nil {
		t.Fatalf("JSON returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !out.OK {
		t.Fatal("decoded ok = false, want true")
	}
}

func TestClientSendsPOSTJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != string(MethodPost) {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.Header.Get(headerContentType) != contentTypeJSON {
			t.Fatalf("content type = %q, want %q", r.Header.Get(headerContentType), contentTypeJSON)
		}

		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if payload["name"] != "rin" {
			t.Fatalf("payload name = %q, want rin", payload["name"])
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer server.Close()

	client := mustNewClient(t, nil)
	var out struct {
		ID string `json:"id"`
	}
	resp, err := client.JSON(context.Background(), Request{
		Method: MethodPost,
		URL:    server.URL + "/users",
		Body:   map[string]string{"name": "rin"},
	}, &out)
	if err != nil {
		t.Fatalf("JSON returned error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if out.ID != "1" {
		t.Fatalf("id = %q, want 1", out.ID)
	}
}

func TestClientUsesBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users" {
			t.Fatalf("path = %q, want /users", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := mustNewClient(t, &ClientConfig{BaseURL: server.URL})
	resp, err := client.Do(context.Background(), Request{URL: "/users"})
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestClientDoesNotRetryByDefault(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := mustNewClient(t, nil)
	_, err := client.Do(context.Background(), Request{URL: server.URL})
	if err == nil {
		t.Fatal("Do returned nil error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestClientRetriesTemporaryServerStatus(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := mustNewClient(t, &ClientConfig{
		RetryCount:    1,
		RetryWaitTime: time.Millisecond,
	})
	resp, err := client.Do(context.Background(), Request{URL: server.URL})
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestClientReturnsStatusErrorForUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := mustNewClient(t, nil)
	resp, err := client.Do(context.Background(), Request{URL: server.URL})
	if resp == nil {
		t.Fatal("response is nil")
	}
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error type = %T, want *StatusError", err)
	}
	if statusErr.StatusCode != http.StatusNotFound {
		t.Fatalf("status error code = %d, want 404", statusErr.StatusCode)
	}
	if !strings.Contains(statusErr.BodySnippet, "not found") {
		t.Fatalf("body snippet = %q, want not found", statusErr.BodySnippet)
	}
}

func TestClientAcceptStatusAllowsExpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"missing":true}`))
	}))
	defer server.Close()

	client := mustNewClient(t, nil)
	resp, err := client.Do(context.Background(), Request{
		URL:          server.URL,
		AcceptStatus: []int{http.StatusNotFound},
	})
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestClientRejectsInvalidRequest(t *testing.T) {
	client := mustNewClient(t, nil)

	tests := []struct {
		name string
		req  Request
		want string
	}{
		{name: "nil context", req: Request{URL: "http://example.com"}, want: "context"},
		{name: "empty url", req: Request{}, want: "url"},
		{name: "relative url", req: Request{URL: "/users"}, want: "scheme"},
		{name: "body conflict", req: Request{URL: "http://example.com", Body: "x", RawBody: []byte("x")}, want: "cannot both"},
		{name: "json encode", req: Request{URL: "http://example.com", Body: func() {}}, want: "encode request json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.name == "nil context" {
				ctx = nil
			}

			_, err := client.Do(ctx, tt.req)
			if err == nil {
				t.Fatal("Do returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte("late"))
	}))
	defer server.Close()

	client := mustNewClient(t, &ClientConfig{Timeout: 10 * time.Millisecond})
	_, err := client.Do(context.Background(), Request{URL: server.URL})
	if err == nil {
		t.Fatal("Do returned nil error")
	}
	if !strings.Contains(err.Error(), "do http request") {
		t.Fatalf("error %q does not contain request context", err.Error())
	}
}

func TestClientResponseBodyLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("too large"))
	}))
	defer server.Close()

	client := mustNewClient(t, &ClientConfig{MaxResponseBodyBytes: 3})
	_, err := client.Do(context.Background(), Request{URL: server.URL})
	if err == nil {
		t.Fatal("Do returned nil error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error %q does not contain exceeds", err.Error())
	}
}

func TestNewClientRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *ClientConfig
		want string
	}{
		{name: "negative timeout", cfg: &ClientConfig{Timeout: -1}, want: "timeout"},
		{name: "negative max body", cfg: &ClientConfig{MaxResponseBodyBytes: -1}, want: "max response"},
		{name: "negative retry count", cfg: &ClientConfig{RetryCount: -1}, want: "retry count"},
		{name: "negative retry wait", cfg: &ClientConfig{RetryWaitTime: -1}, want: "retry wait"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.cfg)
			if err == nil {
				t.Fatal("NewClient returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

func mustNewClient(t *testing.T, cfg *ClientConfig) Client {
	t.Helper()

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	return client
}
