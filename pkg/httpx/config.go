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
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
	ErrorLog          *log.Logger
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
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
	ErrorLog          *log.Logger
}
