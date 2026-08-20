// Package messaging 定义业务模块声明消息生产、消费与治理要求时使用的稳定公共契约。
// 具体 Broker Client、物理 topology、连接和确认句柄不得越过本包边界。
package messaging

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	maxIdentityLength   = 160
	maxContentTypeLen   = 128
	maxFingerprintLen   = 256
	maxMetadataLength   = 512
	maxPayloadBytes     = 16 << 20
	maxDeliveries       = 1_000
	maxConsumerParallel = 10_000
)

var (
	identityPattern  = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$`)
	messageIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
)

// ContractID 标识稳定 wire contract，不包含 Broker destination。
type ContractID string

// ProducerID 标识一个稳定生产者能力。
type ProducerID string

// ConsumerID 标识一个稳定消费者能力。
type ConsumerID string

// RouteID 标识由 composition 解析的逻辑消息路由。
type RouteID string

// MessageID 标识一次逻辑消息；重投递必须保持不变。
type MessageID string

// SchemaVersion 是 wire contract 的显式版本。
type SchemaVersion uint32

// ContractRef 是消息与 Binding 引用的稳定 contract identity。
type ContractRef struct {
	id      ContractID
	version SchemaVersion
}

// ID 返回 contract ID。
func (r ContractRef) ID() ContractID { return r.id }

// Version 返回 schema 版本。
func (r ContractRef) Version() SchemaVersion { return r.version }

// String 返回适合诊断的稳定低敏 identity。
func (r ContractRef) String() string { return fmt.Sprintf("%s.v%d", r.id, r.version) }

// ContractSpec 是构造不可变 Contract 的输入。
type ContractSpec struct {
	ID              ContractID
	Version         SchemaVersion
	ContentType     string
	MaxPayloadBytes int
	Fingerprint     string
}

// Contract 描述 wire identity、内容类型、大小和 schema fingerprint。
type Contract struct {
	ref             ContractRef
	contentType     string
	maxPayloadBytes int
	fingerprint     string
}

// DefineContract 构造并校验不可变 Contract。
func DefineContract(spec ContractSpec) (Contract, error) {
	contentType := strings.TrimSpace(spec.ContentType)
	fingerprint := strings.TrimSpace(spec.Fingerprint)
	if err := validateIdentity("contract", string(spec.ID)); err != nil {
		return Contract{}, err
	}
	if spec.Version == 0 {
		return Contract{}, fmt.Errorf("%w: schema version must be positive", ErrInvalidContract)
	}
	if contentType == "" || len(contentType) > maxContentTypeLen {
		return Contract{}, fmt.Errorf("%w: content type is empty or too long", ErrInvalidContract)
	}
	if spec.MaxPayloadBytes <= 0 || spec.MaxPayloadBytes > maxPayloadBytes {
		return Contract{}, fmt.Errorf("%w: payload limit must be within 1..%d bytes", ErrInvalidContract, maxPayloadBytes)
	}
	if fingerprint == "" || len(fingerprint) > maxFingerprintLen {
		return Contract{}, fmt.Errorf("%w: fingerprint is empty or too long", ErrInvalidContract)
	}
	return Contract{
		ref: ContractRef{id: spec.ID, version: spec.Version}, contentType: contentType,
		maxPayloadBytes: spec.MaxPayloadBytes, fingerprint: fingerprint,
	}, nil
}

// Ref 返回稳定 identity。
func (c Contract) Ref() ContractRef { return c.ref }

// ContentType 返回 wire content type。
func (c Contract) ContentType() string { return c.contentType }

// MaxPayloadBytes 返回 payload 硬上限。
func (c Contract) MaxPayloadBytes() int { return c.maxPayloadBytes }

// Fingerprint 返回用于冲突检测的 schema fingerprint。
func (c Contract) Fingerprint() string { return c.fingerprint }

func (c Contract) sameDefinition(other Contract) bool {
	return c.ref == other.ref && c.contentType == other.contentType &&
		c.maxPayloadBytes == other.maxPayloadBytes && c.fingerprint == other.fingerprint
}

func (c Contract) validate() error {
	_, err := DefineContract(ContractSpec{
		ID: c.ref.id, Version: c.ref.version, ContentType: c.contentType,
		MaxPayloadBytes: c.maxPayloadBytes, Fingerprint: c.fingerprint,
	})
	return err
}

// MessageSpec 是构造不可变 Message 的输入。
type MessageSpec struct {
	ID            MessageID
	Contract      ContractRef
	OccurredAt    time.Time
	OrderingKey   string
	CorrelationID string
	CausationID   string
	Payload       []byte
}

// Message 是 Provider 无关的不可变消息 Envelope。
type Message struct {
	id            MessageID
	contract      ContractRef
	occurredAt    time.Time
	orderingKey   string
	correlationID string
	causationID   string
	payload       []byte
}

// NewMessage 构造不可变 Message，并复制 payload 防止调用方后续修改。
func NewMessage(spec MessageSpec) (Message, error) {
	if err := validateMessageID(spec.ID); err != nil {
		return Message{}, err
	}
	if err := validateContractRef(spec.Contract); err != nil {
		return Message{}, err
	}
	if spec.OccurredAt.IsZero() {
		return Message{}, fmt.Errorf("%w: occurred time is required", ErrInvalidMessage)
	}
	for name, value := range map[string]string{
		"ordering key": spec.OrderingKey, "correlation id": spec.CorrelationID, "causation id": spec.CausationID,
	} {
		if len(value) > maxMetadataLength || strings.ContainsAny(value, "\r\n\x00") {
			return Message{}, fmt.Errorf("%w: %s is invalid", ErrInvalidMessage, name)
		}
	}
	if len(spec.Payload) > maxPayloadBytes {
		return Message{}, fmt.Errorf("%w: payload exceeds package limit", ErrInvalidMessage)
	}
	return Message{
		id: spec.ID, contract: spec.Contract, occurredAt: spec.OccurredAt,
		orderingKey: strings.TrimSpace(spec.OrderingKey), correlationID: strings.TrimSpace(spec.CorrelationID),
		causationID: strings.TrimSpace(spec.CausationID), payload: append([]byte(nil), spec.Payload...),
	}, nil
}

// ID 返回 Message ID。
func (m Message) ID() MessageID { return m.id }

// Contract 返回 wire contract identity。
func (m Message) Contract() ContractRef { return m.contract }

// OccurredAt 返回业务发生时间。
func (m Message) OccurredAt() time.Time { return m.occurredAt }

// OrderingKey 返回调用方显式声明的顺序键。
func (m Message) OrderingKey() string { return m.orderingKey }

// CorrelationID 返回跨消息关联 identity。
func (m Message) CorrelationID() string { return m.correlationID }

// CausationID 返回因果 identity。
func (m Message) CausationID() string { return m.causationID }

// Payload 返回 payload 副本，避免跨边界共享可变字节。
func (m Message) Payload() []byte { return append([]byte(nil), m.payload...) }

// ConfirmRequirement 表示发布成功所需的 Broker 证明。
type ConfirmRequirement string

const (
	// ConfirmBroker 表示只有 Broker 确认接管才可返回成功。
	ConfirmBroker ConfirmRequirement = "broker"
)

// Importance 表示 Route 不可用时对应用 readiness 的影响。
type Importance string

const (
	// ImportanceRequired 表示依赖不可用时 readiness 必须失败。
	ImportanceRequired Importance = "required"
	// ImportanceOptional 表示依赖不可用时 health 降级但 readiness 可保持。
	ImportanceOptional Importance = "optional"
)

// DeadLetterRequirement 表示无法处理的消息是否必须由 Provider 保留到死信目标。
type DeadLetterRequirement string

const (
	// DeadLetterRequired 表示 Provider 必须证明具备受治理的死信能力。
	DeadLetterRequired DeadLetterRequirement = "required"
)

// DeliveryPolicy 定义业务交付预算和 Execution 生命周期窗口。
type DeliveryPolicy struct {
	maxDeliveries        uint64
	handlerTimeout       time.Duration
	processingLease      time.Duration
	idempotencyRetention time.Duration
	deadLetter           DeadLetterRequirement
}

// NewDeliveryPolicy 构造有界交付策略。
func NewDeliveryPolicy(maxDeliveryCount uint64, handlerTimeout, processingLease, idempotencyRetention time.Duration, deadLetter DeadLetterRequirement) (DeliveryPolicy, error) {
	policy := DeliveryPolicy{
		maxDeliveries: maxDeliveryCount, handlerTimeout: handlerTimeout, processingLease: processingLease,
		idempotencyRetention: idempotencyRetention, deadLetter: deadLetter,
	}
	if err := policy.Validate(); err != nil {
		return DeliveryPolicy{}, err
	}
	return policy, nil
}

// MaxDeliveries 返回业务失败允许的最大 delivery 次数。
func (p DeliveryPolicy) MaxDeliveries() uint64 { return p.maxDeliveries }

// HandlerTimeout 返回单次业务处理超时。
func (p DeliveryPolicy) HandlerTimeout() time.Duration { return p.handlerTimeout }

// ProcessingLease 返回 Execution running lease。
func (p DeliveryPolicy) ProcessingLease() time.Duration { return p.processingLease }

// IdempotencyRetention 返回成功后的 Message ID 去重窗口。
func (p DeliveryPolicy) IdempotencyRetention() time.Duration { return p.idempotencyRetention }

// DeadLetter 返回死信要求。
func (p DeliveryPolicy) DeadLetter() DeadLetterRequirement { return p.deadLetter }

// Validate 校验交付预算、超时和 lease/retention 必须有界且语义一致。
func (p DeliveryPolicy) Validate() error {
	if p.maxDeliveries <= 0 || p.maxDeliveries > maxDeliveries {
		return fmt.Errorf("%w: max deliveries must be within 1..%d", ErrInvalidDelivery, maxDeliveries)
	}
	if p.handlerTimeout <= 0 || p.processingLease <= p.handlerTimeout || p.idempotencyRetention <= 0 {
		return fmt.Errorf("%w: timeout must be positive, lease must exceed timeout, retention must be positive", ErrInvalidDelivery)
	}
	if p.deadLetter != DeadLetterRequired {
		return fmt.Errorf("%w: unsupported dead-letter requirement %q", ErrInvalidDelivery, p.deadLetter)
	}
	return nil
}

// ConcurrencyPolicy 定义 Consumer 并发与 Broker prefetch 上限。
type ConcurrencyPolicy struct {
	maxConcurrent int
	prefetch      int
}

// NewConcurrencyPolicy 构造有界 Consumer 并发策略。
func NewConcurrencyPolicy(maxConcurrent, prefetch int) (ConcurrencyPolicy, error) {
	policy := ConcurrencyPolicy{maxConcurrent: maxConcurrent, prefetch: prefetch}
	if err := policy.Validate(); err != nil {
		return ConcurrencyPolicy{}, err
	}
	return policy, nil
}

// MaxConcurrent 返回 Handler 并发上限。
func (p ConcurrencyPolicy) MaxConcurrent() int { return p.maxConcurrent }

// Prefetch 返回 Broker 未确认 delivery 上限。
func (p ConcurrencyPolicy) Prefetch() int { return p.prefetch }

// Validate 校验并发和 prefetch 必须为正数、有上限，且 prefetch 不小于并发数。
func (p ConcurrencyPolicy) Validate() error {
	if p.maxConcurrent <= 0 || p.maxConcurrent > maxConsumerParallel || p.prefetch < p.maxConcurrent || p.prefetch > maxConsumerParallel {
		return fmt.Errorf("%w: concurrency/prefetch must be positive, bounded and prefetch >= concurrency", ErrInvalidConcurrency)
	}
	return nil
}

// Handler 是模块 binding Adapter 提供的消息处理边界。
type Handler func(context.Context, Message) error

// ProducerSpec 是构造 Producer Binding 的输入。
type ProducerSpec struct {
	ID       ProducerID
	Contract ContractRef
	Route    RouteID
	Confirm  ConfirmRequirement
}

// ProducerBinding 是不可变生产声明。
type ProducerBinding struct {
	id       ProducerID
	contract ContractRef
	route    RouteID
	confirm  ConfirmRequirement
}

// BindProducer 构造不可变 Producer Binding。
func BindProducer(spec ProducerSpec) (ProducerBinding, error) {
	if err := validateIdentity("producer", string(spec.ID)); err != nil {
		return ProducerBinding{}, err
	}
	if err := validateContractRef(spec.Contract); err != nil {
		return ProducerBinding{}, err
	}
	if err := validateIdentity("route", string(spec.Route)); err != nil {
		return ProducerBinding{}, err
	}
	if spec.Confirm != ConfirmBroker {
		return ProducerBinding{}, fmt.Errorf("%w: unsupported confirm requirement %q", ErrInvalidBinding, spec.Confirm)
	}
	return ProducerBinding{id: spec.ID, contract: spec.Contract, route: spec.Route, confirm: spec.Confirm}, nil
}

// ID 返回 Producer ID。
func (b ProducerBinding) ID() ProducerID { return b.id }

// Contract 返回引用的 Contract。
func (b ProducerBinding) Contract() ContractRef { return b.contract }

// Route 返回逻辑 Route。
func (b ProducerBinding) Route() RouteID { return b.route }

// Confirm 返回发布确认要求。
func (b ProducerBinding) Confirm() ConfirmRequirement { return b.confirm }

func (b ProducerBinding) validate() error {
	_, err := BindProducer(ProducerSpec{ID: b.id, Contract: b.contract, Route: b.route, Confirm: b.confirm})
	return err
}

// ConsumerSpec 是构造 Consumer Binding 的输入。
type ConsumerSpec struct {
	ID          ConsumerID
	Contract    ContractRef
	Route       RouteID
	Delivery    DeliveryPolicy
	Concurrency ConcurrencyPolicy
	Importance  Importance
}

// ConsumerBinding 是不可变消费声明。
type ConsumerBinding struct {
	id          ConsumerID
	contract    ContractRef
	route       RouteID
	delivery    DeliveryPolicy
	concurrency ConcurrencyPolicy
	importance  Importance
	handler     Handler
}

// BindConsumer 构造不可变 Consumer Binding，不启动网络或 goroutine。
func BindConsumer(spec ConsumerSpec, handler Handler) (ConsumerBinding, error) {
	if err := validateIdentity("consumer", string(spec.ID)); err != nil {
		return ConsumerBinding{}, err
	}
	if err := validateContractRef(spec.Contract); err != nil {
		return ConsumerBinding{}, err
	}
	if err := validateIdentity("route", string(spec.Route)); err != nil {
		return ConsumerBinding{}, err
	}
	if handler == nil {
		return ConsumerBinding{}, ErrNilHandler
	}
	if err := spec.Delivery.Validate(); err != nil {
		return ConsumerBinding{}, err
	}
	if err := spec.Concurrency.Validate(); err != nil {
		return ConsumerBinding{}, err
	}
	if spec.Importance != ImportanceRequired && spec.Importance != ImportanceOptional {
		return ConsumerBinding{}, fmt.Errorf("%w: unsupported importance %q", ErrInvalidBinding, spec.Importance)
	}
	return ConsumerBinding{
		id: spec.ID, contract: spec.Contract, route: spec.Route, delivery: spec.Delivery,
		concurrency: spec.Concurrency, importance: spec.Importance, handler: handler,
	}, nil
}

// ID 返回 Consumer ID。
func (b ConsumerBinding) ID() ConsumerID { return b.id }

// Contract 返回引用的 Contract。
func (b ConsumerBinding) Contract() ContractRef { return b.contract }

// Route 返回逻辑 Route。
func (b ConsumerBinding) Route() RouteID { return b.route }

// Delivery 返回交付策略。
func (b ConsumerBinding) Delivery() DeliveryPolicy { return b.delivery }

// Concurrency 返回并发策略。
func (b ConsumerBinding) Concurrency() ConcurrencyPolicy { return b.concurrency }

// Importance 返回故障重要性。
func (b ConsumerBinding) Importance() Importance { return b.importance }

// Handle 调用模块 Handler；只有统一 Consumer runtime 可以调用。
func (b ConsumerBinding) Handle(ctx context.Context, message Message) error {
	return b.handler(ctx, message)
}

func (b ConsumerBinding) validate() error {
	_, err := BindConsumer(ConsumerSpec{
		ID: b.id, Contract: b.contract, Route: b.route, Delivery: b.delivery,
		Concurrency: b.concurrency, Importance: b.importance,
	}, b.handler)
	return err
}

// Contribution 是模块显式输出的消息声明集合。
type Contribution struct {
	contracts []Contract
	producers []ProducerBinding
	consumers []ConsumerBinding
}

// Contribute 构造不可变模块消息贡献；跨模块冲突由 BuildCatalog 统一判断。
func Contribute(contracts []Contract, producers []ProducerBinding, consumers []ConsumerBinding) Contribution {
	return Contribution{
		contracts: append([]Contract(nil), contracts...), producers: append([]ProducerBinding(nil), producers...),
		consumers: append([]ConsumerBinding(nil), consumers...),
	}
}

// Contracts 返回 Contract 副本。
func (c Contribution) Contracts() []Contract { return append([]Contract(nil), c.contracts...) }

// Producers 返回 Producer Binding 副本。
func (c Contribution) Producers() []ProducerBinding {
	return append([]ProducerBinding(nil), c.producers...)
}

// Consumers 返回 Consumer Binding 副本。
func (c Contribution) Consumers() []ConsumerBinding {
	return append([]ConsumerBinding(nil), c.consumers...)
}

func validateIdentity(kind, value string) error {
	if len(value) > maxIdentityLength || !identityPattern.MatchString(value) {
		return fmt.Errorf("%w: invalid %s identity %q", ErrInvalidIdentity, kind, value)
	}
	return nil
}

func validateMessageID(id MessageID) error {
	value := string(id)
	if len(value) > maxIdentityLength || !messageIDPattern.MatchString(value) {
		return fmt.Errorf("%w: invalid message identity %q", ErrInvalidIdentity, value)
	}
	return nil
}

func validateContractRef(ref ContractRef) error {
	if err := validateIdentity("contract", string(ref.id)); err != nil {
		return err
	}
	if ref.version == 0 {
		return fmt.Errorf("%w: schema version must be positive", ErrInvalidContract)
	}
	return nil
}
