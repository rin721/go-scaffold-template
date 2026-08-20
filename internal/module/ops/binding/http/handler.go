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
	"github.com/rin721/go-scaffold-template/pkg/logger"
)

// Access 是 composition 连接 Auth module 时实现的 management scope 端口。
type Access interface {
	Authenticate(http.Handler) http.Handler
	Authorize(context.Context, string) error
}

// New 构造不包含 pprof 的固定 management 路由。
func New(ops *service.Service, metrics http.Handler, access Access, mode configbinding.AccessMode, logging logger.Logger) (http.Handler, error) {
	if ops == nil || metrics == nil || access == nil || logging == nil {
		return nil, fmt.Errorf("management HTTP dependencies are incomplete")
	}
	mux := http.NewServeMux()
	mux.Handle("GET /startupz", probe(ops, model.ProbeStartup, logging))
	mux.Handle("GET /livez", probe(ops, model.ProbeLiveness, logging))
	mux.Handle("GET /readyz", probe(ops, model.ProbeReady, logging))
	mux.Handle("GET /build", jsonHandler(model.OperationBuild, logging, func(context.Context) (any, error) { return ops.Build(), nil }))
	mux.Handle("GET /diagnostics", protect(access, model.OperationDiagnostics, jsonHandler(model.OperationDiagnostics, logging, func(ctx context.Context) (any, error) { return ops.Diagnostics(ctx) }), logging))
	switch mode {
	case configbinding.AccessDisabled:
	case configbinding.AccessPublic:
		mux.Handle("GET /metrics", metrics)
	case configbinding.AccessProtected:
		mux.Handle("GET /metrics", protect(access, model.OperationMetrics, metrics, logging))
	default:
		return nil, fmt.Errorf("management metrics access %q is unsupported", mode)
	}
	return noStore(mux), nil
}

func probe(ops *service.Service, kind model.ProbeKind, logging logger.Logger) http.Handler {
	return jsonHandler(string(kind), logging, func(ctx context.Context) (any, error) {
		result, passing, err := ops.Probe(ctx, kind)
		if err != nil {
			logManagementError(logging, "probe", string(kind), http.StatusServiceUnavailable, err)
			return statusValue{status: http.StatusServiceUnavailable, value: model.Probe{Status: "fail"}}, nil
		}
		status := http.StatusOK
		if !passing {
			status = http.StatusServiceUnavailable
			logManagementWarning(logging, "probe", string(kind), status, "management probe failed")
		}
		return statusValue{status: status, value: result}, nil
	})
}

type statusValue struct {
	status int
	value  any
}

func jsonHandler(operationName string, logging logger.Logger, operation func(context.Context) (any, error)) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		value, err := operation(request.Context())
		if err != nil {
			logManagementError(logging, "operation", operationName, http.StatusInternalServerError, err)
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

func protect(access Access, operation string, next http.Handler, logging logger.Logger) http.Handler {
	authorized := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := access.Authorize(request.Context(), operation); err != nil {
			logManagementWarning(logging, "authorize", operation, http.StatusForbidden, "management operation rejected")
			httpx.WriteProblem(writer, request, &httpx.StatusError{StatusCode: http.StatusForbidden, Code: "forbidden", Message: "management scope is required", Err: err})
			return
		}
		next.ServeHTTP(writer, request)
	})
	return access.Authenticate(authorized)
}

func logManagementWarning(logging logger.Logger, phase string, operation string, status int, message string) {
	if logging == nil {
		return
	}
	logging.Warn(message,
		logger.String("owner", "management"),
		logger.String("phase", phase),
		logger.String("operation", operation),
		logger.Int("status", status),
		logger.String("status_class", statusClass(status)),
	)
}

func logManagementError(logging logger.Logger, phase string, operation string, status int, err error) {
	if logging == nil || err == nil {
		return
	}
	logging.Error("management operation failed",
		logger.String("owner", "management"),
		logger.String("phase", phase),
		logger.String("operation", operation),
		logger.Int("status", status),
		logger.String("status_class", statusClass(status)),
		logger.String("error_type", managementErrorType(err)),
		logger.String("cause_type", fmt.Sprintf("%T", err)),
	)
}

func managementErrorType(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%T", err)
}

func statusClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	case status >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(writer, request)
	})
}
