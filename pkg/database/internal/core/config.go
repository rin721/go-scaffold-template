package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func ResolveConfig(cfg *Config) (ResolvedConfig, error) {
	if cfg == nil {
		return ResolvedConfig{}, fmt.Errorf("%w: config is nil", ErrInvalidEngine)
	}

	resolved := DefaultConfig()
	resolved.Engine = cfg.Engine
	resolved.Driver = cfg.Driver
	resolved.DSN = strings.TrimSpace(cfg.DSN)

	if cfg.Pool.MaxOpenConns != 0 {
		resolved.Pool.MaxOpenConns = cfg.Pool.MaxOpenConns
	}
	if cfg.Pool.MaxIdleConns != 0 {
		resolved.Pool.MaxIdleConns = cfg.Pool.MaxIdleConns
	}
	if cfg.Pool.ConnMaxLifetime != 0 {
		resolved.Pool.ConnMaxLifetime = cfg.Pool.ConnMaxLifetime
	}
	if cfg.Pool.ConnMaxIdleTime != 0 {
		resolved.Pool.ConnMaxIdleTime = cfg.Pool.ConnMaxIdleTime
	}
	if cfg.PingTimeout != 0 {
		resolved.PingTimeout = cfg.PingTimeout
	}

	if err := ValidateResolvedConfig(ResolvedConfig(resolved)); err != nil {
		return ResolvedConfig{}, err
	}

	return ResolvedConfig(resolved), nil
}

func ValidateResolvedConfig(cfg ResolvedConfig) error {
	switch cfg.Engine {
	case EngineGORM, EngineSQL:
	default:
		return fmt.Errorf("%w: %q", ErrInvalidEngine, cfg.Engine)
	}

	switch cfg.Driver {
	case DriverPostgres, DriverMySQL:
	default:
		return fmt.Errorf("%w: %q", ErrInvalidDriver, cfg.Driver)
	}

	if strings.TrimSpace(cfg.DSN) == "" {
		return ErrEmptyDSN
	}

	if cfg.Pool.MaxOpenConns < 0 {
		return fmt.Errorf("%w: max open conns must be greater than or equal to 0", ErrInvalidPoolConfig)
	}
	if cfg.Pool.MaxIdleConns < 0 {
		return fmt.Errorf("%w: max idle conns must be greater than or equal to 0", ErrInvalidPoolConfig)
	}
	if cfg.Pool.ConnMaxLifetime < 0 {
		return fmt.Errorf("%w: conn max lifetime must be greater than or equal to 0", ErrInvalidPoolConfig)
	}
	if cfg.Pool.ConnMaxIdleTime < 0 {
		return fmt.Errorf("%w: conn max idle time must be greater than or equal to 0", ErrInvalidPoolConfig)
	}
	if cfg.PingTimeout < 0 {
		return fmt.Errorf("%w: ping timeout must be greater than or equal to 0", ErrInvalidPoolConfig)
	}

	return nil
}

func ValidateContext(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	return nil
}

func ApplyPoolConfig(db *sql.DB, cfg PoolConfig) {
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
}

func Ping(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	if err := ValidateContext(ctx); err != nil {
		return err
	}
	if timeout <= 0 {
		return db.PingContext(ctx)
	}

	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return db.PingContext(pingCtx)
}

func SQLDriverName(driver Driver) string {
	switch driver {
	case DriverPostgres:
		return "pgx"
	case DriverMySQL:
		return "mysql"
	default:
		return string(driver)
	}
}
