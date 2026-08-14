// Package cache 定义由 Kernel 治理的共享缓存后端组件。
package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgcache "github.com/rin721/go-scaffold-template/pkg/cache"
	"github.com/rin721/go-scaffold-template/pkg/cache/redisstore"
)

const (
	ID         app.ID = "cache"
	ConfigPath        = "cache"
)

// Driver 表示共享缓存后端的明确选择。
type Driver string

const (
	// DriverDisabled 表示当前进程不启用共享缓存后端。
	DriverDisabled Driver = "disabled"
	// DriverRedis 表示当前进程使用 Redis 作为共享缓存后端。
	DriverRedis Driver = "redis"
)

const (
	defaultRedisAddress      = "127.0.0.1:6379"
	defaultRedisDialTimeout  = 5 * time.Second
	defaultRedisReadTimeout  = 3 * time.Second
	defaultRedisWriteTimeout = 3 * time.Second
	defaultRedisPingTimeout  = 5 * time.Second
)

// Config 是 Cache App 的 typed 配置契约。
type Config struct {
	Driver Driver      `mapstructure:"driver"`
	Redis  RedisConfig `mapstructure:"redis"`
}

// RedisConfig 描述单 Redis 后端的连接、连接池和 tag namespace。
type RedisConfig struct {
	Address      string        `mapstructure:"address"`
	Username     string        `mapstructure:"username"`
	Password     string        `mapstructure:"password"`
	Database     int           `mapstructure:"database"`
	DialTimeout  time.Duration `mapstructure:"dialTimeout"`
	ReadTimeout  time.Duration `mapstructure:"readTimeout"`
	WriteTimeout time.Duration `mapstructure:"writeTimeout"`
	PoolSize     int           `mapstructure:"poolSize"`
	MinIdleConns int           `mapstructure:"minIdleConns"`
	TagPrefix    string        `mapstructure:"tagPrefix"`
	PingTimeout  time.Duration `mapstructure:"pingTimeout"`
}

// Access 是业务 typed Client 依赖的稳定共享后端入口。
//
// use 保持为包内方法，避免调用方取得 RemoteStore 并绕过 typed Client 契约。
type Access interface {
	Ping(context.Context) error
	use(context.Context, func(pkgcache.RemoteStore) error) error
}

type resource struct {
	driver      Driver
	client      *redis.Client
	store       pkgcache.RemoteStore
	pingTimeout time.Duration
}

type access struct {
	delegate app.Lease[*resource]
}

// Definition 返回无安装副作用的 Cache 组件声明。
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
		app.RestartRequired,
		app.WithReady(ready),
		app.WithStop(stop),
	)
}

// NewClient 使用稳定 Cache Access 构造由调用方拥有的泛型缓存 Client。
func NewClient[T any](backend Access, cfg *pkgcache.Config) (pkgcache.Client[T], error) {
	if backend == nil {
		return nil, fmt.Errorf("cache app access is nil")
	}
	return pkgcache.New[T](remoteAccess{backend: backend}, cfg)
}

func newAccess(delegate app.Lease[*resource]) (Access, error) {
	if delegate == nil {
		return nil, fmt.Errorf("cache lease is nil")
	}
	return &access{delegate: delegate}, nil
}

func (a *access) Ping(ctx context.Context) error {
	if ctx == nil {
		return pkgcache.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.delegate.Use(ctx, func(current *resource) error {
		if current == nil {
			return fmt.Errorf("cache instance is nil")
		}
		if current.driver == DriverDisabled {
			return pkgcache.ErrDisabled
		}
		if current.client == nil {
			return fmt.Errorf("cache redis client is unavailable")
		}
		pingCtx, cancel := context.WithTimeout(ctx, current.pingTimeout)
		defer cancel()
		if err := current.client.Ping(pingCtx).Err(); err != nil {
			return fmt.Errorf("ping cache backend: %w", err)
		}
		return nil
	})
}

func (a *access) use(ctx context.Context, use func(pkgcache.RemoteStore) error) error {
	if ctx == nil {
		return pkgcache.ErrNilContext
	}
	if use == nil {
		return fmt.Errorf("cache backend callback is nil")
	}
	return a.delegate.Use(ctx, func(current *resource) error {
		if current == nil {
			return fmt.Errorf("cache instance is nil")
		}
		if current.driver == DriverDisabled {
			return pkgcache.ErrDisabled
		}
		if current.store == nil {
			return fmt.Errorf("cache remote store is unavailable")
		}
		return use(current.store)
	})
}

type remoteAccess struct{ backend Access }

func (r remoteAccess) Get(ctx context.Context, key string) ([]byte, time.Duration, error) {
	var value []byte
	var ttl time.Duration
	err := r.backend.use(ctx, func(store pkgcache.RemoteStore) error {
		var err error
		value, ttl, err = store.Get(ctx, key)
		return err
	})
	return value, ttl, err
}

func (r remoteAccess) Set(ctx context.Context, key string, value []byte, ttl time.Duration, tags []string, tagsTTL time.Duration) error {
	return r.backend.use(ctx, func(store pkgcache.RemoteStore) error {
		return store.Set(ctx, key, value, ttl, tags, tagsTTL)
	})
}

func (r remoteAccess) Delete(ctx context.Context, key string) error {
	return r.backend.use(ctx, func(store pkgcache.RemoteStore) error {
		return store.Delete(ctx, key)
	})
}

func (r remoteAccess) InvalidateTags(ctx context.Context, tags []string) ([]string, error) {
	var keys []string
	err := r.backend.use(ctx, func(store pkgcache.RemoteStore) error {
		var err error
		keys, err = store.InvalidateTags(ctx, tags)
		return err
	})
	return keys, err
}

func build(ctx context.Context, cfg Config, _ struct{}) (*resource, error) {
	if ctx == nil {
		return nil, app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cfg.Driver == DriverDisabled {
		return &resource{driver: DriverDisabled, pingTimeout: cfg.Redis.PingTimeout}, nil
	}
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Address,
		Username:     cfg.Redis.Username,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.Database,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
	})
	store, err := redisstore.New(client, &redisstore.Config{TagPrefix: cfg.Redis.TagPrefix})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create cache redis store: %w", err), client.Close())
	}
	return &resource{
		driver:      DriverRedis,
		client:      client,
		store:       store,
		pingTimeout: cfg.Redis.PingTimeout,
	}, nil
}

