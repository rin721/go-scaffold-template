// Package rabbitmq 使用 amqp091-go 实现受治理的 RabbitMQ 消息 Provider。
package rabbitmq

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	messagingapp "github.com/rin721/go-scaffold-template/internal/kernel/app/messaging"
	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
	pkgmessaging "github.com/rin721/go-scaffold-template/pkg/messaging"
	pkgobservability "github.com/rin721/go-scaffold-template/pkg/observability"
)

const (
	headerContractID     = "x-contract-id"
	headerContractVer    = "x-contract-version"
	headerOrderingKey    = "x-ordering-key"
	headerCausationID    = "x-causation-id"
	headerTraceID        = "x-trace-id"
	headerDeliveryCount  = "x-delivery-count"
	consumerTagPrefix    = "go-scaffold"
	maxHeaderValueLength = 512
)

// Factory 返回 RabbitMQ Driver 的显式 Provider Factory。
func Factory() messagingapp.Factory { return factory{} }

type factory struct{}

func (factory) Kind() messagingapp.Driver { return messagingapp.DriverRabbitMQ }

func (factory) Build(
	ctx context.Context,
	name string,
	config messagingapp.ProviderConfig,
	dependencies messagingapp.ProviderDependencies,
) (messagingapp.Provider, error) {
	if ctx == nil {
		return nil, fmt.Errorf("rabbitmq provider context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if config.Driver != messagingapp.DriverRabbitMQ || dependencies.Logger == nil || dependencies.Clock == nil {
		return nil, fmt.Errorf("rabbitmq provider dependencies or driver are invalid")
	}
	parsed, err := url.Parse(config.RabbitMQ.URI)
	if err != nil || (parsed.Scheme != "amqp" && parsed.Scheme != "amqps") || parsed.Host == "" {
		return nil, fmt.Errorf("rabbitmq provider URI is invalid")
	}
	if config.RabbitMQ.TLS.Enabled && parsed.Scheme != "amqps" {
		return nil, fmt.Errorf("rabbitmq TLS requires an amqps URI")
	}
	tlsConfig, err := loadTLS(config.RabbitMQ.TLS)
	if err != nil {
		return nil, err
	}
	ownedCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	provider := &provider{
		name: name, config: config.RabbitMQ, dependencies: dependencies,
		tlsConfig: tlsConfig, ctx: ownedCtx, cancel: cancel, activation: make(chan struct{}, 1),
	}
	provider.state.Store(pkgmessaging.ProviderConnecting)
	provider.lastError.Store("")
	return provider, nil
}

type provider struct {
	name         string
	config       messagingapp.RabbitMQConfig
	dependencies messagingapp.ProviderDependencies
	tlsConfig    *tls.Config
	ctx          context.Context
	cancel       context.CancelFunc
	activation   chan struct{}

	bindOnce  sync.Once
	bindErr   error
	closeOnce sync.Once
	closeErr  error
	wg        sync.WaitGroup
	cleanupWG sync.WaitGroup
	active    atomic.Bool
	stopped   atomic.Bool

	mu        sync.RWMutex
	consumers []messagingapp.Consumer
	current   *session

	publishMu  sync.Mutex
	state      atomic.Value
	lastError  atomic.Value
	inFlight   atomic.Int64
	confirmed  atomic.Uint64
	failed     atomic.Uint64
	recoveries atomic.Uint64
}

type session struct {
	mu               sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	connection       *amqp.Connection
	publish          *amqp.Channel
	confirms         <-chan amqp.Confirmation
	returns          <-chan amqp.Return
	closed           <-chan *amqp.Error
	publishClosed    <-chan *amqp.Error
	connectionClosed <-chan *amqp.Error
	failure          chan error
	consuming        bool
	stopping         bool
	resetAfterStop   bool
	stopDone         chan struct{}
	wg               sync.WaitGroup
}

func (*provider) Capabilities() messagingapp.Capabilities {
	return messagingapp.Capabilities{
		PublisherConfirm: true, MandatoryRoute: true, ManualAck: true, DelayedRetry: true, DeadLetter: true,
	}
}

func (p *provider) Bind(consumers []messagingapp.Consumer) error {
	p.bindOnce.Do(func() {
		seen := make(map[pkgmessaging.ConsumerID]struct{}, len(consumers))
		for index, consumer := range consumers {
			if consumer.Handle == nil {
				p.bindErr = fmt.Errorf("rabbitmq consumer %d handler is nil", index)
				return
			}
			if _, exists := seen[consumer.Binding.ID()]; exists {
				p.bindErr = fmt.Errorf("rabbitmq consumer %q is duplicated", consumer.Binding.ID())
				return
			}
			seen[consumer.Binding.ID()] = struct{}{}
			if consumer.Route.Queue == "" || consumer.Route.MaxPayloadBytes <= 0 {
				p.bindErr = fmt.Errorf("rabbitmq consumer %q route is incomplete", consumer.Binding.ID())
				return
			}
		}
		p.consumers = append([]messagingapp.Consumer(nil), consumers...)
		current, err := p.connect()
		if err != nil && deterministicBrokerError(err) {
			p.setState(pkgmessaging.ProviderFailed, err)
			p.bindErr = fmt.Errorf("verify RabbitMQ provider %q: %w", p.name, err)
			return
		}
		attempt := 0
		if err != nil {
			p.setState(pkgmessaging.ProviderRecovering, err)
			attempt = 1
		} else {
			p.mu.Lock()
			p.current = current
			p.mu.Unlock()
			p.setState(pkgmessaging.ProviderReady, nil)
		}
		p.wg.Add(1)
		go p.run(current, attempt)
	})
	return p.bindErr
}

func (p *provider) Activate(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("rabbitmq activate context is nil")
	}
	if p.stopped.Load() {
		return pkgmessaging.ErrRetired
	}
	p.active.Store(true)
	select {
	case p.activation <- struct{}{}:
	default:
	}
	return nil
}

