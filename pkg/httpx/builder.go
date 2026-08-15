package httpx

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
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
		Addr:                defaults.Addr,
		ReadHeaderTimeout:   defaults.ReadHeaderTimeout,
		ReadTimeout:         defaults.ReadTimeout,
		WriteTimeout:        defaults.WriteTimeout,
		IdleTimeout:         defaults.IdleTimeout,
		MaxHeaderBytes:      defaults.MaxHeaderBytes,
		RequestTimeout:      defaults.RequestTimeout,
		MaxRequestBodyBytes: defaults.MaxRequestBodyBytes,
		MaxInFlight:         defaults.MaxInFlight,
		TrustedProxyCIDRs:   append([]string(nil), defaults.TrustedProxyCIDRs...),
		RateLimit:           defaults.RateLimit,
		CORS:                cloneCORSConfig(defaults.CORS),
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
	if cfg.RequestTimeout < 0 || cfg.MaxRequestBodyBytes < 0 || cfg.MaxInFlight < 0 ||
		cfg.RateLimit.RequestsPerSecond < 0 || cfg.RateLimit.Burst < 0 {
		return resolvedServerConfig{}, fmt.Errorf("server request budgets must be non-negative")
	}
	if (cfg.RateLimit.RequestsPerSecond == 0) != (cfg.RateLimit.Burst == 0) {
		return resolvedServerConfig{}, fmt.Errorf("server rate limit and burst must both be zero or positive")
	}
	if err := validateTrustedProxyCIDRs(cfg.TrustedProxyCIDRs); err != nil {
		return resolvedServerConfig{}, err
	}
	if err := validateCORSConfig(cfg.CORS); err != nil {
		return resolvedServerConfig{}, err
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
	if cfg.RequestTimeout > 0 {
		resolved.RequestTimeout = cfg.RequestTimeout
	}
	if cfg.MaxRequestBodyBytes > 0 {
		resolved.MaxRequestBodyBytes = cfg.MaxRequestBodyBytes
	}
	if cfg.MaxInFlight > 0 {
		resolved.MaxInFlight = cfg.MaxInFlight
	}
	resolved.TrustedProxyCIDRs = append([]string(nil), cfg.TrustedProxyCIDRs...)
	if cfg.RateLimit.RequestsPerSecond > 0 {
		resolved.RateLimit = cfg.RateLimit
	}
	if len(cfg.CORS.AllowedOrigins) > 0 || len(cfg.CORS.AllowedMethods) > 0 || len(cfg.CORS.AllowedHeaders) > 0 {
		resolved.CORS = cloneCORSConfig(cfg.CORS)
	}
	resolved.ErrorLog = cfg.ErrorLog

	return resolved, nil
}

func validateTrustedProxyCIDRs(values []string) error {
	for _, value := range values {
		if value == "" {
			return fmt.Errorf("trusted proxy CIDR is empty")
		}
		if _, _, err := net.ParseCIDR(value); err != nil {
			return fmt.Errorf("parse trusted proxy CIDR %q: %w", value, err)
		}
	}
	return nil
}

func validateCORSConfig(cfg CORSConfig) error {
	seen := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, origin := range cfg.AllowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" || parsed.Path != "" {
			return fmt.Errorf("CORS origin %q must be an absolute HTTP origin", origin)
		}
		if _, exists := seen[origin]; exists {
			return fmt.Errorf("CORS origin %q is duplicated", origin)
		}
		seen[origin] = struct{}{}
	}
	for _, method := range cfg.AllowedMethods {
		if method == "" || method != strings.ToUpper(method) {
			return fmt.Errorf("CORS method %q must be uppercase", method)
		}
	}
	for _, header := range cfg.AllowedHeaders {
		if strings.TrimSpace(header) == "" {
			return fmt.Errorf("CORS allowed header is empty")
		}
	}
	return nil
}

func cloneCORSConfig(cfg CORSConfig) CORSConfig {
	return CORSConfig{
		AllowedOrigins: append([]string(nil), cfg.AllowedOrigins...),
		AllowedMethods: append([]string(nil), cfg.AllowedMethods...),
		AllowedHeaders: append([]string(nil), cfg.AllowedHeaders...),
	}
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
