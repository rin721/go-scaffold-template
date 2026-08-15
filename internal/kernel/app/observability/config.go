package observability

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

const ConfigPath = "observability"

// Config 是 Telemetry App 的 typed 配置契约。
type Config struct {
	ServiceName string  `mapstructure:"serviceName"`
	Tracing     Tracing `mapstructure:"tracing"`
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

// DefaultConfig 返回默认关闭外部 exporter 的安全配置。
func DefaultConfig() Config {
	return Config{ServiceName: "go-scaffold-template", Tracing: Tracing{
		SampleRatio: 0.1, QueueSize: 2048, BatchSize: 256,
		BatchTimeout: 5 * time.Second, ExportTimeout: 3 * time.Second, ShutdownTimeout: 5 * time.Second,
	}}
}

func decode(snapshot config.Snapshot) (Config, error) {
	resolved := DefaultConfig()
	if err := snapshot.DecodeSection(ConfigPath, &resolved); err != nil {
		return Config{}, fmt.Errorf("decode observability configuration: %w", err)
	}
	if err := validateConfig(resolved); err != nil {
		return Config{}, err
	}
	return resolved, nil
}

// Configuration 返回 Observability section 的唯一配置 authority。
func Configuration() config.Binding {
	return config.Binding{
		CapabilityID: string(TelemetryID), ConfigPath: ConfigPath, Contract: defaults{},
		Validate: func(snapshot config.Snapshot) error { _, err := decode(snapshot); return err },
	}
}

func validateConfig(value Config) error {
	if strings.TrimSpace(value.ServiceName) == "" {
		return fmt.Errorf("observability service name is required")
	}
	tracing := value.Tracing
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

type defaults struct{}

func (defaults) Defaults(ctx context.Context) (config.Object, config.Control, error) {
	if ctx == nil {
		return nil, config.Continue, fmt.Errorf("observability defaults context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, config.Continue, err
	}
	value := DefaultConfig()
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
		panic("invalid compile-time observability default: " + strconv.Quote(fmt.Sprint(value)))
	}
	return resolved
}

var _ config.DefaultContract = defaults{}