func (p *provider) Deactivate(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("rabbitmq deactivate context is nil")
	}
	p.active.Store(false)
	p.mu.RLock()
	current := p.current
	p.mu.RUnlock()
	if current == nil {
		return nil
	}
	return p.stopConsumers(ctx, current, true)
}

func (p *provider) Close(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("rabbitmq close context is nil")
	}
	p.closeOnce.Do(func() {
		p.closeErr = errors.Join(p.closeErr, p.Deactivate(ctx))
		p.stopped.Store(true)
		p.setState(pkgmessaging.ProviderDraining, nil)
		p.cancel()
		p.mu.RLock()
		current := p.current
		p.mu.RUnlock()
		if current != nil && current.connection != nil {
			p.closeErr = errors.Join(p.closeErr, closeConnection(current.connection))
		}
		done := make(chan struct{})
		go func() {
			p.wg.Wait()
			p.cleanupWG.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-ctx.Done():
			p.closeErr = errors.Join(p.closeErr, ctx.Err())
		}
		p.setState(pkgmessaging.ProviderStopped, nil)
	})
	return p.closeErr
}

func (p *provider) Publish(ctx context.Context, route messagingapp.Route, message pkgmessaging.Message) (messagingapp.PublishResult, error) {
	if ctx == nil {
		return messagingapp.PublishResult{}, fmt.Errorf("%w: nil context", pkgmessaging.ErrInvalidMessage)
	}
	if p.stopped.Load() {
		return messagingapp.PublishResult{}, pkgmessaging.ErrUnavailable
	}
	if route.Contract != message.Contract() || len(message.Payload()) > route.MaxPayloadBytes {
		return messagingapp.PublishResult{}, pkgmessaging.ErrContractMismatch
	}
	p.publishMu.Lock()
	defer p.publishMu.Unlock()
	p.mu.RLock()
	current := p.current
	p.mu.RUnlock()
	if current == nil || current.publish == nil || p.currentState() != pkgmessaging.ProviderReady {
		return messagingapp.PublishResult{}, pkgmessaging.ErrUnavailable
	}
	p.inFlight.Add(1)
	defer p.inFlight.Add(-1)
	publishing := amqp.Publishing{
		Headers: amqp.Table{
			headerContractID: string(message.Contract().ID()), headerContractVer: int64(message.Contract().Version()),
			headerOrderingKey: message.OrderingKey(), headerCausationID: message.CausationID(),
			headerTraceID: boundedHeader(pkgobservability.TraceIDFrom(ctx)),
		},
		ContentType: route.ContentType, DeliveryMode: amqp.Persistent,
		CorrelationId: message.CorrelationID(), MessageId: string(message.ID()),
		Timestamp: message.OccurredAt(), Type: message.Contract().String(), Body: message.Payload(),
	}
	if err := current.publish.PublishWithContext(ctx, route.Exchange, route.RoutingKey, true, false, publishing); err != nil {
		p.failed.Add(1)
		if ctx.Err() != nil {
			return messagingapp.PublishResult{}, fmt.Errorf("%w: %w", pkgmessaging.ErrPublishAmbiguous, ctx.Err())
		}
		return messagingapp.PublishResult{}, fmt.Errorf("%w: %w", pkgmessaging.ErrPublishAmbiguous, err)
	}
	returned := false
	for {
		select {
		case <-ctx.Done():
			p.failed.Add(1)
			return messagingapp.PublishResult{}, fmt.Errorf("%w: %w", pkgmessaging.ErrPublishAmbiguous, ctx.Err())
		case _, ok := <-current.returns:
			if ok {
				returned = true
			}
		case confirmation, ok := <-current.confirms:
			if !ok {
				p.failed.Add(1)
				return messagingapp.PublishResult{}, pkgmessaging.ErrPublishAmbiguous
			}
			select {
			case <-current.returns:
				returned = true
			default:
			}
			if returned {
				p.failed.Add(1)
				return messagingapp.PublishResult{}, pkgmessaging.ErrUnroutable
			}
			if !confirmation.Ack {
				p.failed.Add(1)
				return messagingapp.PublishResult{}, pkgmessaging.ErrPublishRejected
			}
			p.confirmed.Add(1)
			return messagingapp.PublishResult{
				ConfirmedAt: p.dependencies.Clock.Now(), Reference: "delivery:" + strconv.FormatUint(confirmation.DeliveryTag, 10),
			}, nil
		case <-current.closed:
			p.failed.Add(1)
			return messagingapp.PublishResult{}, pkgmessaging.ErrPublishAmbiguous
		}
	}
}

