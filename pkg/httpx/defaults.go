package httpx

import (
	"net/http"
	"time"
)

const (
	// DefaultClientTimeout 是客户端默认请求总超时。
	DefaultClientTimeout = 10 * time.Second
	// DefaultMaxResponseBodyBytes 是客户端默认最大响应体读取大小。
	DefaultMaxResponseBodyBytes int64 = 10 << 20
	// DefaultRetryWaitTime 是启用重试时的默认初始等待时间。
	DefaultRetryWaitTime = 100 * time.Millisecond
	// DefaultRetryMaxWaitTime 是启用重试时的默认最大等待时间。
	DefaultRetryMaxWaitTime = 2 * time.Second
)

const (
	// DefaultServerReadHeaderTimeout 是服务端默认请求头读取超时。
	DefaultServerReadHeaderTimeout = 5 * time.Second
	// DefaultServerReadTimeout 是服务端默认请求读取超时。
	DefaultServerReadTimeout = 15 * time.Second
	// DefaultServerWriteTimeout 是服务端默认响应写入超时。
	DefaultServerWriteTimeout = 30 * time.Second
	// DefaultServerIdleTimeout 是服务端默认空闲连接超时。
	DefaultServerIdleTimeout = 60 * time.Second
	// DefaultServerRequestTimeout 是单个业务请求的 application budget。
	DefaultServerRequestTimeout = 10 * time.Second
	// DefaultMaxRequestBodyBytes 是默认最大请求体。
	DefaultMaxRequestBodyBytes int64 = 1 << 20
	// DefaultServerMaxInFlight 是单进程同时处理的请求上限。
	DefaultServerMaxInFlight = 128
	// DefaultServerRequestsPerSecond 是单进程默认补充速率。
	DefaultServerRequestsPerSecond = 100
	// DefaultServerRateLimitBurst 是单进程默认突发容量。
	DefaultServerRateLimitBurst = 200
)

// DefaultClientConfig 返回可修改的客户端默认配置。
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Timeout:              DefaultClientTimeout,
		MaxResponseBodyBytes: DefaultMaxResponseBodyBytes,
		RetryWaitTime:        DefaultRetryWaitTime,
		RetryMaxWaitTime:     DefaultRetryMaxWaitTime,
	}
}

// DefaultRouterConfig 返回可修改的路由默认配置。
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		ErrorHandler: DefaultErrorHandler,
	}
}

// DefaultServerConfig 返回可修改的服务端默认配置。
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr:                defaultServerAddr,
		ReadHeaderTimeout:   DefaultServerReadHeaderTimeout,
		ReadTimeout:         DefaultServerReadTimeout,
		WriteTimeout:        DefaultServerWriteTimeout,
		IdleTimeout:         DefaultServerIdleTimeout,
		MaxHeaderBytes:      http.DefaultMaxHeaderBytes,
		RequestTimeout:      DefaultServerRequestTimeout,
		MaxRequestBodyBytes: DefaultMaxRequestBodyBytes,
		MaxInFlight:         DefaultServerMaxInFlight,
		RateLimit: RateLimitConfig{
			RequestsPerSecond: DefaultServerRequestsPerSecond,
			Burst:             DefaultServerRateLimitBurst,
		},
		CORS: CORSConfig{
			AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodOptions},
			AllowedHeaders: []string{"Authorization", "Content-Type", headerRequestID},
		},
	}
}
