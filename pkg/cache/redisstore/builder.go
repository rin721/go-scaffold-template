package redisstore

import "strings"

func resolveConfig(cfg *Config) resolvedConfig {
	defaults := DefaultConfig()
	resolved := resolvedConfig{TagPrefix: defaults.TagPrefix}

	if cfg == nil {
		return resolved
	}

	if cfg.TagPrefix != "" {
		resolved.TagPrefix = strings.TrimSpace(cfg.TagPrefix)
	}
	return resolved
}
