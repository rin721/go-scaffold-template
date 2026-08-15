// Package jwtadapter 通过 jwx 封装 JWT/JWKS 验证与刷新生命周期。
package jwtadapter

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/rin721/go-scaffold-template/internal/module/auth/model"
	"github.com/rin721/go-scaffold-template/internal/module/auth/service"
	"github.com/rin721/go-scaffold-template/pkg/clock"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
	"golang.org/x/sync/singleflight"
)

const (
	participantName = "module.auth.jwt-jwks"
	maxTokenBytes   = 16 << 10
	maxRedirects    = 3
)

// Config 是 JWT Adapter 实际使用的项目自有配置。
type Config struct {
	Issuer                 string
	Audience               string
	JWKSURL                string
	Algorithms             []string
	ScopesClaim            string
	RequestTimeout         time.Duration
	RefreshInterval        time.Duration
	RefreshTimeout         time.Duration
	Leeway                 time.Duration
	MaxResponseBodyBytes   int64
	AllowLoopbackOrPrivate bool
}

// Verifier 拥有 JWKS cache、后台刷新 goroutine 和受控 HTTP client。
type Verifier struct {
	config     Config
	clock      clock.Clock
	algorithms map[string]struct{}

	mu     sync.RWMutex
	cache  *jwk.Cache
	cancel context.CancelFunc
	ready  bool

	refresh singleflight.Group
}

// New 只校验并冻结配置，不执行网络 I/O，也不启动 goroutine。
func New(config Config, currentClock clock.Clock) (*Verifier, error) {
	if currentClock == nil {
		return nil, fmt.Errorf("JWT clock is nil")
	}
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	algorithms := make(map[string]struct{}, len(config.Algorithms))
	for _, name := range config.Algorithms {
		if _, exists := algorithms[name]; exists {
			return nil, fmt.Errorf("JWT algorithm %q is duplicated", name)
		}
		if _, ok := jwa.LookupSignatureAlgorithm(name); !ok {
			return nil, fmt.Errorf("JWT algorithm %q is unsupported", name)
		}
		algorithms[name] = struct{}{}
	}
	config.Algorithms = append([]string(nil), config.Algorithms...)
	return &Verifier{config: config, clock: currentClock, algorithms: algorithms}, nil
}

func validateConfig(config Config) error {
	if strings.TrimSpace(config.Issuer) == "" || strings.TrimSpace(config.Audience) == "" || strings.TrimSpace(config.JWKSURL) == "" {
		return fmt.Errorf("JWT issuer, audience and JWKS URL are required")
	}
	if len(config.Algorithms) == 0 || strings.TrimSpace(config.ScopesClaim) == "" {
		return fmt.Errorf("JWT algorithms and scopes claim are required")
	}
	if config.RequestTimeout <= 0 || config.RefreshInterval <= 0 || config.RefreshTimeout <= 0 || config.Leeway < 0 || config.MaxResponseBodyBytes <= 0 {
		return fmt.Errorf("JWT budgets are invalid")
	}
	parsed, err := url.Parse(config.JWKSURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("JWKS URL is invalid")
	}
	return nil
}

// Name 返回由 Supervisor 诊断和排序使用的稳定所有者名称。
func (*Verifier) Name() string { return participantName }

// Start 创建并等待首次 JWKS 获取成功；失败时不会留下后台资源。
func (v *Verifier) Start(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("start JWT verifier: context is nil")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cache != nil || v.cancel != nil {
		return fmt.Errorf("start JWT verifier: already started")
	}
	lifetime, cancel := context.WithCancel(context.Background())
	stopOnParent := context.AfterFunc(ctx, cancel)
	client := httprc.NewClient(
		httprc.WithWorkers(1),
		httprc.WithWhitelist(httprc.NewMapWhitelist().Add(v.config.JWKSURL)),
	)
	cache, err := jwk.NewCache(lifetime, client)
	if err != nil {
		stopOnParent()
		cancel()
		return fmt.Errorf("start JWT JWKS cache: %w", err)
	}
	fetchCtx, fetchCancel := context.WithTimeout(ctx, v.config.RefreshTimeout)
	err = cache.Register(fetchCtx, v.config.JWKSURL,
		jwk.WithHTTPClient(v.httpClient()),
		jwk.WithMaxFetchBodySize(v.config.MaxResponseBodyBytes),
		jwk.WithRejectDuplicateKID(true),
		jwk.WithConstantInterval(v.config.RefreshInterval),
	)
	fetchCancel()
	if err != nil {
		stopOnParent()
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), v.config.RefreshTimeout)
		shutdownErr := cache.Shutdown(shutdownCtx)
		shutdownCancel()
		return fmt.Errorf("register JWT JWKS resource: %w", errors.Join(err, shutdownErr))
	}
	v.cache = cache
	v.cancel = func() {
		stopOnParent()
		cancel()
	}
	v.ready = true
	return nil
}

