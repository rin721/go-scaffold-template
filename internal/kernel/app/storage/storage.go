// Package storage 定义由 Kernel 治理的对象 Storage App 组件。
package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgstorage "github.com/rin721/go-scaffold-template/pkg/storage"
)

const (
	ID         app.ID = "storage"
	ConfigPath        = "storage"
)

// Route 表示一次对象存储借用要访问的语义实例。
type Route string

const (
	RoutePrimary Route = "primary"
	RouteLocal   Route = "local"
	RouteObject  Route = "object"
)

const defaultLocalBasePath = ".data/storage"

// Config 是 Storage App 的 typed 配置契约。
type Config struct {
	Driver pkgstorage.Driver `mapstructure:"driver"`
	Local  LocalConfig       `mapstructure:"local"`
	S3     ObjectConfig      `mapstructure:"s3"`
	MinIO  ObjectConfig      `mapstructure:"minio"`
}

// LocalConfig 描述对象存储 local adapter 的配置。
type LocalConfig struct {
	BasePath  string `mapstructure:"basePath"`
	PublicURL string `mapstructure:"publicUrl"`
}

// ObjectConfig 描述 S3-compatible adapter 的项目配置。
type ObjectConfig struct {
	Provider        string `mapstructure:"provider"`
	Endpoint        string `mapstructure:"endpoint"`
	Region          string `mapstructure:"region"`
	Bucket          string `mapstructure:"bucket"`
	AccessKeyID     string `mapstructure:"accessKeyId"`
	SecretAccessKey string `mapstructure:"secretAccessKey"`
	PathStyle       bool   `mapstructure:"usePathStyle"`
	PublicBaseURL   string `mapstructure:"publicBaseUrl"`
}

// Client 是租约内可用且不含共享资源关闭权的对象存储能力。
type Client interface {
	Put(context.Context, string, []byte, pkgstorage.PutOptions) error
	Get(context.Context, string) ([]byte, pkgstorage.ObjectInfo, error)
	Delete(context.Context, string) error
	Exists(context.Context, string) (bool, error)
}

// Access 是调用方接收的稳定对象存储租约入口。
type Access interface {
	Use(context.Context, Route, func(Client) error) error
}

type access struct {
	delegate app.Lease[*pkgstorage.StorageManager]
}

// Definition 返回无安装副作用的 Storage 组件声明。
func Definition() (app.Definition[Access], error) {
	source, err := app.Configured(ConfigPath, decode, defaults{})
	if err != nil {
		return app.Definition[Access]{}, err
	}
	return app.ManagedConfigured(
		ID,
		source,
		app.FixedDependencies(struct{}{}),
		build,
		app.Leased(newAccess),
		app.KernelInstanceSwap,
		app.WithReady(ready),
		app.WithStop(stop),
	)
}

func newAccess(delegate app.Lease[*pkgstorage.StorageManager]) (Access, error) {
	if delegate == nil {
		return nil, fmt.Errorf("storage lease is nil")
	}
	return &access{delegate: delegate}, nil
}

