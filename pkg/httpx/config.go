package httpx

import (
	"log"
	"net/http"
	"time"
)

// ClientConfig 定义 HTTP 客户端构造参数。
type ClientConfig struct {
	BaseURL              string
	Timeout              time.Duration
	MaxResponseBodyBytes int64
	Transport            http.RoundTripper
	RetryCount           int
	RetryWaitTime        time.Duration
	RetryMaxWaitTime     time.Duration
}

// RouterConfig 定义 Router 构造参数。
type RouterConfig struct {
	ErrorHandler ErrorHandler
}

// ServerConfig 定义 HTTP 服务端构造参数。
type ServerConfig struct {
	Addr                string
	ReadHeaderTimeout   time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	MaxHeaderBytes      int
	RequestTimeout      time.Duration
	MaxRequestBodyBytes int64
	MaxInFlight         int
	TrustedProxyCIDRs   []string
	RateLimit           RateLimitConfig
	CORS                CORSConfig
	ErrorLog            *log.Logger
}

// RateLimitConfig 定义单进程入口令牌桶；跨副本配额不在本契约范围。
type RateLimitConfig struct {
	RequestsPerSecond int
	Burst             int
}

// CORSConfig 定义显式跨域 allowlist；空 origin 列表表示拒绝跨域。
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
}

type resolvedClientConfig struct {
	BaseURL              string
	Timeout              time.Duration
	MaxResponseBodyBytes int64
	Transport            http.RoundTripper
	RetryCount           int
	RetryWaitTime        time.Duration
	RetryMaxWaitTime     time.Duration
}

type resolvedRouterConfig struct {
	ErrorHandler ErrorHandler
}

type resolvedServerConfig struct {
	Addr                string
	ReadHeaderTimeout   time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	MaxHeaderBytes      int
	RequestTimeout      time.Duration
	MaxRequestBodyBytes int64
	MaxInFlight         int
	TrustedProxyCIDRs   []string
	RateLimit           RateLimitConfig
	CORS                CORSConfig
	ErrorLog            *log.Logger
}
