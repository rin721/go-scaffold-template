package messaging

import (
	"context"
	"time"

	pkgclock "github.com/rin721/go-scaffold-template/pkg/clock"
	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
	pkgmessaging "github.com/rin721/go-scaffold-template/pkg/messaging"
)

// Capabilities 描述 Provider 能够可靠满足的公共消息语义。
type Capabilities struct {
	PublisherConfirm bool
	MandatoryRoute   bool
	ManualAck        bool
	DelayedRetry     bool
	DeadLetter       bool
}

// ProviderDependencies 是 Factory 构造资源时可用的项目稳定能力。
type ProviderDependencies struct {
	Logger   pkglogger.Logger
	Clock    pkgclock.Clock
	Recovery RecoveryConfig
}

// Factory 由 composition 显式提交，避免业务模块或 Kernel App 隐式查找 Driver。
type Factory interface {
	Kind() Driver
	Build(context.Context, string, ProviderConfig, ProviderDependencies) (Provider, error)
}

// Route 是 Provider 使用的物理 topology 快照，只存在于 internal 边界。
type Route struct {
	ID                    pkgmessaging.RouteID
	Exchange              string
	ExchangeType          string
	RoutingKey            string
	Queue                 string
	QueueType             string
	Reliable              bool
	DeliveryLimit         uint64
	DelayedRetryMin       time.Duration
	DelayedRetryMax       time.Duration
	DeadLetterExchange    string
	DeadLetterRoutingKey  string
	AtLeastOnceDeadLetter bool
	Contract              pkgmessaging.ContractRef
	ContentType           string
	MaxPayloadBytes       int
}

// Incoming 是 Provider 验证 Envelope 后提交给统一 Consumer runtime 的 delivery。
type Incoming struct {
	Message       pkgmessaging.Message
	DeliveryCount uint64
	Redelivered   bool
	TraceID       string
}

// Disposition 是统一 Consumer runtime 对单个 delivery 的最终处置决定。
type Disposition uint8

const (
	DispositionAck Disposition = iota + 1
	DispositionRetryCounted
	DispositionDeferUncounted
	DispositionDeadLetter
)

// Consumer 把 Binding、物理 Route 与治理 Handler 显式交给 Provider。
type Consumer struct {
	Binding pkgmessaging.ConsumerBinding
	Route   Route
	Handle  func(context.Context, Incoming) Disposition
}

// PublishResult 是 Provider 确认接管后的低敏证明。
type PublishResult struct {
	ConfirmedAt time.Time
	Reference   string
}

// Provider 只承载中间件接入、确认、重投递、死信和连接恢复，不包含业务逻辑。
type Provider interface {
	Capabilities() Capabilities
	Bind([]Consumer) error
	Activate(context.Context) error
	Deactivate(context.Context) error
	Publish(context.Context, Route, pkgmessaging.Message) (PublishResult, error)
	Diagnostics() pkgmessaging.ProviderDiagnostics
	Close(context.Context) error
}