func (a *access) Use(ctx context.Context, route Route, use func(Client) error) error {
	if ctx == nil {
		return pkgstorage.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if use == nil {
		return pkgstorage.ErrNilClientFunc
	}
	if !validRoute(route) {
		return fmt.Errorf("%w: %q", pkgstorage.ErrInvalidRoute, route)
	}
	return a.delegate.Use(ctx, func(manager *pkgstorage.StorageManager) error {
		if manager == nil {
			return pkgstorage.ErrClientUnavailable
		}
		if manager.Driver == pkgstorage.DriverDisabled {
			return pkgstorage.ErrDisabled
		}
		selected := selectClient(manager, route)
		if selected == nil {
			return pkgstorage.ErrClientUnavailable
		}
		return borrow(ctx, selected, use)
	})
}

func validRoute(route Route) bool {
	return route == RoutePrimary || route == RouteLocal || route == RouteObject
}

func selectClient(manager *pkgstorage.StorageManager, route Route) pkgstorage.StorageClient {
	switch route {
	case RoutePrimary:
		return manager.Primary()
	case RouteLocal:
		return manager.Local
	case RouteObject:
		return manager.Object
	default:
		return nil
	}
}

func build(ctx context.Context, cfg Config, _ struct{}) (*pkgstorage.StorageManager, error) {
	if ctx == nil {
		return nil, app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	packageConfig := cfg.packageConfig()
	manager, err := pkgstorage.NewManager(ctx, &packageConfig)
	if err != nil {
		return nil, fmt.Errorf("create storage manager: %w", err)
	}
	return manager, nil
}

func ready(ctx context.Context, manager *pkgstorage.StorageManager) error {
	if ctx == nil {
		return app.ErrNilContext
	}
	if manager == nil {
		return pkgstorage.ErrClientUnavailable
	}
	if manager.Driver == pkgstorage.DriverDisabled {
		return nil
	}
	var joined error
	if manager.Local != nil {
		if err := manager.Local.HealthCheck(ctx); err != nil {
			joined = errors.Join(joined, fmt.Errorf("verify local storage readiness: %w", err))
		}
	}
	if manager.Object != nil {
		if err := manager.Object.HealthCheck(ctx); err != nil {
			joined = errors.Join(joined, fmt.Errorf("verify object storage readiness: %w", err))
		}
	}
	return joined
}

func stop(_ context.Context, manager *pkgstorage.StorageManager) error {
	if manager == nil {
		return pkgstorage.ErrClientUnavailable
	}
	return manager.Close()
}

func decode(snapshot config.Snapshot) (Config, error) {
	cfg := defaultConfig()
	if err := snapshot.DecodeSection(ConfigPath, &cfg); err != nil {
		return Config{}, err
	}
	return normalizeConfig(cfg)
}

func normalizeConfig(cfg Config) (Config, error) {
	defaults := defaultConfig()
	cfg.Driver = pkgstorage.Driver(strings.ToLower(strings.TrimSpace(string(cfg.Driver))))
	cfg.Local.BasePath = strings.TrimSpace(cfg.Local.BasePath)
	cfg.Local.PublicURL = strings.TrimSpace(cfg.Local.PublicURL)
	cfg.S3 = normalizeObjectConfig(cfg.S3, defaults.S3)
	cfg.MinIO = normalizeObjectConfig(cfg.MinIO, defaults.MinIO)
	switch cfg.Driver {
	case pkgstorage.DriverDisabled:
	case pkgstorage.DriverLocal:
		if cfg.Local.BasePath == "" {
			return Config{}, fmt.Errorf("storage local base path is required")
		}
	case pkgstorage.DriverS3:
		if err := validateObjectConfig("s3", cfg.S3, false); err != nil {
			return Config{}, err
		}
	case pkgstorage.DriverMinIO:
		if err := validateObjectConfig("minio", cfg.MinIO, true); err != nil {
			return Config{}, err
		}
	case pkgstorage.DriverLocalS3:
		if cfg.Local.BasePath == "" {
			return Config{}, fmt.Errorf("storage local base path is required")
		}
		if err := validateObjectConfig("s3", cfg.S3, false); err != nil {
			return Config{}, err
		}
	case pkgstorage.DriverLocalMinIO:
		if cfg.Local.BasePath == "" {
			return Config{}, fmt.Errorf("storage local base path is required")
		}
		if err := validateObjectConfig("minio", cfg.MinIO, true); err != nil {
			return Config{}, err
		}
	default:
		return Config{}, fmt.Errorf("unsupported storage driver %q", cfg.Driver)
	}
	return cfg, nil
}

func normalizeObjectConfig(cfg ObjectConfig, defaults ObjectConfig) ObjectConfig {
	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.AccessKeyID = strings.TrimSpace(cfg.AccessKeyID)
	cfg.PublicBaseURL = strings.TrimSpace(cfg.PublicBaseURL)
	if cfg.Provider == "" {
		cfg.Provider = defaults.Provider
	}
	if cfg.Region == "" {
		cfg.Region = defaults.Region
	}
	return cfg
}

func validateObjectConfig(section string, cfg ObjectConfig, minio bool) error {
	if minio {
		if cfg.Provider != string(pkgstorage.ObjectProviderMinIO) {
			return fmt.Errorf("storage %s provider must be minio", section)
		}
	} else if cfg.Provider != string(pkgstorage.ObjectProviderS3) && cfg.Provider != string(pkgstorage.ObjectProviderR2) {
		return fmt.Errorf("storage %s provider must be s3 or r2", section)
	}
	if cfg.Endpoint == "" {
		return fmt.Errorf("storage %s endpoint is required", section)
	}
	if cfg.Bucket == "" {
		return fmt.Errorf("storage %s bucket is required", section)
	}
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return fmt.Errorf("storage %s credentials are required", section)
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
	values := defaultConfig()
	return config.Object{
		config.FieldOf("driver", config.String(string(values.Driver))),
		config.FieldOf("local", config.ObjectValue(config.Object{
			config.FieldOf("basePath", config.String(values.Local.BasePath)),
			config.FieldOf("publicUrl", config.String(values.Local.PublicURL)),
		})),
		config.FieldOf("s3", objectDefaults(values.S3)),
		config.FieldOf("minio", objectDefaults(values.MinIO)),
	}, config.Continue, nil
}

func objectDefaults(values ObjectConfig) config.Value {
	return config.ObjectValue(config.Object{
		config.FieldOf("provider", config.String(values.Provider)),
		config.FieldOf("endpoint", config.String(values.Endpoint)),
		config.FieldOf("region", config.String(values.Region)),
		config.FieldOf("bucket", config.String(values.Bucket)),
		config.FieldOf("accessKeyId", config.String(values.AccessKeyID)),
		config.FieldOf("secretAccessKey", config.String(values.SecretAccessKey)),
		config.FieldOf("usePathStyle", config.Bool(values.PathStyle)),
		config.FieldOf("publicBaseUrl", config.String(values.PublicBaseURL)),
	})
}

func defaultConfig() Config {
	return Config{
		Driver: pkgstorage.DriverLocal,
		Local:  LocalConfig{BasePath: defaultLocalBasePath},
		S3: ObjectConfig{
			Provider: string(pkgstorage.ObjectProviderS3),
			Region:   "us-east-1",
		},
		MinIO: ObjectConfig{
			Provider:  string(pkgstorage.ObjectProviderMinIO),
			Region:    "us-east-1",
			PathStyle: true,
		},
	}
}

func (c Config) packageConfig() pkgstorage.Config {
	return pkgstorage.Config{
		Driver: c.Driver,
		Local: pkgstorage.LocalConfig{
			BasePath:  c.Local.BasePath,
			PublicURL: c.Local.PublicURL,
		},
		S3:    c.S3.packageConfig(),
		MinIO: c.MinIO.packageConfig(),
	}
}

func (c ObjectConfig) packageConfig() pkgstorage.ObjectConfig {
	return pkgstorage.ObjectConfig{
		Provider:        c.Provider,
		Endpoint:        c.Endpoint,
		Region:          c.Region,
		Bucket:          c.Bucket,
		AccessKeyID:     c.AccessKeyID,
		SecretAccessKey: c.SecretAccessKey,
		PathStyle:       c.PathStyle,
		PublicBaseURL:   c.PublicBaseURL,
	}
}

type borrowState struct {
	mu     sync.RWMutex
	closed bool
	ctx    context.Context
	cancel context.CancelFunc
}

func borrow(ctx context.Context, client pkgstorage.StorageClient, use func(Client) error) error {
	lifetime, cancel := context.WithCancel(ctx)
	state := &borrowState{ctx: lifetime, cancel: cancel}
	defer state.close()
	return use(&borrowedClient{delegate: client, state: state})
}

func (s *borrowState) close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
}

func (s *borrowState) operationContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, pkgstorage.ErrNilContext
	}
	s.mu.RLock()
	closed := s.closed
	lifetime := s.ctx
	s.mu.RUnlock()
	if closed {
		return nil, nil, pkgstorage.ErrClientUnavailable
	}
	operationCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(lifetime, cancel)
	return operationCtx, func() {
		stop()
		cancel()
	}, nil
}

