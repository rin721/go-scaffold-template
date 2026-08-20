package messaging

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgmessaging "github.com/rin721/go-scaffold-template/pkg/messaging"
)

const (
	ID         app.ID = "messaging"
	ConfigPath        = "messaging"
)

const (
	defaultPublishConfirmTimeout = 5 * time.Second
	defaultHandoffTimeout        = 30 * time.Second
	defaultShutdownTimeout       = 30 * time.Second
	defaultConnectTimeout        = 3 * time.Second
	defaultInitialBackoff        = 500 * time.Millisecond
	defaultMaxBackoff            = 30 * time.Second
	defaultHeartbeat             = 10 * time.Second
	maxOperationTimeout          = 10 * time.Minute
	maxRecoveryBackoff           = 10 * time.Minute
	maxConfiguredEntries         = 1_000
	maxPhysicalNameLength        = 255
)

// Driver 标识由 composition 显式提供 Factory 的消息中间件类型。
type Driver string

const (
	DriverRabbitMQ Driver = "rabbitmq"
	DriverFake     Driver = "fake"
)

// Config 是 Messaging App 的强类型配置边界。
type Config struct {
	Enabled               bool                      `mapstructure:"enabled"`
	PublishConfirmTimeout time.Duration             `mapstructure:"publishConfirmTimeout"`
	HandoffTimeout        time.Duration             `mapstructure:"handoffTimeout"`
	ShutdownTimeout       time.Duration             `mapstructure:"shutdownTimeout"`
	Recovery              RecoveryConfig            `mapstructure:"recovery"`
	Providers             map[string]ProviderConfig `mapstructure:"providers"`
	Routes                map[string]RouteConfig    `mapstructure:"routes"`
}

// RecoveryConfig 控制 Provider 连接恢复的超时和有界退避。
type RecoveryConfig struct {
	ConnectTimeout time.Duration `mapstructure:"connectTimeout"`
	InitialBackoff time.Duration `mapstructure:"initialBackoff"`
	MaxBackoff     time.Duration `mapstructure:"maxBackoff"`
}

// ProviderConfig 是命名 Provider 的项目自有配置，不暴露第三方 Client 类型。
type ProviderConfig struct {
	Driver   Driver         `mapstructure:"driver"`
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
}

// RabbitMQConfig 控制一个 RabbitMQ Provider 的连接与 TLS 边界。
type RabbitMQConfig struct {
	URI       string            `mapstructure:"uri"`
	Heartbeat time.Duration     `mapstructure:"heartbeat"`
	TLS       RabbitMQTLSConfig `mapstructure:"tls"`
}

// RabbitMQTLSConfig 只保存 TLS 文件引用和验证要求，配置及日志不得展开凭据内容。
type RabbitMQTLSConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	ServerName         string `mapstructure:"serverName"`
	CAFile             string `mapstructure:"caFile"`
	CertificateFile    string `mapstructure:"certificateFile"`
	PrivateKeyFile     string `mapstructure:"privateKeyFile"`
	InsecureSkipVerify bool   `mapstructure:"insecureSkipVerify"`
}

// RouteConfig 把业务 Route ID 映射到一个 Provider 和物理 topology。
type RouteConfig struct {
	Provider             string                  `mapstructure:"provider"`
	Exchange             string                  `mapstructure:"exchange"`
	ExchangeType         string                  `mapstructure:"exchangeType"`
	RoutingKey           string                  `mapstructure:"routingKey"`
	Queue                string                  `mapstructure:"queue"`
	QueueType            string                  `mapstructure:"queueType"`
	Importance           pkgmessaging.Importance `mapstructure:"importance"`
	Reliable             bool                    `mapstructure:"reliable"`
	DeliveryLimit        uint64                  `mapstructure:"deliveryLimit"`
	DelayedRetryMin      time.Duration           `mapstructure:"delayedRetryMin"`
	DelayedRetryMax      time.Duration           `mapstructure:"delayedRetryMax"`
	DeadLetterExchange   string                  `mapstructure:"deadLetterExchange"`
	DeadLetterRoutingKey string                  `mapstructure:"deadLetterRoutingKey"`
	AtLeastOnceDLX       bool                    `mapstructure:"atLeastOnceDeadLetter"`
}

