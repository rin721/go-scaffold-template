package cache

import "time"

const (
	DefaultTagsTTL         = 720 * time.Hour
	DefaultCleanupInterval = time.Minute
)

// DefaultConfig 返回一份可修改的默认配置。
//
// DefaultTTL 默认为 0，调用方必须在配置或 Set 选项中显式提供有效 TTL。
func DefaultConfig() Config {
	return Config{
		DefaultTagsTTL:  DefaultTagsTTL,
		CleanupInterval: DefaultCleanupInterval,
	}
}
