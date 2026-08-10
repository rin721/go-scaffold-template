package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client 定义业务代码使用的 HTTP 客户端能力。
type Client interface {
	Do(ctx context.Context, req Request) (*Response, error)
	JSON(ctx context.Context, req Request, out any) (*Response, error)
	CloseIdleConnections()
}

type standardClient struct {
	client               *http.Client
	baseURL              string
	maxResponseBodyBytes int64
	retryCount           int
	retryWaitTime        time.Duration
	retryMaxWaitTime     time.Duration
}

// NewClient 根据配置创建 HTTP 客户端。
func NewClient(cfg *ClientConfig) (Client, error) {
	resolved, err := resolveClientConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve http client config: %w", err)
	}

	client := &http.Client{
		Timeout: resolved.Timeout,
	}
	if resolved.Transport != nil {
		client.Transport = resolved.Transport
	}

	return &standardClient{
		client:               client,
		baseURL:              resolved.BaseURL,
		maxResponseBodyBytes: resolved.MaxResponseBodyBytes,
		retryCount:           resolved.RetryCount,
		retryWaitTime:        resolved.RetryWaitTime,
		retryMaxWaitTime:     resolved.RetryMaxWaitTime,
	}, nil
}

func (c *standardClient) Do(ctx context.Context, req Request) (*Response, error) {
	requestURL, payload, contentType, err := prepareRequest(c.baseURL, req)
	if err != nil {
		return nil, fmt.Errorf("build http request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.retryCount; attempt++ {
		httpReq, err := buildHTTPRequest(ctx, requestURL, payload, contentType, req)
		if err != nil {
			return nil, fmt.Errorf("build http request: %w", err)
		}

		httpResp, err := c.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("do http request: %w", err)
			if !c.shouldRetry(ctx, attempt) {
				return nil, lastErr
			}
			continue
		}

		resp, err := readHTTPResponse(httpResp, c.maxResponseBodyBytes)
		if err != nil {
			return nil, fmt.Errorf("read response body: %w", err)
		}

		if !statusAccepted(resp.StatusCode, req.AcceptStatus) {
			statusErr := &StatusError{
				StatusCode:  resp.StatusCode,
				Code:        errorCodeHTTPStatus,
				Message:     http.StatusText(resp.StatusCode),
				BodySnippet: string(bodySnippet(resp.Body)),
			}
			if shouldRetryStatus(resp.StatusCode) && c.shouldRetry(ctx, attempt) {
				lastErr = statusErr
				continue
			}
			return resp, statusErr
		}

		return resp, nil
	}

	return nil, lastErr
}

func (c *standardClient) JSON(ctx context.Context, req Request, out any) (*Response, error) {
	resp, err := c.Do(ctx, req)
	if err != nil {
		return resp, err
	}
	if out == nil {
		return resp, nil
	}
	if err := resp.DecodeJSON(out); err != nil {
		return resp, err
	}
	return resp, nil
}

func (c *standardClient) CloseIdleConnections() {
	c.client.CloseIdleConnections()
}

func prepareRequest(baseURL string, req Request) (requestURL string, payload []byte, contentType string, err error) {
	if req.URL == "" {
		return "", nil, "", fmt.Errorf("request url is empty")
	}
	if req.Body != nil && req.RawBody != nil {
		return "", nil, "", fmt.Errorf("request body and raw body cannot both be set")
	}
	requestURL, err = resolveRequestURL(baseURL, req.URL)
	if err != nil {
		return "", nil, "", err
	}

	switch {
	case req.RawBody != nil:
		payload = append([]byte(nil), req.RawBody...)
		contentType = req.ContentType
	case req.Body != nil:
		payload, err = json.Marshal(req.Body)
		if err != nil {
			return "", nil, "", fmt.Errorf("encode request json: %w", err)
		}
		contentType = contentTypeJSON
	default:
		contentType = req.ContentType
	}

	return requestURL, payload, contentType, nil
}

func buildHTTPRequest(ctx context.Context, requestURL string, payload []byte, contentType string, req Request) (*http.Request, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}

	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}

	httpReq, err := http.NewRequestWithContext(ctx, string(methodOrDefault(req.Method)), requestURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if len(req.Headers) > 0 {
		for key, value := range req.Headers {
			httpReq.Header.Set(key, value)
		}
	}
	if len(req.Query) > 0 {
		query := httpReq.URL.Query()
		for key, value := range req.Query {
			query.Set(key, value)
		}
		httpReq.URL.RawQuery = query.Encode()
	}
	if contentType != "" {
		httpReq.Header.Set(headerContentType, contentType)
	}

	return httpReq, nil
}

func resolveRequestURL(baseURL string, requestURL string) (string, error) {
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return "", fmt.Errorf("parse request url %q: %w", requestURL, err)
	}
	if parsedURL.IsAbs() {
		if parsedURL.Scheme == "" || parsedURL.Host == "" {
			return "", fmt.Errorf("request url must include scheme and host")
		}
		return parsedURL.String(), nil
	}
	if baseURL == "" {
		return "", fmt.Errorf("request url must include scheme and host")
	}
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base url %q: %w", baseURL, err)
	}
	if !parsedBaseURL.IsAbs() || parsedBaseURL.Host == "" {
		return "", fmt.Errorf("base url must include scheme and host")
	}
	return parsedBaseURL.ResolveReference(parsedURL).String(), nil
}

func readHTTPResponse(httpResp *http.Response, maxBodyBytes int64) (*Response, error) {
	defer httpResp.Body.Close()

	body, err := readResponseBody(httpResp.Body, maxBodyBytes)
	if err != nil {
		return nil, err
	}

	return &Response{
		StatusCode: httpResp.StatusCode,
		Header:     httpResp.Header.Clone(),
		Body:       body,
	}, nil
}

func readResponseBody(body io.Reader, maxBodyBytes int64) ([]byte, error) {
	limited := io.LimitReader(body, maxBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if int64(len(data)) > maxBodyBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxBodyBytes)
	}
	return data, nil
}

func shouldRetryStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func (c *standardClient) shouldRetry(ctx context.Context, attempt int) bool {
	if attempt >= c.retryCount {
		return false
	}

	wait := retryWait(attempt, c.retryWaitTime, c.retryMaxWaitTime)
	if wait <= 0 {
		return true
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func retryWait(attempt int, wait time.Duration, maxWait time.Duration) time.Duration {
	for range attempt {
		wait *= 2
		if maxWait > 0 && wait > maxWait {
			return maxWait
		}
	}
	return wait
}

func bodySnippet(body []byte) []byte {
	const maxSnippetBytes = 512
	if len(body) <= maxSnippetBytes {
		return body
	}
	return body[:maxSnippetBytes]
}
