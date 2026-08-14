package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"github.com/vmihailenco/msgpack/v5"
)

type layeredClient[T any] struct {
	local   *gocache.Cache
	remote  RemoteStore
	cfg     resolvedConfig
	mu      sync.RWMutex
	tagKeys map[string]map[string]struct{}

	closed        atomic.Bool
	closeOnce     sync.Once
	cleanupCancel context.CancelFunc
	cleanupDone   chan struct{}
}

// New 创建 L1 内存缓存 + L2 远端缓存组成的泛型缓存客户端。
func New[T any](remote RemoteStore, cfg *Config) (Client[T], error) {
	if remote == nil {
		return nil, ErrNilRemoteStore
	}

	resolved, err := resolveConfig(cfg)
	if err != nil {
		return nil, err
	}

	client := &layeredClient[T]{
		local:   gocache.New(gocache.NoExpiration, 0),
		remote:  remote,
		cfg:     resolved,
		tagKeys: make(map[string]map[string]struct{}),
	}
	client.startCleanup()
	return client, nil
}

func (c *layeredClient[T]) Get(ctx context.Context, key string) (T, error) {
	var zero T
	if err := validateContext(ctx); err != nil {
		return zero, err
	}
	if err := c.validateOpen(); err != nil {
		return zero, err
	}

	cacheKey, err := c.cacheKey(key)
	if err != nil {
		return zero, err
	}

	if value, found := c.local.Get(cacheKey); found {
		bytes, ok := value.([]byte)
		if !ok {
			return zero, fmt.Errorf("%w: local value for %q has type %T", ErrInvalidCachedValue, key, value)
		}
		return decodeValue[T](bytes)
	}

	bytes, ttl, err := c.remote.Get(ctx, cacheKey)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return zero, ErrNotFound
		}
		return zero, fmt.Errorf("get remote cache value %q: %w", key, err)
	}

	if ttl > 0 {
		c.local.Set(cacheKey, cloneBytes(bytes), ttl)
	}

	return decodeValue[T](bytes)
}

func (c *layeredClient[T]) Set(ctx context.Context, key string, value T, options ...SetOption) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := c.validateOpen(); err != nil {
		return err
	}

	cacheKey, err := c.cacheKey(key)
	if err != nil {
		return err
	}

	resolvedOptions, err := resolveSetOptions(c.cfg, options...)
	if err != nil {
		return err
	}

	bytes, err := encodeValue(value)
	if err != nil {
		return err
	}

	c.local.Set(cacheKey, cloneBytes(bytes), resolvedOptions.ttl)
	c.addTagIndexes(cacheKey, resolvedOptions.tags)

	if err := c.remote.Set(ctx, cacheKey, bytes, resolvedOptions.ttl, resolvedOptions.tags, resolvedOptions.tagsTTL); err != nil {
		c.local.Delete(cacheKey)
		c.removeKeyFromAllTags(cacheKey)
		return fmt.Errorf("set remote cache value %q: %w", key, err)
	}

	return nil
}

func (c *layeredClient[T]) Delete(ctx context.Context, key string) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := c.validateOpen(); err != nil {
		return err
	}

	cacheKey, err := c.cacheKey(key)
	if err != nil {
		return err
	}

	c.local.Delete(cacheKey)
	c.removeKeyFromAllTags(cacheKey)

	if err := c.remote.Delete(ctx, cacheKey); err != nil {
		return fmt.Errorf("delete remote cache value %q: %w", key, err)
	}
	return nil
}

func (c *layeredClient[T]) InvalidateTags(ctx context.Context, tags ...string) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := c.validateOpen(); err != nil {
		return err
	}

	normalizedTags := normalizeTags(c.cfg, tags)
	if len(normalizedTags) == 0 {
		return nil
	}

	localKeys := c.keysForTags(normalizedTags)
	remoteKeys, err := c.remote.InvalidateTags(ctx, normalizedTags)
	allKeys := append(localKeys, remoteKeys...)
	c.deleteLocalKeys(allKeys)
	c.removeTagIndexes(normalizedTags)

	if err != nil {
		return fmt.Errorf("invalidate remote cache tags: %w", err)
	}
	return nil
}

// Close 停止当前 typed Client 拥有的 L1 清理任务并释放本地状态。
func (c *layeredClient[T]) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		if c.cleanupCancel != nil {
			c.cleanupCancel()
		}
		if c.cleanupDone != nil {
			<-c.cleanupDone
		}
		c.local.Flush()
		c.mu.Lock()
		c.tagKeys = make(map[string]map[string]struct{})
		c.mu.Unlock()
	})
	return nil
}

func (c *layeredClient[T]) startCleanup() {
	c.cleanupDone = make(chan struct{})
	if c.cfg.CleanupInterval <= 0 {
		close(c.cleanupDone)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cleanupCancel = cancel
	go func() {
		defer close(c.cleanupDone)
		ticker := time.NewTicker(c.cfg.CleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.local.DeleteExpired()
			}
		}
	}()
}

func (c *layeredClient[T]) validateOpen() error {
	if c == nil || c.closed.Load() {
		return ErrClientClosed
	}
	return nil
}

func (c *layeredClient[T]) cacheKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", ErrEmptyKey
	}
	return c.cfg.KeyPrefix + key, nil
}

func (c *layeredClient[T]) addTagIndexes(key string, tags []string) {
	if len(tags) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, tag := range tags {
		keys, exists := c.tagKeys[tag]
		if !exists {
			keys = make(map[string]struct{})
			c.tagKeys[tag] = keys
		}
		keys[key] = struct{}{}
	}
}

func (c *layeredClient[T]) keysForTags(tags []string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	seen := make(map[string]struct{})
	keys := make([]string, 0)
	for _, tag := range tags {
		for key := range c.tagKeys[tag] {
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	return keys
}

func (c *layeredClient[T]) deleteLocalKeys(keys []string) {
	for _, key := range keys {
		if key != "" {
			c.local.Delete(key)
		}
	}
	for _, key := range keys {
		c.removeKeyFromAllTags(key)
	}
}

func (c *layeredClient[T]) removeTagIndexes(tags []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, tag := range tags {
		delete(c.tagKeys, tag)
	}
}

func (c *layeredClient[T]) removeKeyFromAllTags(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for tag, keys := range c.tagKeys {
		delete(keys, key)
		if len(keys) == 0 {
			delete(c.tagKeys, tag)
		}
	}
}

func encodeValue[T any](value T) ([]byte, error) {
	bytes, err := msgpack.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal value: %v", ErrInvalidCachedValue, err)
	}
	return bytes, nil
}

func decodeValue[T any](bytes []byte) (T, error) {
	var value T
	if err := msgpack.Unmarshal(bytes, &value); err != nil {
		return value, fmt.Errorf("%w: unmarshal value: %v", ErrInvalidCachedValue, err)
	}
	return value, nil
}

func cloneBytes(bytes []byte) []byte {
	cloned := make([]byte, len(bytes))
	copy(cloned, bytes)
	return cloned
}
