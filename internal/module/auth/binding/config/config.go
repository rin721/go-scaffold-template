// Package configbinding 绑定 Auth module 的配置、默认值和跨节安全校验。
package configbinding

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
)

const (
	capabilityID                  = "module.auth"
	configPath                    = "auth"
	ModeDevelopmentAnonymous Mode = "development-anonymous"
	ModeJWT                  Mode = "jwt"
)

// Mode 是 Auth module 的有限运行 profile。
type Mode string

// JWT 保存 production verifier 的受控配置。
type JWT struct {
	Issuer               string        `mapstructure:"issuer"`
	Audience             string        `mapstructure:"audience"`
	JWKSURL              string        `mapstructure:"jwksURL"`
	Algorithms           []string      `mapstructure:"algorithms"`
	ScopesClaim          string        `mapstructure:"scopesClaim"`
	RequestTimeout       time.Duration `mapstructure:"requestTimeout"`
	RefreshInterval      time.Duration `mapstructure:"refreshInterval"`
	RefreshTimeout       time.Duration `mapstructure:"refreshTimeout"`
	Leeway               time.Duration `mapstructure:"leeway"`
	MaxResponseBodyBytes int64         `mapstructure:"maxResponseBodyBytes"`
}

// Config 是 Auth module 唯一配置契约。
type Config struct {
	Mode             Mode     `mapstructure:"mode"`
	AnonymousSubject string   `mapstructure:"anonymousSubject"`
	AnonymousScopes  []string `mapstructure:"anonymousScopes"`
	JWT              JWT      `mapstructure:"jwt"`
}

// Default 返回可修改的 development-loopback 安全默认值。
func Default() Config {
	return Config{
		Mode:             ModeDevelopmentAnonymous,
		AnonymousSubject: "development-loopback",
		AnonymousScopes:  []string{"management:read", "todos:read", "todos:write"},
		JWT: JWT{
			Algorithms: []string{"RS256"}, ScopesClaim: "scope",
			RequestTimeout: 5 * time.Second, RefreshInterval: 15 * time.Minute,
			RefreshTimeout: 5 * time.Second, Leeway: 30 * time.Second,
			MaxResponseBodyBytes: 1 << 20,
		},
	}
}

// Binding 返回 Auth module 对默认配置和 candidate validation 的唯一声明。
func Binding() config.Binding {
	return config.Binding{
		CapabilityID: capabilityID, ConfigPath: configPath, Contract: defaults{},
		Validate: func(snapshot config.Snapshot) error {
			_, err := Decode(snapshot)
			return err
		},
	}
}

// Decode 从完整 Snapshot 解码并执行跨 logger/http section 的安全校验。
func Decode(snapshot config.Snapshot) (Config, error) {
	resolved := Default()
	if err := snapshot.DecodeSection(configPath, &resolved); err != nil {
		return Config{}, fmt.Errorf("decode auth configuration: %w", err)
	}
	environment, err := stringValue(snapshot, "logger.environment")
	if err != nil {
		return Config{}, err
	}
	httpAddress, err := stringValue(snapshot, "http.addr")
	if err != nil {
		return Config{}, err
	}
	if err := validate(resolved, environment, httpAddress); err != nil {
		return Config{}, err
	}
	return resolved, nil
}

func stringValue(snapshot config.Snapshot, path string) (string, error) {
	value, exists := snapshot.Value(path)
	text, ok := value.(string)
	if !exists || !ok || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("configuration path %s is unavailable", path)
	}
	return text, nil
}

func validate(resolved Config, environment, httpAddress string) error {
	switch resolved.Mode {
	case ModeDevelopmentAnonymous:
		if environment != "development" || !isLoopbackAddress(httpAddress) {
			return fmt.Errorf("development anonymous auth requires development environment and loopback HTTP")
		}
		if strings.TrimSpace(resolved.AnonymousSubject) == "" || len(resolved.AnonymousScopes) == 0 {
			return fmt.Errorf("development anonymous principal is incomplete")
		}
		for _, scope := range resolved.AnonymousScopes {
			if strings.TrimSpace(scope) == "" {
				return fmt.Errorf("development anonymous scope is empty")
			}
		}
	case ModeJWT:
		if err := validateJWT(resolved.JWT, environment == "development"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported auth mode %q", resolved.Mode)
	}
	if environment == "production" && resolved.Mode != ModeJWT {
		return fmt.Errorf("production requires JWT auth mode")
	}
	return nil
}

func validateJWT(value JWT, allowHTTP bool) error {
	if strings.TrimSpace(value.Issuer) == "" || strings.TrimSpace(value.Audience) == "" || strings.TrimSpace(value.JWKSURL) == "" {
		return fmt.Errorf("JWT issuer, audience and JWKS URL are required")
	}
	if len(value.Algorithms) == 0 || strings.TrimSpace(value.ScopesClaim) == "" {
		return fmt.Errorf("JWT algorithms and scopes claim are required")
	}
	for _, algorithm := range value.Algorithms {
		if strings.TrimSpace(algorithm) == "" || strings.EqualFold(algorithm, "none") {
			return fmt.Errorf("JWT algorithm %q is invalid", algorithm)
		}
	}
	if value.RequestTimeout <= 0 || value.RefreshInterval <= 0 || value.RefreshTimeout <= 0 || value.Leeway < 0 || value.MaxResponseBodyBytes <= 0 {
		return fmt.Errorf("JWT budgets are invalid")
	}
	parsed, err := url.Parse(value.JWKSURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("JWKS URL is invalid")
	}
	if parsed.Scheme != "https" && !(allowHTTP && parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return fmt.Errorf("JWKS URL must use HTTPS; development HTTP is limited to loopback")
	}
	return nil
}

func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	return err == nil && isLoopbackHost(host)
}

func isLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type defaults struct{}

func (defaults) Defaults(ctx context.Context) (config.Object, config.Control, error) {
	if ctx == nil {
		return nil, config.Continue, fmt.Errorf("auth defaults context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, config.Continue, err
	}
	value := Default()
	maxBody, err := config.Number(fmt.Sprintf("%d", value.JWT.MaxResponseBodyBytes))
	if err != nil {
		return nil, config.Continue, err
	}
	return config.Object{
		config.FieldOf("mode", config.String(string(value.Mode))),
		config.FieldOf("anonymousSubject", config.String(value.AnonymousSubject)),
		config.FieldOf("anonymousScopes", stringsList(value.AnonymousScopes)),
		config.FieldOf("jwt", config.ObjectValue(config.Object{
			config.FieldOf("issuer", config.String(value.JWT.Issuer)),
			config.FieldOf("audience", config.String(value.JWT.Audience)),
			config.FieldOf("jwksURL", config.String(value.JWT.JWKSURL)),
			config.FieldOf("algorithms", stringsList(value.JWT.Algorithms)),
			config.FieldOf("scopesClaim", config.String(value.JWT.ScopesClaim)),
			config.FieldOf("requestTimeout", config.Duration(value.JWT.RequestTimeout)),
			config.FieldOf("refreshInterval", config.Duration(value.JWT.RefreshInterval)),
			config.FieldOf("refreshTimeout", config.Duration(value.JWT.RefreshTimeout)),
			config.FieldOf("leeway", config.Duration(value.JWT.Leeway)),
			config.FieldOf("maxResponseBodyBytes", maxBody),
		})),
	}, config.Continue, nil
}

func stringsList(values []string) config.Value {
	items := make([]config.Value, len(values))
	for index, value := range values {
		items[index] = config.String(value)
	}
	return config.List(items...)
}

var _ config.DefaultContract = defaults{}
