package cache

import (
	"fmt"
	"strings"
)

func resolveConfig(cfg *Config) (resolvedConfig, error) {
	defaults := DefaultConfig()
	resolved := resolvedConfig{
		DefaultTTL:      defaults.DefaultTTL,
		DefaultTagsTTL:  defaults.DefaultTagsTTL,
		KeyPrefix:       defaults.KeyPrefix,
		CleanupInterval: defaults.CleanupInterval,
	}

	if cfg == nil {
		return resolved, nil
	}

	if cfg.DefaultTTL != 0 {
		resolved.DefaultTTL = cfg.DefaultTTL
	}
	if cfg.DefaultTagsTTL != 0 {
		resolved.DefaultTagsTTL = cfg.DefaultTagsTTL
	}
	if cfg.KeyPrefix != "" {
		resolved.KeyPrefix = strings.TrimSpace(cfg.KeyPrefix)
	}
	if cfg.CleanupInterval != 0 {
		resolved.CleanupInterval = cfg.CleanupInterval
	}

	if resolved.DefaultTTL < 0 {
		return resolvedConfig{}, fmt.Errorf("%w: default ttl must be greater than or equal to 0", ErrInvalidTTL)
	}
	if resolved.DefaultTagsTTL < 0 {
		return resolvedConfig{}, fmt.Errorf("%w: default tags ttl must be greater than or equal to 0", ErrInvalidTTL)
	}
	if resolved.CleanupInterval < 0 {
		return resolvedConfig{}, fmt.Errorf("cache cleanup interval must be greater than or equal to 0")
	}

	return resolved, nil
}

func validateContext(ctx any) error {
	if ctx == nil {
		return ErrNilContext
	}
	return nil
}
