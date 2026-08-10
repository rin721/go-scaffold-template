package redisstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	scaffoldcache "github.com/rin721/go-scaffold2/pkg/cache"
)

// Store 是基于 go-redis UniversalClient 的远端缓存实现。
type Store struct {
	client redis.UniversalClient
	cfg    resolvedConfig
}

// New 使用应用层传入的 Redis client 创建远端缓存适配器。
func New(client redis.UniversalClient, cfg *Config) (scaffoldcache.RemoteStore, error) {
	if client == nil {
		return nil, scaffoldcache.ErrNilRemoteStore
	}
	return &Store{client: client, cfg: resolveConfig(cfg)}, nil
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, time.Duration, error) {
	if err := validateContext(ctx); err != nil {
		return nil, 0, err
	}
	if err := validateKey(key); err != nil {
		return nil, 0, err
	}

	bytes, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, 0, scaffoldcache.ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}

	ttl, err := s.client.TTL(ctx, key).Result()
	if err != nil {
		return nil, 0, err
	}

	return bytes, ttl, nil
}

func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration, tags []string, tagsTTL time.Duration) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateKey(key); err != nil {
		return err
	}
	if ttl <= 0 {
		return fmt.Errorf("%w: ttl must be greater than 0", scaffoldcache.ErrInvalidTTL)
	}
	if tagsTTL < 0 {
		return fmt.Errorf("%w: tags ttl must be greater than or equal to 0", scaffoldcache.ErrInvalidTTL)
	}

	if err := s.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return err
	}

	for _, tag := range tags {
		if err := s.addTagIndex(ctx, tag, key, ttl, tagsTTL); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, key string) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateKey(key); err != nil {
		return err
	}

	return s.client.Del(ctx, key).Err()
}

func (s *Store) InvalidateTags(ctx context.Context, tags []string) ([]string, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	keys := make([]string, 0)
	var joined error

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}

		tagKey := s.tagKey(tag)
		members, err := s.client.SMembers(ctx, tagKey).Result()
		if err != nil {
			joined = errors.Join(joined, fmt.Errorf("read redis tag %q: %w", tag, err))
			continue
		}

		for _, member := range members {
			if _, exists := seen[member]; exists {
				continue
			}
			seen[member] = struct{}{}
			keys = append(keys, member)
		}

		if err := s.client.Del(ctx, tagKey).Err(); err != nil {
			joined = errors.Join(joined, fmt.Errorf("delete redis tag %q: %w", tag, err))
		}
	}

	if len(keys) > 0 {
		if err := s.client.Del(ctx, keys...).Err(); err != nil {
			joined = errors.Join(joined, fmt.Errorf("delete redis tagged values: %w", err))
		}
	}

	return keys, joined
}

func (s *Store) addTagIndex(ctx context.Context, tag string, key string, ttl time.Duration, tagsTTL time.Duration) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil
	}

	tagKey := s.tagKey(tag)
	if err := s.client.SAdd(ctx, tagKey, key).Err(); err != nil {
		return fmt.Errorf("add redis tag %q: %w", tag, err)
	}

	if tagsTTL > ttl {
		ttl = tagsTTL
	}
	if err := s.client.Expire(ctx, tagKey, ttl).Err(); err != nil {
		return fmt.Errorf("expire redis tag %q: %w", tag, err)
	}

	return nil
}

func (s *Store) tagKey(tag string) string {
	return s.cfg.TagPrefix + tag
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return scaffoldcache.ErrNilContext
	}
	return nil
}

func validateKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return scaffoldcache.ErrEmptyKey
	}
	return nil
}