type borrowedClient struct {
	delegate pkgstorage.StorageClient
	state    *borrowState
}

func (c *borrowedClient) Put(ctx context.Context, key string, data []byte, options pkgstorage.PutOptions) error {
	operationCtx, cancel, err := c.state.operationContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	return c.delegate.Put(operationCtx, key, data, options)
}

func (c *borrowedClient) Get(ctx context.Context, key string) ([]byte, pkgstorage.ObjectInfo, error) {
	operationCtx, cancel, err := c.state.operationContext(ctx)
	if err != nil {
		return nil, pkgstorage.ObjectInfo{}, err
	}
	defer cancel()
	return c.delegate.Get(operationCtx, key)
}

func (c *borrowedClient) Delete(ctx context.Context, key string) error {
	operationCtx, cancel, err := c.state.operationContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	return c.delegate.Delete(operationCtx, key)
}

func (c *borrowedClient) Exists(ctx context.Context, key string) (bool, error) {
	operationCtx, cancel, err := c.state.operationContext(ctx)
	if err != nil {
		return false, err
	}
	defer cancel()
	return c.delegate.Exists(operationCtx, key)
}

var _ Access = (*access)(nil)
var _ Client = (*borrowedClient)(nil)
var _ config.DefaultContract = defaults{}