func ready(ctx context.Context, current *resource) error {
	if current == nil {
		return fmt.Errorf("cache instance is nil")
	}
	if current.driver == DriverDisabled {
		return nil
	}
	if current.client == nil {
		return fmt.Errorf("cache redis client is unavailable")
	}
	pingCtx, cancel := context.WithTimeout(ctx, current.pingTimeout)
	defer cancel()
	if err := current.client.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("verify cache readiness: %w", err)
	}
	return nil
}

func stop(_ context.Context, current *resource) error {
	if current == nil {
		return fmt.Errorf("cache instance is nil")
	}
	if current.client == nil {
		return nil
	}
	if err := current.client.Close(); err != nil {
		return fmt.Errorf("close cache redis client: %w", err)
	}
	return nil
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
	cfg.Driver = Driver(strings.ToLower(strings.TrimSpace(string(cfg.Driver))))
	cfg.Redis.Address = strings.TrimSpace(cfg.Redis.Address)
	cfg.Redis.Username = strings.TrimSpace(cfg.Redis.Username)
	cfg.Redis.TagPrefix = strings.TrimSpace(cfg.Redis.TagPrefix)
	if cfg.Redis.DialTimeout == 0 {
		cfg.Redis.DialTimeout = defaults.Redis.DialTimeout
	}
	if cfg.Redis.ReadTimeout == 0 {
		cfg.Redis.ReadTimeout = defaults.Redis.ReadTimeout
	}
	if cfg.Redis.WriteTimeout == 0 {
		cfg.Redis.WriteTimeout = defaults.Redis.WriteTimeout
	}
	if cfg.Redis.PingTimeout == 0 {
		cfg.Redis.PingTimeout = defaults.Redis.PingTimeout
	}
	if cfg.Redis.TagPrefix == "" {
		cfg.Redis.TagPrefix = defaults.Redis.TagPrefix
	}
	switch cfg.Driver {
	case DriverDisabled:
	case DriverRedis:
		if cfg.Redis.Address == "" {
			return Config{}, fmt.Errorf("cache redis address is required")
		}
	default:
		return Config{}, fmt.Errorf("unsupported cache driver %q", cfg.Driver)
	}
	if cfg.Redis.Database < 0 {
		return Config{}, fmt.Errorf("cache redis database must be non-negative")
	}
	if cfg.Redis.DialTimeout < 0 || cfg.Redis.ReadTimeout < 0 || cfg.Redis.WriteTimeout < 0 || cfg.Redis.PingTimeout < 0 {
		return Config{}, fmt.Errorf("cache redis timeouts must be non-negative")
	}
	if cfg.Redis.PoolSize < 0 || cfg.Redis.MinIdleConns < 0 {
		return Config{}, fmt.Errorf("cache redis pool values must be non-negative")
	}
	if cfg.Redis.PoolSize > 0 && cfg.Redis.MinIdleConns > cfg.Redis.PoolSize {
		return Config{}, fmt.Errorf("cache redis min idle connections exceed pool size")
	}
	return cfg, nil
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
	database, err := config.Number(fmt.Sprint(values.Redis.Database))
	if err != nil {
		return nil, config.Continue, err
	}
	poolSize, err := config.Number(fmt.Sprint(values.Redis.PoolSize))
	if err != nil {
		return nil, config.Continue, err
	}
	minIdle, err := config.Number(fmt.Sprint(values.Redis.MinIdleConns))
	if err != nil {
		return nil, config.Continue, err
	}
	return config.Object{
		config.FieldOf("driver", config.String(string(values.Driver))),
		config.FieldOf("redis", config.ObjectValue(config.Object{
			config.FieldOf("address", config.String(values.Redis.Address)),
			config.FieldOf("username", config.String(values.Redis.Username)),
			config.FieldOf("password", config.String(values.Redis.Password)),
			config.FieldOf("database", database),
			config.FieldOf("dialTimeout", config.Duration(values.Redis.DialTimeout)),
			config.FieldOf("readTimeout", config.Duration(values.Redis.ReadTimeout)),
			config.FieldOf("writeTimeout", config.Duration(values.Redis.WriteTimeout)),
			config.FieldOf("poolSize", poolSize),
			config.FieldOf("minIdleConns", minIdle),
			config.FieldOf("tagPrefix", config.String(values.Redis.TagPrefix)),
			config.FieldOf("pingTimeout", config.Duration(values.Redis.PingTimeout)),
		})),
	}, config.Continue, nil
}

func defaultConfig() Config {
	return Config{
		Driver: DriverDisabled,
		Redis: RedisConfig{
			Address:      defaultRedisAddress,
			DialTimeout:  defaultRedisDialTimeout,
			ReadTimeout:  defaultRedisReadTimeout,
			WriteTimeout: defaultRedisWriteTimeout,
			TagPrefix:    redisstore.DefaultTagPrefix,
			PingTimeout:  defaultRedisPingTimeout,
		},
	}
}

var _ Access = (*access)(nil)
var _ pkgcache.RemoteStore = remoteAccess{}
var _ config.DefaultContract = defaults{}
