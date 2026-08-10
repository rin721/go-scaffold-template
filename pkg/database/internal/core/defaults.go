package core

import "time"

const (
	DefaultMaxOpenConns    = 25
	DefaultMaxIdleConns    = 5
	DefaultConnMaxLifetime = 30 * time.Minute
	DefaultConnMaxIdleTime = 5 * time.Minute
	DefaultPingTimeout     = 5 * time.Second
)

// DefaultConfig 返回数据库默认配置骨架。
//
// Engine、Driver 和 DSN 必须由调用方显式设置，避免在生产项目中隐式连接错误数据库。
func DefaultConfig() Config {
	return Config{
		Pool: PoolConfig{
			MaxOpenConns:    DefaultMaxOpenConns,
			MaxIdleConns:    DefaultMaxIdleConns,
			ConnMaxLifetime: DefaultConnMaxLifetime,
			ConnMaxIdleTime: DefaultConnMaxIdleTime,
		},
		PingTimeout: DefaultPingTimeout,
	}
}
