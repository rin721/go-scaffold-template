// Package auditadapter 把 Auth module 审计事件写入低敏结构化日志。
package auditadapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/module/auth/model"
	"github.com/rin721/go-scaffold-template/internal/module/auth/service"
	"github.com/rin721/go-scaffold-template/pkg/logger"
)

// Sink 只记录分类、action 与不可逆标识摘要。
type Sink struct{ logger logger.Logger }

// New 创建无 I/O 副作用的审计 Sink。
func New(logging logger.Logger) (*Sink, error) {
	if logging == nil {
		return nil, fmt.Errorf("auth audit logger is nil")
	}
	return &Sink{logger: logging}, nil
}

// Record 写入不包含 token、claims、raw path 或对象内容的安全事件。
func (s *Sink) Record(ctx context.Context, event model.AuditEvent) error {
	if ctx == nil {
		return fmt.Errorf("auth audit context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.Operation == "" && event.Action == "" || event.Outcome == "" {
		return fmt.Errorf("auth audit event is incomplete")
	}
	s.logger.Info("security decision",
		logger.String("operation", event.Operation),
		logger.String("action", string(event.Action)),
		logger.String("actor_kind", string(event.Principal.Kind)),
		logger.String("subject_hash", digest(event.Principal.Subject)),
		logger.String("resource_type", event.Resource.Type),
		logger.String("resource_hash", digest(event.Resource.ID)),
		logger.String("decision", string(event.Decision.Reason)),
		logger.String("outcome", string(event.Outcome)),
	)
	return nil
}

func digest(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

var _ service.AuditSink = (*Sink)(nil)