func (p *provider) Diagnostics() pkgmessaging.ProviderDiagnostics {
	state := p.currentState()
	lastError, _ := p.lastError.Load().(string)
	return pkgmessaging.ProviderDiagnostics{
		Name: p.name, Driver: string(messagingapp.DriverRabbitMQ), State: state,
		Ready: state == pkgmessaging.ProviderReady, InFlight: p.inFlight.Load(),
		Confirmed: p.confirmed.Load(), Failed: p.failed.Load(), Recoveries: p.recoveries.Load(), LastErrorType: lastError,
	}
}

func (p *provider) run(current *session, attempt int) {
	defer p.wg.Done()
	everConnected := current != nil
	for p.ctx.Err() == nil {
		if current == nil {
			if attempt > 0 {
				p.setState(pkgmessaging.ProviderRecovering, nil)
				if !wait(p.ctx, recoveryDelay(attempt, p.dependencies.Recovery)) {
					return
				}
			} else if !everConnected {
				p.setState(pkgmessaging.ProviderConnecting, nil)
			}
			connected, err := p.connect()
			if err != nil {
				if deterministicBrokerError(err) {
					p.setState(pkgmessaging.ProviderFailed, err)
					return
				}
				p.setState(pkgmessaging.ProviderRecovering, err)
				attempt++
				continue
			}
			current = connected
			if everConnected {
				p.recoveries.Add(1)
			}
			everConnected = true
			attempt = 0
			p.mu.Lock()
			p.current = current
			p.mu.Unlock()
			p.setState(pkgmessaging.ProviderReady, nil)
		}
		if p.active.Load() {
			if err := p.startConsumers(current); err != nil {
				failed := deterministicBrokerError(err)
				if failed {
					p.setState(pkgmessaging.ProviderFailed, err)
				} else {
					p.setState(pkgmessaging.ProviderRecovering, err)
				}
				_ = p.stopConsumers(context.Background(), current, false)
				_ = closeConnection(current.connection)
				p.clear(current)
				if failed {
					return
				}
				current = nil
				attempt = 1
				continue
			}
		}
		reconnect := false
		failed := false
		for !reconnect {
			select {
			case <-p.ctx.Done():
				_ = p.stopConsumers(context.Background(), current, false)
				_ = closeConnection(current.connection)
				p.clear(current)
				return
			case <-p.activation:
				if p.active.Load() && !current.isConsuming() {
					if err := p.startConsumers(current); err != nil {
						failed = deterministicBrokerError(err)
						if failed {
							p.setState(pkgmessaging.ProviderFailed, err)
						} else {
							p.setState(pkgmessaging.ProviderRecovering, err)
						}
						_ = p.stopConsumers(context.Background(), current, false)
						_ = closeConnection(current.connection)
						reconnect = true
					}
				}
			case err := <-current.failure:
				failed = deterministicBrokerError(err)
				if failed {
					p.setState(pkgmessaging.ProviderFailed, err)
				} else {
					p.setState(pkgmessaging.ProviderRecovering, err)
				}
				_ = p.stopConsumers(context.Background(), current, false)
				_ = closeConnection(current.connection)
				reconnect = true
			case err, ok := <-current.publishClosed:
				if ok && err != nil {
					failed = deterministicBrokerError(err)
					if failed {
						p.setState(pkgmessaging.ProviderFailed, err)
					} else {
						p.setState(pkgmessaging.ProviderRecovering, err)
					}
				}
				_ = p.stopConsumers(context.Background(), current, false)
				_ = closeConnection(current.connection)
				reconnect = true
			case err, ok := <-current.connectionClosed:
				if ok && err != nil {
					failed = deterministicBrokerError(err)
					if failed {
						p.setState(pkgmessaging.ProviderFailed, err)
					} else {
						p.setState(pkgmessaging.ProviderRecovering, err)
					}
				}
				_ = p.stopConsumers(context.Background(), current, false)
				reconnect = true
			}
		}
		_ = p.stopConsumers(context.Background(), current, false)
		p.clear(current)
		current = nil
		if failed {
			return
		}
		attempt = 1
	}
}

