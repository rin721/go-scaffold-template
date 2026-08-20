package schedule

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgschedule "github.com/rin721/go-scaffold-template/pkg/schedule"
)

const (
	ID         app.ID = "scheduler"
	ConfigPath        = "scheduler"
)

const (
	defaultTimezone            = "UTC"
	defaultMaxConcurrency      = 32
	defaultShutdownTimeout     = 30 * time.Second
	defaultOccurrenceRetention = 24 * time.Hour
	defaultCoordinationNS      = "go-scaffold-template:scheduler"
	defaultLeaseTTL            = 30 * time.Second
	defaultRenewInterval       = 10 * time.Second
	defaultAcquireTimeout      = 2 * time.Second
	defaultRetryMin            = 500 * time.Millisecond
	defaultRetryMax            = 10 * time.Second
)

const (
	maxConfiguredConcurrency = 10_000
	maxShutdownTimeout       = 10 * time.Minute
	maxOccurrenceRetention   = 365 * 24 * time.Hour
	maxCoordinationNamespace = 256
	maxLeaseTTL              = 24 * time.Hour
	maxAcquireTimeout        = time.Minute
	maxCoordinationRetry     = time.Hour
)

// Config 是 Scheduler App 的强类型安全边界。
type Config struct {
	Enabled             bool                  `mapstructure:"enabled"`
	Timezone            string                `mapstructure:"timezone"`
	MaxConcurrency      int                   `mapstructure:"maxConcurrency"`
	ShutdownTimeout     time.Duration         `mapstructure:"shutdownTimeout"`
	OccurrenceRetention time.Duration         `mapstructure:"occurrenceRetention"`
	Coordination        CoordinationConfig    `mapstructure:"coordination"`
	Tasks               map[string]TaskConfig `mapstructure:"tasks"`
}

// CoordinationConfig 控制租约与自动恢复的全局安全窗口。
type CoordinationConfig struct {
	Namespace      string        `mapstructure:"namespace"`
	LeaseTTL       time.Duration `mapstructure:"leaseTTL"`
	RenewInterval  time.Duration `mapstructure:"renewInterval"`
	AcquireTimeout time.Duration `mapstructure:"acquireTimeout"`
	RetryMin       time.Duration `mapstructure:"retryMin"`
	RetryMax       time.Duration `mapstructure:"retryMax"`
}

// TaskConfig 只允许按稳定 Task ID 覆盖运维属性，不注入任务函数或另一份触发定义。
type TaskConfig struct {
	Enabled           *bool                         `mapstructure:"enabled"`
	UnavailablePolicy pkgschedule.UnavailablePolicy `mapstructure:"unavailablePolicy"`
}

func decode(snapshot config.Snapshot) (Config, error) {
	resolved := defaultConfig()
	if err := snapshot.DecodeSection(ConfigPath, &resolved); err != nil {
		return Config{}, fmt.Errorf("decode scheduler configuration: %w", err)
	}
	return normalizeConfig(resolved)
}

// Configuration 返回 scheduler 配置节的唯一 authority。
func Configuration() config.Binding {
	return config.Binding{
		CapabilityID: string(ID), ConfigPath: ConfigPath, Contract: defaults{},
		Validate: func(snapshot config.Snapshot) error { _, err := decode(snapshot); return err },
	}
}

