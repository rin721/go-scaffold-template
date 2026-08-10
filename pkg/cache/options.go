package cache

import (
	"fmt"
	"strings"
	"time"
)

// SetOption 定义单次写入缓存时的可选参数。
type SetOption func(*setOptions)

type setOptions struct {
	ttl     time.Duration
	tags    []string
	tagsTTL time.Duration
}

type resolvedSetOptions struct {
	ttl     time.Duration
	tags    []string
	tagsTTL time.Duration
}

// WithTTL 设置当前缓存项的有效期。
func WithTTL(ttl time.Duration) SetOption {
	return func(options *setOptions) {
		options.ttl = ttl
	}
}

// WithTags 设置当前缓存项关联的失效标签。
func WithTags(tags ...string) SetOption {
	return func(options *setOptions) {
		options.tags = append(options.tags, tags...)
	}
}

// WithTagsTTL 设置标签索引的有效期。
func WithTagsTTL(ttl time.Duration) SetOption {
	return func(options *setOptions) {
		options.tagsTTL = ttl
	}
}

func resolveSetOptions(cfg resolvedConfig, options ...SetOption) (resolvedSetOptions, error) {
	resolved := resolvedSetOptions{
		ttl:     cfg.DefaultTTL,
		tagsTTL: cfg.DefaultTagsTTL,
	}

	var selected setOptions
	for _, option := range options {
		if option != nil {
			option(&selected)
		}
	}

	if selected.ttl != 0 {
		resolved.ttl = selected.ttl
	}
	if selected.tagsTTL != 0 {
		resolved.tagsTTL = selected.tagsTTL
	}
	if len(selected.tags) > 0 {
		resolved.tags = normalizeTags(cfg, selected.tags)
	}

	if resolved.ttl <= 0 {
		return resolvedSetOptions{}, fmt.Errorf("%w: ttl must be greater than 0", ErrInvalidTTL)
	}
	if resolved.tagsTTL < 0 {
		return resolvedSetOptions{}, fmt.Errorf("%w: tags ttl must be greater than or equal to 0", ErrInvalidTTL)
	}

	return resolved, nil
}

func normalizeTags(cfg resolvedConfig, tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		scoped := cfg.KeyPrefix + tag
		if _, exists := seen[scoped]; exists {
			continue
		}
		seen[scoped] = struct{}{}
		normalized = append(normalized, scoped)
	}
	return normalized
}