func decode(snapshot config.Snapshot) (Config, error) {
	resolved := defaultConfig()
	if err := snapshot.DecodeSection(ConfigPath, &resolved); err != nil {
		return Config{}, fmt.Errorf("decode messaging configuration: %w", err)
	}
	return normalizeConfig(resolved)
}

// Decode 解码 Messaging 配置，供 composition 和运维验证器复用同一校验语义。
func Decode(snapshot config.Snapshot) (Config, error) { return decode(snapshot) }

// Configuration 返回 messaging 配置节的唯一 authority。
func Configuration() config.Binding {
	return config.Binding{
		CapabilityID: string(ID), ConfigPath: ConfigPath, Contract: defaults{},
		Validate: func(snapshot config.Snapshot) error { _, err := decode(snapshot); return err },
	}
}

func defaultConfig() Config {
	return Config{
		PublishConfirmTimeout: defaultPublishConfirmTimeout,
		HandoffTimeout:        defaultHandoffTimeout, ShutdownTimeout: defaultShutdownTimeout,
		Recovery: RecoveryConfig{
			ConnectTimeout: defaultConnectTimeout, InitialBackoff: defaultInitialBackoff, MaxBackoff: defaultMaxBackoff,
		},
		Providers: map[string]ProviderConfig{}, Routes: map[string]RouteConfig{},
	}
}

func normalizeConfig(value Config) (Config, error) {
	if value.PublishConfirmTimeout <= 0 || value.HandoffTimeout <= 0 || value.ShutdownTimeout <= 0 ||
		value.PublishConfirmTimeout > maxOperationTimeout || value.HandoffTimeout > maxOperationTimeout || value.ShutdownTimeout > maxOperationTimeout {
		return Config{}, fmt.Errorf("messaging operation timeouts must be positive and bounded")
	}
	if value.Recovery.ConnectTimeout <= 0 || value.Recovery.InitialBackoff <= 0 || value.Recovery.MaxBackoff < value.Recovery.InitialBackoff ||
		value.Recovery.ConnectTimeout > maxOperationTimeout || value.Recovery.MaxBackoff > maxRecoveryBackoff {
		return Config{}, fmt.Errorf("messaging recovery timeouts are invalid")
	}
	if value.Providers == nil {
		value.Providers = map[string]ProviderConfig{}
	}
	if value.Routes == nil {
		value.Routes = map[string]RouteConfig{}
	}
	if len(value.Providers) > maxConfiguredEntries || len(value.Routes) > maxConfiguredEntries {
		return Config{}, fmt.Errorf("messaging provider or route count exceeds supported limit")
	}
	if !value.Enabled {
		if len(value.Providers) != 0 || len(value.Routes) != 0 {
			return Config{}, fmt.Errorf("disabled messaging must not retain active provider or route configuration")
		}
		return value, nil
	}
	if len(value.Providers) == 0 || len(value.Routes) == 0 {
		return Config{}, fmt.Errorf("enabled messaging requires providers and routes")
	}
	for name, provider := range value.Providers {
		if err := validateConfigName("provider", name); err != nil {
			return Config{}, err
		}
		switch provider.Driver {
		case DriverRabbitMQ:
			provider.RabbitMQ.URI = strings.TrimSpace(provider.RabbitMQ.URI)
			if provider.RabbitMQ.URI == "" {
				return Config{}, fmt.Errorf("messaging provider %q RabbitMQ URI is required", name)
			}
			if provider.RabbitMQ.Heartbeat <= 0 {
				provider.RabbitMQ.Heartbeat = defaultHeartbeat
			}
			if provider.RabbitMQ.Heartbeat > maxOperationTimeout {
				return Config{}, fmt.Errorf("messaging provider %q heartbeat exceeds supported limit", name)
			}
			if provider.RabbitMQ.TLS.InsecureSkipVerify {
				return Config{}, fmt.Errorf("messaging provider %q cannot disable TLS certificate verification", name)
			}
			cert, key := strings.TrimSpace(provider.RabbitMQ.TLS.CertificateFile), strings.TrimSpace(provider.RabbitMQ.TLS.PrivateKeyFile)
			if (cert == "") != (key == "") {
				return Config{}, fmt.Errorf("messaging provider %q TLS certificate and private key must be configured together", name)
			}
			provider.RabbitMQ.TLS.CertificateFile, provider.RabbitMQ.TLS.PrivateKeyFile = cert, key
			value.Providers[name] = provider
		case DriverFake:
		default:
			return Config{}, fmt.Errorf("messaging provider %q driver %q is unsupported", name, provider.Driver)
		}
	}
	for id, route := range value.Routes {
		if err := validateConfigName("route", id); err != nil {
			return Config{}, err
		}
		route.Provider = strings.TrimSpace(route.Provider)
		if _, exists := value.Providers[route.Provider]; !exists {
			return Config{}, fmt.Errorf("messaging route %q references unknown provider %q", id, route.Provider)
		}
		if err := validatePhysicalNames(route); err != nil {
			return Config{}, fmt.Errorf("messaging route %q: %w", id, err)
		}
		if route.Exchange != "" && route.ExchangeType == "" {
			route.ExchangeType = "topic"
		}
		if route.Importance != pkgmessaging.ImportanceRequired && route.Importance != pkgmessaging.ImportanceOptional {
			return Config{}, fmt.Errorf("messaging route %q importance is invalid", id)
		}
		if route.Reliable {
			if route.QueueType != "quorum" || route.DeliveryLimit <= 0 || route.DelayedRetryMin <= 0 ||
				route.DelayedRetryMax < route.DelayedRetryMin || route.DeadLetterExchange == "" || !route.AtLeastOnceDLX {
				return Config{}, fmt.Errorf("messaging reliable route %q lacks quorum delayed-retry or at-least-once DLX requirements", id)
			}
		}
		value.Routes[id] = route
	}
	return value, nil
}

