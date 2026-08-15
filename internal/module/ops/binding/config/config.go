// Package configbinding 绑定 Ops module 的 management 与 observability 配置。
package configbinding

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
)

const (
	managementCapabilityID    = "module.ops.management"
	observabilityCapabilityID = "module.ops.observability"
	managementPath            = "management"
	observabilityPath         = "observability"
)

// AccessMode 控制 metrics 是否关闭、公开或需要 management scope。
type AccessMode string

const (
	AccessDisabled  AccessMode = "disabled"
	AccessPublic    AccessMode = "public"
	AccessProtected AccessMode = "protected"
)

// Management 是独立管理 listener 的安全预算。
type Management struct {
	Addr                string        `mapstructure:"addr"`
	ReadHeaderTimeout   time.Duration `mapstructure:"readHeaderTimeout"`
	ReadTimeout         time.Duration `mapstructure:"readTimeout"`
	WriteTimeout        time.Duration `mapstructure:"writeTimeout"`
	IdleTimeout         time.Duration `mapstructure:"idleTimeout"`
	RequestTimeout      time.Duration `mapstructure:"requestTimeout"`
	MaxHeaderBytes      int           `mapstructure:"maxHeaderBytes"`
	MaxRequestBodyBytes int64         `mapstructure:"maxRequestBodyBytes"`
	MaxInFlight         int           `mapstructure:"maxInFlight"`
	MetricsAccess       AccessMode    `mapstructure:"metricsAccess"`
}

// Tracing 配置 generation-owned OTLP/HTTP trace exporter。
type Tracing struct {
	Enabled         bool          `mapstructure:"enabled"`
	Endpoint        string        `mapstructure:"endpoint"`
	Insecure        bool          `mapstructure:"insecure"`
	SampleRatio     float64       `mapstructure:"sampleRatio"`
	QueueSize       int           `mapstructure:"queueSize"`
	BatchSize       int           `mapstructure:"batchSize"`
	BatchTimeout    time.Duration `mapstructure:"batchTimeout"`
	ExportTimeout   time.Duration `mapstructure:"exportTimeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdownTimeout"`
}

// Observability 是 Ops module 的观测配置。
type Observability struct {
	ServiceName string  `mapstructure:"serviceName"`
	Tracing     Tracing `mapstructure:"tracing"`
}

// Config 聚合 Ops module 的两个独立配置段。
type Config struct {
	Management    Management
	Observability Observability
}

// Bindings 返回两个 section 的默认值与候选校验 authority。
func Bindings() []config.Binding {
	return []config.Binding{
		{CapabilityID: managementCapabilityID, ConfigPath: managementPath, Contract: managementDefaults{}, Validate: func(snapshot config.Snapshot) error { _, err := Decode(snapshot); return err }},
		{CapabilityID: observabilityCapabilityID, ConfigPath: observabilityPath, Contract: observabilityDefaults{}, Validate: func(snapshot config.Snapshot) error { _, err := Decode(snapshot); return err }},
	}
}

// Decode 从同一 Snapshot 解码并校验管理面与观测配置。
func Decode(snapshot config.Snapshot) (Config, error) {
	resolved := Default()
	if err := snapshot.DecodeSection(managementPath, &resolved.Management); err != nil {
		return Config{}, fmt.Errorf("decode management configuration: %w", err)
	}
	if err := snapshot.DecodeSection(observabilityPath, &resolved.Observability); err != nil {
		return Config{}, fmt.Errorf("decode observability configuration: %w", err)
	}
	if err := validate(resolved); err != nil {
		return Config{}, err
	}
	return resolved, nil
}

// Default 返回只监听 loopback、默认关闭外部 trace exporter 的安全配置。
func Default() Config {
	return Config{
		Management: Management{
			Addr: "127.0.0.1:9090", ReadHeaderTimeout: 2 * time.Second,
			ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, IdleTimeout: 15 * time.Second,
			RequestTimeout: 2 * time.Second, MaxHeaderBytes: 16 << 10,
			MaxRequestBodyBytes: 4 << 10, MaxInFlight: 16, MetricsAccess: AccessPublic,
		},
		Observability: Observability{ServiceName: "go-scaffold-template", Tracing: Tracing{
			SampleRatio: 0.1, QueueSize: 2048, BatchSize: 256,
			BatchTimeout: 5 * time.Second, ExportTimeout: 3 * time.Second, ShutdownTimeout: 5 * time.Second,
		}},
	}
}

