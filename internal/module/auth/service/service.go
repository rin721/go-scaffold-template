// Package service 实现认证、operation policy 与审计用例。
package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/rin721/go-scaffold-template/internal/module/auth/model"
	"github.com/rin721/go-scaffold-template/pkg/clock"
)

// CredentialVerifier 是 Auth Service 使用方定义的第三方凭据验证 port。
type CredentialVerifier interface {
	Verify(context.Context, model.Credential) (model.Principal, error)
	Ready() bool
}

// AuditSink 是 Auth Service 使用方定义的低敏审计 port。
type AuditSink interface {
	Record(context.Context, model.AuditEvent) error
}

// Authenticator 是 HTTP middleware 使用的最小入口。
type Authenticator interface {
	Authenticate(context.Context, model.Credential) (model.Principal, error)
	DevelopmentPrincipal(context.Context) (model.Principal, error)
	RecordAuthenticationFailure(context.Context) error
}

// Service 是 Auth module 对 transport 与跨模块 Adapter 暴露的完成品。
type Service struct {
	clock       clock.Clock
	verifier    CredentialVerifier
	development *model.Principal
	audit       AuditSink
	byOperation map[string]model.Policy
	byAction    map[model.Action]model.Policy
}

// New 构造不执行 I/O 的 Auth Service，并冻结 policy authority。
func New(currentClock clock.Clock, verifier CredentialVerifier, development *model.Principal, audit AuditSink, policies []model.Policy) (*Service, error) {
	return newService(currentClock, verifier, development, false, audit, policies)
}

// NewLocal 构造只接受显式 CLI operator 的 Auth Service，不启用 HTTP 认证入口。
func NewLocal(currentClock clock.Clock, audit AuditSink, policies []model.Policy) (*Service, error) {
	return newService(currentClock, nil, nil, true, audit, policies)
}

func newService(currentClock clock.Clock, verifier CredentialVerifier, development *model.Principal, localOnly bool, audit AuditSink, policies []model.Policy) (*Service, error) {
	if currentClock == nil || audit == nil {
		return nil, fmt.Errorf("auth service dependencies are incomplete")
	}
	if verifier == nil && development == nil && !localOnly {
		return nil, fmt.Errorf("auth service has no authentication profile")
	}
	if localOnly && (verifier != nil || development != nil) || verifier != nil && development != nil {
		return nil, fmt.Errorf("auth service authentication profiles conflict")
	}
	byOperation := make(map[string]model.Policy, len(policies))
	byAction := make(map[model.Action]model.Policy, len(policies))
	for _, policy := range policies {
		if err := validatePolicy(policy); err != nil {
			return nil, err
		}
		if _, exists := byOperation[policy.Operation]; exists {
			return nil, fmt.Errorf("auth operation policy %q is duplicated", policy.Operation)
		}
		byOperation[policy.Operation] = policy
		if policy.Action != "" {
			if _, exists := byAction[policy.Action]; exists {
				return nil, fmt.Errorf("auth action policy %q is duplicated", policy.Action)
			}
			byAction[policy.Action] = policy
		}
	}
	if len(byOperation) == 0 {
		return nil, fmt.Errorf("auth policy inventory is empty")
	}
	return &Service{
		clock: currentClock, verifier: verifier, development: development, audit: audit,
		byOperation: byOperation, byAction: byAction,
	}, nil
}

func validatePolicy(policy model.Policy) error {
	if strings.TrimSpace(policy.Operation) == "" {
		return fmt.Errorf("auth operation policy has empty operation")
	}
	switch policy.Mode {
	case model.PolicyPublic:
		if policy.Scope != "" || policy.Action != "" {
			return fmt.Errorf("public auth policy %q declares scope or action", policy.Operation)
		}
	case model.PolicyProtected:
		if policy.Scope == "" || policy.Action == "" {
			return fmt.Errorf("protected auth policy %q is incomplete", policy.Operation)
		}
	default:
		return fmt.Errorf("auth policy %q has unsupported mode %q", policy.Operation, policy.Mode)
	}
	return nil
}

// Authenticate 验证 transport 提供的凭据；Verifier 未 Ready 时 fail closed。
func (s *Service) Authenticate(ctx context.Context, credential model.Credential) (model.Principal, error) {
	if ctx == nil || s == nil || s.verifier == nil || !s.verifier.Ready() {
		return model.Principal{}, model.ErrUnauthenticated
	}
	if err := ctx.Err(); err != nil {
		return model.Principal{}, err
	}
	principal, err := s.verifier.Verify(ctx, credential)
	if err != nil {
		return model.Principal{}, model.ErrUnauthenticated
	}
	return principal, nil
}

// DevelopmentPrincipal 只在已由 config gate 构造的开发 profile 返回主体。
func (s *Service) DevelopmentPrincipal(ctx context.Context) (model.Principal, error) {
	if ctx == nil || s == nil || s.development == nil {
		return model.Principal{}, model.ErrUnauthenticated
	}
	if err := ctx.Err(); err != nil {
		return model.Principal{}, err
	}
	return *s.development, nil
}