// Stop 停止定时刷新并等待 jwx 释放其 goroutine；重复停止是安全的。
func (v *Verifier) Stop(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("stop JWT verifier: context is nil")
	}
	v.mu.Lock()
	cache := v.cache
	cancel := v.cancel
	v.cache = nil
	v.cancel = nil
	v.ready = false
	v.mu.Unlock()
	if cache == nil {
		return nil
	}
	if cancel != nil {
		cancel()
	}
	if err := cache.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown JWT JWKS cache: %w", err)
	}
	return nil
}

// Ready 表示首次 JWKS 已获取成功且当前生命周期仍在运行。
func (v *Verifier) Ready() bool {
	if v == nil {
		return false
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.ready && v.cache != nil
}

// Verify 验证签名与必要 claims，并只返回项目自有 Principal。
func (v *Verifier) Verify(ctx context.Context, credential model.Credential) (model.Principal, error) {
	if ctx == nil || credential.Scheme != "Bearer" || credential.Value == "" || len(credential.Value) > maxTokenBytes {
		return model.Principal{}, model.ErrUnauthenticated
	}
	if err := ctx.Err(); err != nil {
		return model.Principal{}, err
	}
	kid, algorithm, err := v.protectedHeader([]byte(credential.Value))
	if err != nil {
		return model.Principal{}, model.ErrUnauthenticated
	}
	set, err := v.keySet(ctx)
	if err != nil {
		return model.Principal{}, model.ErrUnauthenticated
	}
	key, exists := set.LookupKeyID(kid)
	if !exists {
		set, err = v.refreshUnknownKey(ctx, kid)
		if err != nil {
			return model.Principal{}, model.ErrUnauthenticated
		}
		key, exists = set.LookupKeyID(kid)
	}
	if !exists || !keyMatchesAlgorithm(key, algorithm) {
		return model.Principal{}, model.ErrUnauthenticated
	}
	token, err := jwt.Parse([]byte(credential.Value),
		jwt.WithKey(algorithm, key),
		jwt.WithIssuer(v.config.Issuer),
		jwt.WithAudience(v.config.Audience),
		jwt.WithRequiredClaim(jwt.SubjectKey),
		jwt.WithRequiredClaim(jwt.ExpirationKey),
		jwt.WithRequiredClaim(jwt.NotBeforeKey),
		jwt.WithRequiredClaim(jwt.IssuedAtKey),
		jwt.WithClock(jwt.ClockFunc(v.clock.Now)),
		jwt.WithAcceptableSkew(v.config.Leeway),
	)
	if err != nil {
		return model.Principal{}, model.ErrUnauthenticated
	}
	subject, subjectExists := token.Subject()
	issuedAt, issuedAtExists := token.IssuedAt()
	if !subjectExists || !issuedAtExists {
		return model.Principal{}, model.ErrUnauthenticated
	}
	scopes, err := extractScopes(token, v.config.ScopesClaim)
	if err != nil {
		return model.Principal{}, model.ErrUnauthenticated
	}
	return model.NewPrincipal(subject, model.ActorService, scopes, v.clock.Now(), issuedAt)
}

func (v *Verifier) protectedHeader(encoded []byte) (string, jwa.SignatureAlgorithm, error) {
	message, err := jws.Parse(encoded)
	if err != nil || len(message.Signatures()) != 1 {
		return "", jwa.SignatureAlgorithm{}, model.ErrUnauthenticated
	}
	headers := message.Signatures()[0].ProtectedHeaders()
	if headers == nil {
		return "", jwa.SignatureAlgorithm{}, model.ErrUnauthenticated
	}
	kid, hasKid := headers.KeyID()
	algorithm, hasAlgorithm := headers.Algorithm()
	if !hasKid || kid == "" || !hasAlgorithm {
		return "", jwa.SignatureAlgorithm{}, model.ErrUnauthenticated
	}
	if _, allowed := v.algorithms[algorithm.String()]; !allowed {
		return "", jwa.SignatureAlgorithm{}, model.ErrUnauthenticated
	}
	return kid, algorithm, nil
}

func (v *Verifier) keySet(ctx context.Context) (jwk.Set, error) {
	v.mu.RLock()
	cache := v.cache
	ready := v.ready
	v.mu.RUnlock()
	if !ready || cache == nil {
		return nil, model.ErrUnauthenticated
	}
	return cache.Lookup(ctx, v.config.JWKSURL)
}

func (v *Verifier) refreshUnknownKey(ctx context.Context, kid string) (jwk.Set, error) {
	value, err, _ := v.refresh.Do(kid, func() (any, error) {
		v.mu.RLock()
		cache := v.cache
		ready := v.ready
		v.mu.RUnlock()
		if !ready || cache == nil {
			return nil, model.ErrUnauthenticated
		}
		refreshCtx, cancel := context.WithTimeout(ctx, v.config.RefreshTimeout)
		defer cancel()
		return cache.Refresh(refreshCtx, v.config.JWKSURL)
	})
	if err != nil {
		return nil, err
	}
	set, ok := value.(jwk.Set)
	if !ok || set == nil {
		return nil, model.ErrUnauthenticated
	}
	return set, nil
}

func keyMatchesAlgorithm(key jwk.Key, expected jwa.SignatureAlgorithm) bool {
	algorithm, exists := key.Algorithm()
	return exists && algorithm.String() == expected.String()
}

func extractScopes(token jwt.Token, claim string) ([]model.Scope, error) {
	var raw any
	if err := token.Get(claim, &raw); err != nil {
		return nil, err
	}
	var values []string
	switch typed := raw.(type) {
	case string:
		values = strings.Fields(typed)
	case []string:
		values = typed
	case []any:
		values = make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, model.ErrUnauthenticated
			}
			values = append(values, text)
		}
	default:
		return nil, model.ErrUnauthenticated
	}
	if len(values) == 0 {
		return nil, model.ErrUnauthenticated
	}
	scopes := make([]model.Scope, len(values))
	for index, value := range values {
		if strings.TrimSpace(value) == "" {
			return nil, model.ErrUnauthenticated
		}
		scopes[index] = model.Scope(value)
	}
	return scopes, nil
}

func (v *Verifier) httpClient() *http.Client {
	target, _ := url.Parse(v.config.JWKSURL)
	dialer := &net.Dialer{Timeout: v.config.RequestTimeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           v.guardedDialContext(dialer),
		ForceAttemptHTTP2:     true,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   v.config.RequestTimeout,
		ResponseHeaderTimeout: v.config.RequestTimeout,
		IdleConnTimeout:       90 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   v.config.RequestTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects || request.URL.Scheme != target.Scheme || !strings.EqualFold(request.URL.Host, target.Host) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

func (v *Verifier) guardedDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("resolve JWKS endpoint: %w", err)
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve JWKS endpoint: %w", err)
		}
		for _, candidate := range addresses {
			if !v.config.AllowLoopbackOrPrivate && nonPublic(candidate.IP) {
				continue
			}
			connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
			if dialErr == nil {
				return connection, nil
			}
			err = errors.Join(err, dialErr)
		}
		if err == nil {
			err = fmt.Errorf("JWKS endpoint resolved only to blocked addresses")
		}
		return nil, err
	}
}

func nonPublic(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

var (
	_ service.CredentialVerifier = (*Verifier)(nil)
	_ supervisor.Participant     = (*Verifier)(nil)
)
