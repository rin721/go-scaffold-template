// Package middleware 实现 Ops module 自有的 management HTTP 约束。
package middleware

import (
	"context"
	"net/http"
	"time"
)

// Management 为独立管理面施加 body、并发和 application deadline 预算。
func Management(next http.Handler, requestTimeout time.Duration, maxBodyBytes int64, maxInFlight int) http.Handler {
	slots := make(chan struct{}, maxInFlight)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			writer.Header().Set("Retry-After", "1")
			http.Error(writer, "management overloaded", http.StatusServiceUnavailable)
			return
		}
		if request.ContentLength > maxBodyBytes {
			http.Error(writer, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		request.Body = http.MaxBytesReader(writer, request.Body, maxBodyBytes)
		ctx, cancel := context.WithTimeout(request.Context(), requestTimeout)
		defer cancel()
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}