// LocalPrincipal 为显式 CLI operator 构造主体，不解析 bearer token。
func (s *Service) LocalPrincipal(ctx context.Context, subject string, scopes []model.Scope) (model.Principal, error) {
	if ctx == nil || s == nil {
		return model.Principal{}, model.ErrUnauthenticated
	}
	if err := ctx.Err(); err != nil {
		return model.Principal{}, err
	}
	now := s.clock.Now()
	return model.NewPrincipal(subject, model.ActorCLI, scopes, now, now)
}

// AuthorizeOperation 按 OpenAPI operation authority 授权。
func (s *Service) AuthorizeOperation(ctx context.Context, principal model.Principal, operation string) (model.Decision, error) {
	policy, exists := s.byOperation[operation]
	if !exists {
		return model.Decision{Reason: model.ReasonMissingPolicy}, nil
	}
	return decide(ctx, principal, policy, model.ResourceFacts{})
}

// EnforceOperation 执行 operation policy 并在返回前完成低敏审计。
func (s *Service) EnforceOperation(ctx context.Context, principal model.Principal, operation string) error {
	decision, err := s.AuthorizeOperation(ctx, principal, operation)
	if err != nil {
		return err
	}
	outcome := model.AuditDenied
	if decision.Allowed {
		outcome = model.AuditSucceeded
	}
	if err := s.Record(ctx, model.AuditEvent{
		Operation: operation, Principal: principal, Decision: decision, Outcome: outcome,
	}); err != nil {
		return fmt.Errorf("record operation authorization audit: %w", err)
	}
	if !decision.Allowed {
		return model.ErrPermissionDenied
	}
	return nil
}

// AuthorizeAction 按业务 action 与真实资源事实授权。
func (s *Service) AuthorizeAction(ctx context.Context, principal model.Principal, action model.Action, resource model.ResourceFacts) (model.Decision, error) {
	policy, exists := s.byAction[action]
	if !exists {
		return model.Decision{Reason: model.ReasonMissingPolicy}, nil
	}
	return decide(ctx, principal, policy, resource)
}

// EnforceAction 执行业务对象 policy，并把资源标识交给低敏 Sink 脱敏后记录。
func (s *Service) EnforceAction(ctx context.Context, principal model.Principal, action model.Action, resource model.ResourceFacts) error {
	decision, err := s.AuthorizeAction(ctx, principal, action, resource)
	if err != nil {
		return err
	}
	outcome := model.AuditDenied
	if decision.Allowed {
		outcome = model.AuditSucceeded
	}
	if err := s.Record(ctx, model.AuditEvent{
		Action: action, Principal: principal, Resource: resource, Decision: decision, Outcome: outcome,
	}); err != nil {
		return fmt.Errorf("record action authorization audit: %w", err)
	}
	if !decision.Allowed {
		return model.ErrPermissionDenied
	}
	return nil
}

func decide(ctx context.Context, principal model.Principal, policy model.Policy, resource model.ResourceFacts) (model.Decision, error) {
	if ctx == nil {
		return model.Decision{}, fmt.Errorf("authorization context is nil")
	}
	if err := ctx.Err(); err != nil {
		return model.Decision{}, err
	}
	if policy.Mode == model.PolicyPublic {
		return model.Decision{Allowed: true, Reason: model.ReasonPublic}, nil
	}
	if principal.Subject == "" {
		return model.Decision{}, model.ErrUnauthenticated
	}
	if !principal.HasScope(policy.Scope) {
		return model.Decision{Reason: model.ReasonMissingScope}, nil
	}
	if resource.OwnerSubject != "" && resource.OwnerSubject != principal.Subject {
		return model.Decision{Reason: model.ReasonOwnerMismatch}, nil
	}
	return model.Decision{Allowed: true, Reason: model.ReasonAllowed}, nil
}

// Record 交给 module-owned Audit Adapter 记录低敏事件。
func (s *Service) Record(ctx context.Context, event model.AuditEvent) error {
	if s == nil {
		return fmt.Errorf("auth service is nil")
	}
	return s.audit.Record(ctx, event)
}

// RecordAuthenticationFailure 记录不含 token、claims 或 raw path 的认证拒绝。
func (s *Service) RecordAuthenticationFailure(ctx context.Context) error {
	return s.Record(ctx, model.AuditEvent{
		Operation: "http.authenticate", Decision: model.Decision{Reason: model.ReasonUnauthenticated}, Outcome: model.AuditDenied,
	})
}

// Ready 表示当前 HTTP verifier 是否可用；development profile 始终 Ready。
func (s *Service) Ready() bool {
	return s != nil && (s.development != nil || s.verifier != nil && s.verifier.Ready())
}

var _ Authenticator = (*Service)(nil)