func (p *provider) connect() (*session, error) {
	dialer := &net.Dialer{Timeout: p.dependencies.Recovery.ConnectTimeout}
	configuration := amqp.Config{
		Heartbeat: p.config.Heartbeat, TLSClientConfig: p.tlsConfig,
		Properties: amqp.Table{"connection_name": "go-scaffold.messaging." + p.name},
	}
	var transport net.Conn
	configuration.Dial = func(network, address string) (net.Conn, error) {
		connection, err := dialer.DialContext(p.ctx, network, address)
		if err != nil {
			return nil, err
		}
		if err := connection.SetDeadline(time.Now().Add(p.dependencies.Recovery.ConnectTimeout)); err != nil {
			_ = connection.Close()
			return nil, err
		}
		transport = connection
		return connection, nil
	}
	connection, err := amqp.DialConfig(p.config.URI, configuration)
	if err != nil {
		return nil, err
	}
	if transport != nil {
		if err := transport.SetDeadline(time.Time{}); err != nil {
			return nil, errors.Join(err, connection.Close())
		}
	}
	publish, err := connection.Channel()
	if err != nil {
		return nil, errors.Join(err, connection.Close())
	}
	if err := publish.Confirm(false); err != nil {
		return nil, errors.Join(err, publish.Close(), connection.Close())
	}
	if err := verifyConsumerTopologies(connection, p.consumers); err != nil {
		return nil, errors.Join(err, publish.Close(), connection.Close())
	}
	sessionCtx, cancel := context.WithCancel(p.ctx)
	return &session{
		ctx: sessionCtx, cancel: cancel, connection: connection, publish: publish,
		confirms:         publish.NotifyPublish(make(chan amqp.Confirmation, 1)),
		returns:          publish.NotifyReturn(make(chan amqp.Return, 1)),
		closed:           publish.NotifyClose(make(chan *amqp.Error, 1)),
		publishClosed:    publish.NotifyClose(make(chan *amqp.Error, 1)),
		connectionClosed: connection.NotifyClose(make(chan *amqp.Error, 1)), failure: make(chan error, 1),
	}, nil
}

