package cache

import (
	"context"
	"time"
)

// Client 定义业务代码使用的泛型缓存能力。
type Client[T any] interface {
	Get(ctx context.Context, key string) (T, error)
	Set(ctx context.Context, key string, value T, options ...SetOption) error
	Delete(ctx context.Context, key string) error
	InvalidateTags(ctx context.Context, tags ...string) error
}

// RemoteStore 定义远端缓存存储必须满足的字节级能力。
type RemoteStore interface {
	Get(ctx context.Context, key string) ([]byte, time.Duration, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration, tags []string, tagsTTL time.Duration) error
	Delete(ctx context.Context, key string) error
	InvalidateTags(ctx context.Context, tags []string) ([]string, error)
}