func validate(config Config) error {
	management := config.Management
	if strings.TrimSpace(management.Addr) == "" {
		return fmt.Errorf("management address is required")
	}
	if _, _, err := net.SplitHostPort(management.Addr); err != nil {
		return fmt.Errorf("management address is invalid: %w", err)
	}
	if management.ReadHeaderTimeout <= 0 || management.ReadTimeout <= 0 || management.WriteTimeout <= 0 || management.IdleTimeout <= 0 || management.RequestTimeout <= 0 {
		return fmt.Errorf("management timeouts must be positive")
	}
	if management.MaxHeaderBytes <= 0 || management.MaxRequestBodyBytes <= 0 || management.MaxInFlight <= 0 {
		return fmt.Errorf("management limits must be positive")
	}
	switch management.MetricsAccess {
	case AccessDisabled, AccessPublic, AccessProtected:
	default:
		return fmt.Errorf("management metrics access %q is unsupported", management.MetricsAccess)
	}
	observability := config.Observability
	if strings.TrimSpace(observability.ServiceName) == "" {
		return fmt.Errorf("observability service name is required")
	}
	tracing := observability.Tracing
	if tracing.SampleRatio < 0 || tracing.SampleRatio > 1 || tracing.QueueSize <= 0 || tracing.BatchSize <= 0 || tracing.BatchSize > tracing.QueueSize {
		return fmt.Errorf("observability trace sampling or queue limits are invalid")
	}
	if tracing.BatchTimeout <= 0 || tracing.ExportTimeout <= 0 || tracing.ShutdownTimeout <= 0 {
		return fmt.Errorf("observability trace budgets must be positive")
	}
	if !tracing.Enabled {
		return nil
	}
	parsed, err := url.Parse(tracing.Endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("observability trace endpoint is invalid")
	}
	if tracing.Insecure {
		if parsed.Scheme != "http" || !loopbackHost(parsed.Hostname()) {
			return fmt.Errorf("insecure trace endpoint requires HTTP loopback")
		}
	} else if parsed.Scheme != "https" {
		return fmt.Errorf("trace endpoint requires HTTPS")
	}
	return nil
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ManagementIsLoopback 供 composition 执行 Auth profile 与管理面暴露的跨模块安全校验。
func ManagementIsLoopback(address string) bool {
	host, _, err := net.SplitHostPort(address)
	return err == nil && loopbackHost(host)
}

type managementDefaults struct{}

func (managementDefaults) Defaults(ctx context.Context) (config.Object, config.Control, error) {
	if ctx == nil {
		return nil, config.Continue, fmt.Errorf("management defaults context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, config.Continue, err
	}
	value := Default().Management
	return config.Object{
		config.FieldOf("addr", config.String(value.Addr)),
		config.FieldOf("readHeaderTimeout", config.Duration(value.ReadHeaderTimeout)),
		config.FieldOf("readTimeout", config.Duration(value.ReadTimeout)),
		config.FieldOf("writeTimeout", config.Duration(value.WriteTimeout)),
		config.FieldOf("idleTimeout", config.Duration(value.IdleTimeout)),
		config.FieldOf("requestTimeout", config.Duration(value.RequestTimeout)),
		config.FieldOf("maxHeaderBytes", number(value.MaxHeaderBytes)),
		config.FieldOf("maxRequestBodyBytes", number(value.MaxRequestBodyBytes)),
		config.FieldOf("maxInFlight", number(value.MaxInFlight)),
		config.FieldOf("metricsAccess", config.String(string(value.MetricsAccess))),
	}, config.Continue, nil
}

type observabilityDefaults struct{}

func (observabilityDefaults) Defaults(ctx context.Context) (config.Object, config.Control, error) {
	if ctx == nil {
		return nil, config.Continue, fmt.Errorf("observability defaults context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, config.Continue, err
	}
	value := Default().Observability
	return config.Object{
		config.FieldOf("serviceName", config.String(value.ServiceName)),
		config.FieldOf("tracing", config.ObjectValue(config.Object{
			config.FieldOf("enabled", config.Bool(value.Tracing.Enabled)),
			config.FieldOf("endpoint", config.String(value.Tracing.Endpoint)),
			config.FieldOf("insecure", config.Bool(value.Tracing.Insecure)),
			config.FieldOf("sampleRatio", number(value.Tracing.SampleRatio)),
			config.FieldOf("queueSize", number(value.Tracing.QueueSize)),
			config.FieldOf("batchSize", number(value.Tracing.BatchSize)),
			config.FieldOf("batchTimeout", config.Duration(value.Tracing.BatchTimeout)),
			config.FieldOf("exportTimeout", config.Duration(value.Tracing.ExportTimeout)),
			config.FieldOf("shutdownTimeout", config.Duration(value.Tracing.ShutdownTimeout)),
		})),
	}, config.Continue, nil
}

func number(value any) config.Value {
	resolved, err := config.Number(fmt.Sprint(value))
	if err != nil {
		panic("invalid compile-time ops default: " + strconv.Quote(fmt.Sprint(value)))
	}
	return resolved
}

var _ config.DefaultContract = managementDefaults{}
var _ config.DefaultContract = observabilityDefaults{}
