package redisstore

const DefaultTagPrefix = "cache:tag:"

// DefaultConfig 返回一份可修改的默认配置。
func DefaultConfig() Config {
	return Config{TagPrefix: DefaultTagPrefix}
}
