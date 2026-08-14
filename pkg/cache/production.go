package cache

import (
	"context"
	"time"

	"github.com/rin721/go-scaffold-template/pkg/concurrency"
)

// GetOrLoad 使用 singleflight 防止热点缓存击穿。
func GetOrLoad[T any](ctx context.Context, sf *concurrency.SingleFlight, client Client[T], key string, loader func(context.Context) (T, error), options ...SetOption) (T, error) {
	if sf == nil {
		sf = &concurrency.SingleFlight{}
	}
	value, err, _ := sf.Do(key, func() (any, error) {
		cached, err := client.Get(ctx, key)
		if err == nil {
			return cached, nil
		}
		loaded, err := loader(ctx)
		if err != nil {
			var zero T
			return zero, err
		}
		if err := client.Set(ctx, key, loaded, options...); err != nil {
			var zero T
			return zero, err
		}
		return loaded, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return value.(T), nil
}

// GetMany 批量读取缓存，返回命中的 key。
func GetMany[T any](ctx context.Context, client Client[T], keys ...string) (map[string]T, error) {
	out := make(map[string]T, len(keys))
	for _, key := range keys {
		value, err := client.Get(ctx, key)
		if err != nil {
			continue
		}
		out[key] = value
	}
	return out, nil
}

// Diagnostics 描述缓存运行状态。
type Diagnostics struct {
	KeyPrefix string
	CheckedAt time.Time
}
