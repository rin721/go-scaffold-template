package redisstore

// Config 定义 Redis 缓存适配器配置。
type Config struct {
	TagPrefix string
}

type resolvedConfig struct {
	TagPrefix string
}
