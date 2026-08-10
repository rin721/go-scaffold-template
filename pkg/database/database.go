package database

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold2/pkg/database/gormdb"
	"github.com/rin721/go-scaffold2/pkg/database/internal/core"
	"github.com/rin721/go-scaffold2/pkg/database/sqldb"
)

var (
	openGORM = func(ctx context.Context, cfg core.ResolvedConfig) (core.Client, error) {
		return gormdb.Open(ctx, cfg)
	}
	openSQL = func(ctx context.Context, cfg core.ResolvedConfig) (core.Client, error) {
		return sqldb.Open(ctx, cfg)
	}
)

// New 根据 Engine 明确选择底层实现并创建统一数据库客户端。
func New(ctx context.Context, cfg *Config) (Client, error) {
	if err := core.ValidateContext(ctx); err != nil {
		return nil, err
	}

	resolved, err := core.ResolveConfig(cfg)
	if err != nil {
		return nil, err
	}

	switch resolved.Engine {
	case EngineGORM:
		client, err := openGORM(ctx, resolved)
		if err != nil {
			return nil, fmt.Errorf("open gorm database: %w", err)
		}
		return client, nil
	case EngineSQL:
		client, err := openSQL(ctx, resolved)
		if err != nil {
			return nil, fmt.Errorf("open sql database: %w", err)
		}
		return client, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidEngine, resolved.Engine)
	}
}
