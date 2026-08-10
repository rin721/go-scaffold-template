package cache

import "time"

// Config 定义多级缓存客户端构造参数。
type Config struct {
	DefaultTTL      time.Duration
	DefaultTagsTTL  time.Duration
	KeyPrefix       string
	CleanupInterval time.Duration
}

type resolvedConfig struct {
	DefaultTTL      time.Duration
	DefaultTagsTTL  time.Duration
	KeyPrefix       string
	CleanupInterval time.Duration
}
