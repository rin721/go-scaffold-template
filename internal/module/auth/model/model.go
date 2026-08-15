// Package model 定义认证授权模块的稳定项目类型。
package model

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	// ErrUnauthenticated 表示请求没有可用且已验证的身份。
	ErrUnauthenticated = errors.New("principal is unauthenticated")
	// ErrPermissionDenied 表示主体不满足当前 operation/action policy。
	ErrPermissionDenied = errors.New("principal is not authorized")
)

// ActorKind 描述主体的信任来源。
type ActorKind string

const (
	ActorService     ActorKind = "service"
	ActorCLI         ActorKind = "cli"
	ActorDevelopment ActorKind = "development"
)

// Scope 是认证主体携带的精确权限范围。
type Scope string

// Principal 是不暴露第三方 claims 的已验证主体。
type Principal struct {
	Subject         string
	Kind            ActorKind
	Scopes          []Scope
	AuthenticatedAt time.Time
	IssuedAt        time.Time
}

// NewPrincipal 构造规范化且不可包含空 scope 的 Principal。
func NewPrincipal(subject string, kind ActorKind, scopes []Scope, authenticatedAt, issuedAt time.Time) (Principal, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" || kind == "" || authenticatedAt.IsZero() || issuedAt.IsZero() {
		return Principal{}, ErrUnauthenticated
	}
	seen := make(map[Scope]struct{}, len(scopes))
	normalized := make([]Scope, 0, len(scopes))
	for _, scope := range scopes {
		scope = Scope(strings.TrimSpace(string(scope)))
		if scope == "" {
			return Principal{}, fmt.Errorf("principal scope is empty")
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		normalized = append(normalized, scope)
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i] < normalized[j] })
	return Principal{
		Subject: subject, Kind: kind, Scopes: normalized,
		AuthenticatedAt: authenticatedAt.UTC(), IssuedAt: issuedAt.UTC(),
	}, nil
}

// HasScope 判断 Principal 是否拥有精确 scope；不支持隐式通配符。
func (p Principal) HasScope(scope Scope) bool {
	for _, candidate := range p.Scopes {
		if candidate == scope {
			return true
		}
	}
	return false
}

// Credential 是 transport 提取后交给 verifier 的不透明凭据。
type Credential struct {
	Scheme string
	Value  string
}

// PolicyMode 区分公开与受保护 operation。
type PolicyMode string

const (
	PolicyPublic    PolicyMode = "public"
	PolicyProtected PolicyMode = "protected"
)

// Action 是业务用例拥有的稳定授权动作。
type Action string

// Policy 是由 OpenAPI inventory 传入 Auth module 的 operation policy。
type Policy struct {
	Operation string
	Mode      PolicyMode
	Scope     Scope
	Action    Action
}

// ResourceFacts 是授权所需的最小真实资源事实。
type ResourceFacts struct {
	Type         string
	ID           string
	OwnerSubject string
}

// DecisionReason 是可审计但不泄漏对象内容的原因类。
type DecisionReason string

const (
	ReasonAllowed         DecisionReason = "allowed"
	ReasonPublic          DecisionReason = "public"
	ReasonUnauthenticated DecisionReason = "unauthenticated"
	ReasonMissingPolicy   DecisionReason = "missing_policy"
	ReasonMissingScope    DecisionReason = "missing_scope"
	ReasonOwnerMismatch   DecisionReason = "owner_mismatch"
)

// Decision 是显式 fail-closed 的授权结果。
type Decision struct {
	Allowed bool
	Reason  DecisionReason
}

// AuditOutcome 是安全审计的低基数结果类。
type AuditOutcome string

const (
	AuditSucceeded AuditOutcome = "succeeded"
	AuditDenied    AuditOutcome = "denied"
	AuditFailed    AuditOutcome = "failed"
)

// AuditEvent 只携带安全判断需要的项目类型；Sink 负责标识符脱敏。
type AuditEvent struct {
	Operation string
	Action    Action
	Principal Principal
	Resource  ResourceFacts
	Decision  Decision
	Outcome   AuditOutcome
}

type principalContextKey struct{}

// WithPrincipal 把已验证 Principal 写入单次 transport context。
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFromContext 读取 transport 边界写入的 Principal。
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok && principal.Subject != "" && principal.Kind != "" && !principal.AuthenticatedAt.IsZero()
}