func (p *provider) startConsumers(current *session) error {
	current.mu.Lock()
	defer current.mu.Unlock()
	if current.consuming {
		return nil
	}
	if current.ctx.Err() != nil {
		current.ctx, current.cancel = context.WithCancel(p.ctx)
	}
	current.consuming = true
	consumerCtx := current.ctx
	for _, consumer := range p.consumers {
		channel, err := current.connection.Channel()
		if err != nil {
			return err
		}
		if err := verifyTopology(channel, consumer.Route); err != nil {
			return errors.Join(err, channel.Close())
		}
		if err := channel.Qos(consumer.Binding.Concurrency().Prefetch(), 0, false); err != nil {
			return errors.Join(err, channel.Close())
		}
		tag := consumerTagPrefix + "." + p.name + "." + string(consumer.Binding.ID())
		deliveries, err := channel.ConsumeWithContext(consumerCtx, consumer.Route.Queue, tag, false, false, false, false, nil)
		if err != nil {
			return errors.Join(err, channel.Close())
		}
		current.wg.Add(1)
		go p.consume(current, consumerCtx, channel, consumer, deliveries)
	}
	return nil
}

func (p *provider) stopConsumers(ctx context.Context, current *session, reset bool) error {
	if ctx == nil {
		return fmt.Errorf("rabbitmq stop consumers context is nil")
	}
	if current == nil {
		return nil
	}
	current.mu.Lock()
	if !current.consuming {
		current.mu.Unlock()
		return nil
	}
	if !current.stopping {
		current.stopping = true
		current.resetAfterStop = reset
		current.stopDone = make(chan struct{})
		current.cancel()
		p.cleanupWG.Add(1)
		go p.finishConsumerStop(current)
	} else if !reset {
		// 连接恢复或关闭优先于普通 handoff，完成后不得在旧 session 上重新建 Consumer。
		current.resetAfterStop = false
	}
	done := current.stopDone
	current.mu.Unlock()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *provider) finishConsumerStop(current *session) {
	defer p.cleanupWG.Done()
	current.wg.Wait()
	current.mu.Lock()
	reset := current.resetAfterStop && p.ctx.Err() == nil
	current.consuming = false
	current.stopping = false
	current.resetAfterStop = false
	if reset {
		current.ctx, current.cancel = context.WithCancel(p.ctx)
	}
	done := current.stopDone
	current.stopDone = nil
	current.mu.Unlock()
	close(done)
	if reset && p.active.Load() {
		select {
		case p.activation <- struct{}{}:
		default:
		}
	}
}

func (s *session) isConsuming() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.consuming
}

func (p *provider) consume(current *session, consumerCtx context.Context, channel *amqp.Channel, consumer messagingapp.Consumer, deliveries <-chan amqp.Delivery) {
	defer current.wg.Done()
	defer channel.Close()
	slots := make(chan struct{}, consumer.Binding.Concurrency().MaxConcurrent())
	var handlers sync.WaitGroup
	defer handlers.Wait()
	for {
		select {
		case <-consumerCtx.Done():
			return
		case delivery, ok := <-deliveries:
			if !ok {
				if consumerCtx.Err() == nil {
					reportFailure(current.failure, fmt.Errorf("rabbitmq consumer delivery stream closed"))
				}
				return
			}
			select {
			case slots <- struct{}{}:
			case <-consumerCtx.Done():
				return
			}
			handlers.Add(1)
			go func() {
				defer handlers.Done()
				defer func() { <-slots }()
				p.handleDelivery(consumerCtx, consumer, delivery)
			}()
		}
	}
}