func normalizeConfig(value Config) (Config, error) {
	defaults := defaultConfig()
	value.Timezone = strings.TrimSpace(value.Timezone)
	value.Coordination.Namespace = strings.Trim(strings.TrimSpace(value.Coordination.Namespace), ":")
	if value.Timezone == "" {
		value.Timezone = defaults.Timezone
	}
	if _, err := time.LoadLocation(value.Timezone); err != nil {
		return Config{}, fmt.Errorf("scheduler timezone is invalid: %w", err)
	}
	if value.MaxConcurrency <= 0 {
		return Config{}, fmt.Errorf("scheduler max concurrency must be positive")
	}
	if value.MaxConcurrency > maxConfiguredConcurrency {
		return Config{}, fmt.Errorf("scheduler max concurrency exceeds the supported limit")
	}
	if value.ShutdownTimeout <= 0 || value.OccurrenceRetention <= 0 {
		return Config{}, fmt.Errorf("scheduler shutdown and occurrence retention must be positive")
	}
	if value.ShutdownTimeout > maxShutdownTimeout || value.OccurrenceRetention > maxOccurrenceRetention {
		return Config{}, fmt.Errorf("scheduler shutdown or occurrence retention exceeds the supported limit")
	}
	coordination := value.Coordination
	if coordination.Namespace == "" || len(coordination.Namespace) > maxCoordinationNamespace || strings.ContainsAny(coordination.Namespace, " \t\r\n") {
		return Config{}, fmt.Errorf("scheduler coordination namespace is invalid")
	}
	if coordination.LeaseTTL <= 0 || coordination.RenewInterval <= 0 || coordination.AcquireTimeout <= 0 ||
		coordination.RetryMin <= 0 || coordination.RetryMax <= 0 {
		return Config{}, fmt.Errorf("scheduler coordination durations must be positive")
	}
	if coordination.LeaseTTL > maxLeaseTTL || coordination.AcquireTimeout > maxAcquireTimeout ||
		coordination.RetryMin > maxCoordinationRetry || coordination.RetryMax > maxCoordinationRetry {
		return Config{}, fmt.Errorf("scheduler coordination duration exceeds the supported limit")
	}
	if coordination.RenewInterval*3 > coordination.LeaseTTL {
		return Config{}, fmt.Errorf("scheduler renew interval must leave at least two safety windows before lease expiry")
	}
	if coordination.AcquireTimeout >= coordination.LeaseTTL {
		return Config{}, fmt.Errorf("scheduler acquire timeout must be shorter than lease ttl")
	}
	if coordination.RetryMax < coordination.RetryMin {
		return Config{}, fmt.Errorf("scheduler retry max must not be shorter than retry min")
	}
	for taskID, override := range value.Tasks {
		if strings.TrimSpace(taskID) == "" || taskID != strings.TrimSpace(taskID) {
			return Config{}, fmt.Errorf("scheduler task override id is invalid")
		}
		if override.UnavailablePolicy != "" {
			switch override.UnavailablePolicy {
			case pkgschedule.UnavailableSkip, pkgschedule.UnavailablePause, pkgschedule.UnavailableFail, pkgschedule.UnavailableLocal:
			default:
				return Config{}, fmt.Errorf("scheduler task %q unavailable policy is invalid", taskID)
			}
		}
	}
	return value, nil
}

func defaultConfig() Config {
	return Config{
		Timezone: defaultTimezone, MaxConcurrency: defaultMaxConcurrency,
		ShutdownTimeout: defaultShutdownTimeout, OccurrenceRetention: defaultOccurrenceRetention,
		Coordination: CoordinationConfig{
			Namespace: defaultCoordinationNS, LeaseTTL: defaultLeaseTTL,
			RenewInterval: defaultRenewInterval, AcquireTimeout: defaultAcquireTimeout,
			RetryMin: defaultRetryMin, RetryMax: defaultRetryMax,
		},
	}
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
	maxConcurrency, err := config.Number(strconv.Itoa(value.MaxConcurrency))
	if err != nil {
		return nil, config.Continue, err
	}
	fields := config.Object{
		config.FieldOf("enabled", config.Bool(value.Enabled)),
		config.FieldOf("timezone", config.String(value.Timezone)),
		config.FieldOf("maxConcurrency", maxConcurrency),
		config.FieldOf("shutdownTimeout", config.Duration(value.ShutdownTimeout)),
		config.FieldOf("occurrenceRetention", config.Duration(value.OccurrenceRetention)),
		config.FieldOf("coordination", config.ObjectValue(config.Object{
			config.FieldOf("namespace", config.String(value.Coordination.Namespace)),
			config.FieldOf("leaseTTL", config.Duration(value.Coordination.LeaseTTL)),
			config.FieldOf("renewInterval", config.Duration(value.Coordination.RenewInterval)),
			config.FieldOf("acquireTimeout", config.Duration(value.Coordination.AcquireTimeout)),
			config.FieldOf("retryMin", config.Duration(value.Coordination.RetryMin)),
			config.FieldOf("retryMax", config.Duration(value.Coordination.RetryMax)),
		})),
	}
	if len(value.Tasks) > 0 {
		ids := make([]string, 0, len(value.Tasks))
		for id := range value.Tasks {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		taskFields := make([]config.Field, 0, len(ids))
		for _, id := range ids {
			override := value.Tasks[id]
			values := make([]config.Field, 0, 2)
			if override.Enabled != nil {
				values = append(values, config.FieldOf("enabled", config.Bool(*override.Enabled)))
			}
			if override.UnavailablePolicy != "" {
				values = append(values, config.FieldOf("unavailablePolicy", config.String(string(override.UnavailablePolicy))))
			}
			taskFields = append(taskFields, config.FieldOf(id, config.ObjectValue(config.Object(values))))
		}
		fields = append(fields, config.FieldOf("tasks", config.ObjectValue(config.Object(taskFields))))
	}
	return fields, config.Continue, nil
}

var _ config.DefaultContract = defaults{}
