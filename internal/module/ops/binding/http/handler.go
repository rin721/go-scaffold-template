// Package httpbinding 实现 Ops module 的独立 management HTTP binding。
package httpbinding

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	configbinding "github.com/rin721/go-scaffold-template/internal/module/ops/binding/config"
	"github.com/rin721/go-scaffold-template/internal/module/ops/model"
	"github.com/rin721/go-scaffold-template/internal/module/ops/service"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
)

// Access 是 composition 连接 Auth module 时实现的 management scope 端口。
type Access interface {
	Authenticate(http.Handler) http.Handler
	Authorize(context.Context, string) error
}

// New 构造不包含 pprof 的固定 management 路由。
func New(ops *service.Service, metrics http.Handler, access Access, mode configbinding.AccessMode) (http.Handler, error) {
	if ops == nil || metrics == nil || access == nil {
		return nil, fmt.Errorf("management HTTP dependencies are incomplete")
	}
	mux := http.NewServeMux()
	mux.Handle("GET /startupz", probe(ops, model.ProbeStartup))
	mux.Handle("GET /livez", probe(ops, model.ProbeLiveness))
	mux.Handle("GET /readyz", probe(ops, model.ProbeReady))
	mux.Handle("GET /build", jsonHandler(func(context.Context) (any, error) { return ops.Build(), nil }))
	mux.Handle("GET /diagnostics", protect(access, model.OperationDiagnostics, jsonHandler(func(ctx context.Context) (any, error) { return ops.Diagnostics(ctx) })))
	switch mode {
	case configbinding.AccessDisabled:
	case configbinding.AccessPublic:
		mux.Handle("GET /metrics", metrics)
	case configbinding.AccessProtected:
		mux.Handle("GET /metrics", protect(access, model.OperationMetrics, metrics))
	default:
		return nil, fmt.Errorf("management metrics access %q is unsupported", mode)
	}
	return noStore(mux), nil
}

func probe(ops *service.Service, kind model.ProbeKind) http.Handler {
	return jsonHandler(func(ctx context.Context) (any, error) {
		result, passing, err := ops.Probe(ctx, kind)
		if err != nil {
			return statusValue{status: http.StatusServiceUnavailable, value: model.Probe{Status: "fail"}}, nil
		}
		status := http.StatusOK
		if !passing {
			status = http.StatusServiceUnavailable
		}
		return statusValue{status: status, value: result}, nil
	})
}

type statusValue struct {
	status int
	value  any
}

func jsonHandler(operation func(context.Context) (any, error)) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		value, err := operation(request.Context())
		if err != nil {
			http.Error(writer, "management operation failed", http.StatusInternalServerError)
			return
		}
		status := http.StatusOK
		if response, ok := value.(statusValue); ok {
			status, value = response.status, response.value
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(value)
	})
}

func protect(access Access, operation string, next http.Handler) http.Handler {
	authorized := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := access.Authorize(request.Context(), operation); err != nil {
			httpx.WriteProblem(writer, request, &httpx.StatusError{StatusCode: http.StatusForbidden, Code: "forbidden", Message: "management scope is required", Err: err})
			return
		}
		next.ServeHTTP(writer, request)
	})
	return access.Authenticate(authorized)
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(writer, request)
	})
}