func (p *provider) handleDelivery(ctx context.Context, consumer messagingapp.Consumer, delivery amqp.Delivery) {
	incoming, err := decodeDelivery(consumer.Route, delivery)
	if err != nil {
		_ = delivery.Reject(false)
		return
	}
	disposition := consumer.Handle(ctx, incoming)
	switch disposition {
	case messagingapp.DispositionAck:
		err = delivery.Ack(false)
	case messagingapp.DispositionRetryCounted:
		err = delivery.Reject(true)
	case messagingapp.DispositionDeferUncounted:
		err = delivery.Nack(false, true)
	case messagingapp.DispositionDeadLetter:
		err = delivery.Reject(false)
	default:
		err = delivery.Nack(false, true)
	}
	if err != nil && p.ctx.Err() == nil {
		p.lastError.Store(errorType(err))
	}
}

func decodeDelivery(route messagingapp.Route, delivery amqp.Delivery) (messagingapp.Incoming, error) {
	if delivery.ContentType != route.ContentType || delivery.Type != route.Contract.String() || len(delivery.Body) > route.MaxPayloadBytes {
		return messagingapp.Incoming{}, pkgmessaging.ErrContractMismatch
	}
	contractID, ok := headerString(delivery.Headers, headerContractID)
	if !ok || pkgmessaging.ContractID(contractID) != route.Contract.ID() || headerUint(delivery.Headers, headerContractVer) != uint64(route.Contract.Version()) {
		return messagingapp.Incoming{}, pkgmessaging.ErrContractMismatch
	}
	message, err := pkgmessaging.NewMessage(pkgmessaging.MessageSpec{
		ID: pkgmessaging.MessageID(delivery.MessageId), Contract: route.Contract, OccurredAt: delivery.Timestamp,
		OrderingKey: headerStringOrEmpty(delivery.Headers, headerOrderingKey), CorrelationID: delivery.CorrelationId,
		CausationID: headerStringOrEmpty(delivery.Headers, headerCausationID), Payload: delivery.Body,
	})
	if err != nil {
		return messagingapp.Incoming{}, err
	}
	return messagingapp.Incoming{
		Message: message, DeliveryCount: headerUint(delivery.Headers, headerDeliveryCount), Redelivered: delivery.Redelivered,
		TraceID: headerStringOrEmpty(delivery.Headers, headerTraceID),
	}, nil
}

func verifyTopology(channel *amqp.Channel, route messagingapp.Route) error {
	if route.Exchange != "" {
		if err := channel.ExchangeDeclarePassive(route.Exchange, route.ExchangeType, true, false, false, false, nil); err != nil {
			return fmt.Errorf("verify RabbitMQ exchange: %w", err)
		}
	}
	if _, err := channel.QueueDeclarePassive(route.Queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("verify RabbitMQ queue: %w", err)
	}
	return nil
}

func verifyConsumerTopologies(connection *amqp.Connection, consumers []messagingapp.Consumer) error {
	verified := make(map[pkgmessaging.RouteID]struct{}, len(consumers))
	for _, consumer := range consumers {
		if _, exists := verified[consumer.Route.ID]; exists {
			continue
		}
		channel, err := connection.Channel()
		if err != nil {
			return fmt.Errorf("open RabbitMQ topology channel: %w", err)
		}
		if err := verifyTopology(channel, consumer.Route); err != nil {
			return errors.Join(err, channel.Close())
		}
		if err := channel.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) {
			return fmt.Errorf("close RabbitMQ topology channel: %w", err)
		}
		verified[consumer.Route.ID] = struct{}{}
	}
	return nil
}

func (p *provider) clear(current *session) {
	p.mu.Lock()
	if p.current == current {
		p.current = nil
	}
	p.mu.Unlock()
}

