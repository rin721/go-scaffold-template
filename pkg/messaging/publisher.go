package messaging

import (
	"context"
	"time"
)

// Receipt 只证明 Provider 已确认接管消息，不证明消费完成或业务副作用 exactly-once。
type Receipt struct {
	MessageID   MessageID
	ProducerID  ProducerID
	ConfirmedAt time.Time
	Reference   string
}

// Publisher 是业务模块发布消息时使用的稳定 Provider 无关契约。
type Publisher interface {
	Publish(context.Context, ProducerID, Message) (Receipt, error)
}

// ProviderState 描述低敏 Provider 运行状态。
type ProviderState string

const (
	ProviderDisabled   ProviderState = "disabled"
	ProviderConnecting ProviderState = "connecting"
	ProviderReady      ProviderState = "ready"
	ProviderRecovering ProviderState = "recovering"
	ProviderFailed     ProviderState = "failed"
	ProviderDraining   ProviderState = "draining"
	ProviderStopped    ProviderState = "stopped"
)

// ProviderDiagnostics 是 Ops 使用的低敏、自包含 Provider 快照。
type ProviderDiagnostics struct {
	Name          string        `json:"name"`
	Driver        string        `json:"driver"`
	State         ProviderState `json:"state"`
	Ready         bool          `json:"ready"`
	InFlight      int64         `json:"inFlight"`
	Confirmed     uint64        `json:"confirmed"`
	Failed        uint64        `json:"failed"`
	Recoveries    uint64        `json:"recoveries"`
	LastErrorType string        `json:"lastErrorType,omitempty"`
}

// ConsumerDiagnostics 是 Ops 使用的低敏 Consumer 快照。
type ConsumerDiagnostics struct {
	ID            ConsumerID `json:"id"`
	Route         RouteID    `json:"route"`
	Active        bool       `json:"active"`
	InFlight      int64      `json:"inFlight"`
	Redelivered   uint64     `json:"redelivered"`
	Acknowledged  uint64     `json:"acknowledged"`
	Rejected      uint64     `json:"rejected"`
	DeadLettered  uint64     `json:"deadLettered"`
	LastErrorType string     `json:"lastErrorType,omitempty"`
}

// Diagnostics 是 messaging capability 的低敏快照。
type Diagnostics struct {
	Enabled   bool                  `json:"enabled"`
	Providers []ProviderDiagnostics `json:"providers"`
	Consumers []ConsumerDiagnostics `json:"consumers"`
}
