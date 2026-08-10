package httpx

import (
	"fmt"
	"net/http"
)

func resolveClientConfig(cfg *ClientConfig) (resolvedClientConfig, error) {
	defaults := DefaultClientConfig()
	resolved := resolvedClientConfig{
		Timeout:              defaults.Timeout,
		MaxResponseBodyBytes: defaults.MaxResponseBodyBytes,
		RetryWaitTime:        defaults.RetryWaitTime,
		RetryMaxWaitTime:     defaults.RetryMaxWaitTime,
	}
	if cfg == nil {
		return resolved, nil
	}

	if cfg.Timeout < 0 {
		return resolvedClientConfig{}, fmt.Errorf("client timeout must be non-negative")
	}
	if cfg.MaxResponseBodyBytes < 0 {
		return resolvedClientConfig{}, fmt.Errorf("max response body bytes must be non-negative")
	}
	if cfg.RetryCount < 0 {
		return resolvedClientConfig{}, fmt.Errorf("retry count must be non-negative")
	}
	if cfg.RetryWaitTime < 0 || cfg.RetryMaxWaitTime < 0 {
		return resolvedClientConfig{}, fmt.Errorf("retry wait times must be non-negative")
	}
	resolved.BaseURL = cfg.BaseURL
	if cfg.Timeout > 0 {
		resolved.Timeout = cfg.Timeout
	}
	if cfg.MaxResponseBodyBytes > 0 {
		resolved.MaxResponseBodyBytes = cfg.MaxResponseBodyBytes
	}
	resolved.Transport = cfg.Transport
	resolved.RetryCount = cfg.RetryCount
	if cfg.RetryWaitTime > 0 {
		resolved.RetryWaitTime = cfg.RetryWaitTime
	}
	if cfg.RetryMaxWaitTime > 0 {
		resolved.RetryMaxWaitTime = cfg.RetryMaxWaitTime
	}

	return resolved, nil
}

func resolveRouterConfig(cfg *RouterConfig) resolvedRouterConfig {
	resolved := resolvedRouterConfig{ErrorHandler: DefaultErrorHandler}
	if cfg == nil {
		return resolved
	}
	if cfg.ErrorHandler != nil {
		resolved.ErrorHandler = cfg.ErrorHandler
	}
	return resolved
}

func resolveServerConfig(cfg *ServerConfig) (resolvedServerConfig, error) {
	defaults := DefaultServerConfig()
	resolved := resolvedServerConfig{
		Addr:              defaults.Addr,
		ReadHeaderTimeout: defaults.ReadHeaderTimeout,
		ReadTimeout:       defaults.ReadTimeout,
		WriteTimeout:      defaults.WriteTimeout,
		IdleTimeout:       defaults.IdleTimeout,
		MaxHeaderBytes:    defaults.MaxHeaderBytes,
	}
	if cfg == nil {
		return resolved, nil
	}

	if cfg.ReadHeaderTimeout < 0 || cfg.ReadTimeout < 0 || cfg.WriteTimeout < 0 || cfg.IdleTimeout < 0 {
		return resolvedServerConfig{}, fmt.Errorf("server timeouts must be non-negative")
	}
	if cfg.MaxHeaderBytes < 0 {
		return resolvedServerConfig{}, fmt.Errorf("max header bytes must be non-negative")
	}
	if cfg.Addr != "" {
		resolved.Addr = cfg.Addr
	}
	if cfg.ReadHeaderTimeout > 0 {
		resolved.ReadHeaderTimeout = cfg.ReadHeaderTimeout
	}
	if cfg.ReadTimeout > 0 {
		resolved.ReadTimeout = cfg.ReadTimeout
	}
	if cfg.WriteTimeout > 0 {
		resolved.WriteTimeout = cfg.WriteTimeout
	}
	if cfg.IdleTimeout > 0 {
		resolved.IdleTimeout = cfg.IdleTimeout
	}
	if cfg.MaxHeaderBytes > 0 {
		resolved.MaxHeaderBytes = cfg.MaxHeaderBytes
	}
	resolved.ErrorLog = cfg.ErrorLog

	return resolved, nil
}

func methodOrDefault(method Method) Method {
	if method == "" {
		return MethodGet
	}
	return method
}

func statusAccepted(statusCode int, acceptStatus []int) bool {
	if len(acceptStatus) == 0 {
		return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
	}
	for _, accepted := range acceptStatus {
		if statusCode == accepted {
			return true
		}
	}
	return false
}
