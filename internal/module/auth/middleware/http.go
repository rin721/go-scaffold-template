// Package middleware 实现 Auth module 拥有的 HTTP bearer 认证边界。
package middleware

import (
	"net/http"
	"strings"

	"github.com/rin721/go-scaffold-template/internal/module/auth/model"
	"github.com/rin721/go-scaffold-template/internal/module/auth/service"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
)

// HTTP 构造只负责认证与 Principal 注入的 middleware；operation/object 授权留给后续边界。
func HTTP(authenticator service.Authenticator) (func(http.Handler) http.Handler, error) {
	if authenticator == nil {
		return nil, model.ErrUnauthenticated
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			principal, err := authenticateRequest(request, authenticator)
			if err != nil {
				_ = authenticator.RecordAuthenticationFailure(request.Context())
				writer.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
				httpx.WriteProblem(writer, request, &httpx.StatusError{
					StatusCode: http.StatusUnauthorized, Code: "unauthenticated", Message: "valid bearer authentication is required", Err: model.ErrUnauthenticated,
				})
				return
			}
			request = request.WithContext(model.WithPrincipal(request.Context(), principal))
			next.ServeHTTP(writer, request)
		})
	}, nil
}

func authenticateRequest(request *http.Request, authenticator service.Authenticator) (model.Principal, error) {
	values := request.Header.Values("Authorization")
	if len(values) == 0 {
		return authenticator.DevelopmentPrincipal(request.Context())
	}
	if len(values) != 1 {
		return model.Principal{}, model.ErrUnauthenticated
	}
	scheme, value, ok := strings.Cut(values[0], " ")
	if !ok || scheme != "Bearer" || value == "" || strings.ContainsAny(value, " \t\r\n") {
		return model.Principal{}, model.ErrUnauthenticated
	}
	return authenticator.Authenticate(request.Context(), model.Credential{Scheme: scheme, Value: value})
}