func validateConfigName(kind, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed != value || len(value) > maxPhysicalNameLength || strings.ContainsAny(value, " \t\r\n/:@") {
		return fmt.Errorf("messaging %s name %q is invalid", kind, value)
	}
	return nil
}

func validatePhysicalNames(route RouteConfig) error {
	for name, value := range map[string]string{
		"exchange": route.Exchange, "routing key": route.RoutingKey, "queue": route.Queue,
		"dead-letter exchange": route.DeadLetterExchange, "dead-letter routing key": route.DeadLetterRoutingKey,
	} {
		if len(value) > maxPhysicalNameLength || strings.ContainsAny(value, "\r\n\x00") {
			return fmt.Errorf("%s is invalid", name)
		}
	}
	if strings.TrimSpace(route.RoutingKey) == "" {
		return fmt.Errorf("routing key is required")
	}
	return nil
}

type defaults struct{}

func (defaults) Defaults(ctx context.Context) (config.Object, config.Control, error) {
	if ctx == nil {
		return nil, config.Continue, app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, config.Continue, err
	}
	value := defaultConfig()
	return config.Object{
		config.FieldOf("enabled", config.Bool(value.Enabled)),
		config.FieldOf("publishConfirmTimeout", config.Duration(value.PublishConfirmTimeout)),
		config.FieldOf("handoffTimeout", config.Duration(value.HandoffTimeout)),
		config.FieldOf("shutdownTimeout", config.Duration(value.ShutdownTimeout)),
		config.FieldOf("recovery", config.ObjectValue(config.Object{
			config.FieldOf("connectTimeout", config.Duration(value.Recovery.ConnectTimeout)),
			config.FieldOf("initialBackoff", config.Duration(value.Recovery.InitialBackoff)),
			config.FieldOf("maxBackoff", config.Duration(value.Recovery.MaxBackoff)),
		})),
		config.FieldOf("providers", config.ObjectValue(config.Object{})),
		config.FieldOf("routes", config.ObjectValue(config.Object{})),
	}, config.Continue, nil
}

// SortedProviderNames 返回确定性 Provider 顺序。
func (c Config) SortedProviderNames() []string {
	names := make([]string, 0, len(c.Providers))
	for name := range c.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Route 返回逻辑 Route 的配置。
func (c Config) Route(id pkgmessaging.RouteID) (RouteConfig, bool) {
	route, exists := c.Routes[string(id)]
	return route, exists
}

var _ config.DefaultContract = defaults{}