func (p *provider) setState(state pkgmessaging.ProviderState, err error) {
	previous := p.currentState()
	p.state.Store(state)
	if err == nil {
		p.lastError.Store("")
	} else {
		p.lastError.Store(errorType(err))
	}
	if previous == state {
		return
	}
	fields := []pkglogger.Field{
		pkglogger.String("owner", "messaging-provider"), pkglogger.String("provider", p.name),
		pkglogger.String("driver", string(messagingapp.DriverRabbitMQ)), pkglogger.String("state", string(state)),
	}
	if err != nil {
		fields = append(fields, pkglogger.String("error_type", errorType(err)))
		p.dependencies.Logger.Warn("messaging provider state changed", fields...)
		return
	}
	p.dependencies.Logger.Info("messaging provider state changed", fields...)
}

func (p *provider) currentState() pkgmessaging.ProviderState {
	state, _ := p.state.Load().(pkgmessaging.ProviderState)
	return state
}

func loadTLS(config messagingapp.RabbitMQTLSConfig) (*tls.Config, error) {
	if !config.Enabled {
		return nil, nil
	}
	resolved := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: strings.TrimSpace(config.ServerName)}
	if config.CAFile != "" {
		pem, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read RabbitMQ CA file: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("load system CA pool: %w", err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("RabbitMQ CA file contains no certificate")
		}
		resolved.RootCAs = pool
	}
	if config.CertificateFile != "" {
		certificate, err := tls.LoadX509KeyPair(config.CertificateFile, config.PrivateKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load RabbitMQ client certificate: %w", err)
		}
		resolved.Certificates = []tls.Certificate{certificate}
	}
	return resolved, nil
}

func recoveryDelay(attempt int, config messagingapp.RecoveryConfig) time.Duration {
	delay := config.InitialBackoff
	for index := 1; index < attempt && delay < config.MaxBackoff/2; index++ {
		delay *= 2
	}
	if delay > config.MaxBackoff {
		delay = config.MaxBackoff
	}
	// 使用有界确定性抖动，避免多个实例同频重连且不引入全局随机源。
	jitter := time.Duration((attempt%5)-2) * delay / 20
	return delay + jitter
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func boundedHeader(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > maxHeaderValueLength {
		return value[:maxHeaderValueLength]
	}
	return value
}

func headerString(headers amqp.Table, key string) (string, bool) {
	value, exists := headers[key]
	if !exists {
		return "", false
	}
	resolved, ok := value.(string)
	return resolved, ok
}

func headerStringOrEmpty(headers amqp.Table, key string) string {
	value, _ := headerString(headers, key)
	return value
}

func headerUint(headers amqp.Table, key string) uint64 {
	value, exists := headers[key]
	if !exists {
		return 0
	}
	switch resolved := value.(type) {
	case int8:
		return uint64(max(resolved, 0))
	case int16:
		return uint64(max(resolved, 0))
	case int32:
		return uint64(max(resolved, 0))
	case int64:
		return uint64(max(resolved, 0))
	case uint8:
		return uint64(resolved)
	case uint16:
		return uint64(resolved)
	case uint32:
		return uint64(resolved)
	case uint64:
		return resolved
	default:
		return 0
	}
}

func reportFailure(target chan<- error, err error) {
	select {
	case target <- err:
	default:
	}
}

func closeConnection(connection *amqp.Connection) error {
	if connection == nil {
		return nil
	}
	err := connection.Close()
	if errors.Is(err, amqp.ErrClosed) {
		return nil
	}
	return err
}

func errorType(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "context_deadline_exceeded"
	}
	var amqpErr *amqp.Error
	if errors.As(err, &amqpErr) {
		return "amqp_" + strconv.Itoa(amqpErr.Code)
	}
	return reflect.TypeOf(err).String()
}

func deterministicBrokerError(err error) bool {
	var amqpErr *amqp.Error
	if !errors.As(err, &amqpErr) {
		return false
	}
	switch amqpErr.Code {
	case 403, 404, 405, 406, 530:
		return true
	default:
		return false
	}
}

var _ messagingapp.Factory = factory{}
var _ messagingapp.Provider = (*provider)(nil)
