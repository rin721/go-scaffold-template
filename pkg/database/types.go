package database

import (
	"context"
	"time"
)

// Stats 是与具体连接池实现无关的运行状态快照。
type Stats struct {
	MaxOpenConnections int
	OpenConnections    int
	InUse              int
	Idle               int
	WaitCount          int64
	WaitDuration       time.Duration
	MaxIdleClosed      int64
	MaxIdleTimeClosed  int64
	MaxLifetimeClosed  int64
}

// Tx 是事务作用域令牌，只能传给 Repository.WithTx 使用。
//
// 事务不暴露 Commit、Rollback 或第三方 session，提交责任始终属于 WithinTx。
type Tx interface {
	databaseTransaction()
}

// Client 是业务和仓储层可使用的稳定数据库能力，不包含共享资源关闭权。
type Client interface {
	WithinTx(context.Context, func(context.Context, Tx) error) error
	Migrate(context.Context, ...Schema) error
}

// Resource 是数据库资源所有者使用的完整能力。
//
// Client 返回的非所有者视图与 Resource 不是同一个动态对象，调用方不能通过
// 类型断言重新取得连接池关闭权。
type Resource interface {
	Client() Client
	Ping(context.Context) error
	Stats() Stats
	Close() error
}

type sessionProvider interface {
	databaseSession(context.Context) (any, error)
}
