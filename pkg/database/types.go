package database

import "github.com/rin721/go-scaffold2/pkg/database/internal/core"

// Result 表示 SQL 执行结果。
type Result = core.Result

// Executor 定义数据库查询和执行能力。
type Executor = core.Executor

// Tx 表示事务中的执行能力。
type Tx = core.Tx

// Transactor 定义事务边界能力。
type Transactor = core.Transactor

// HealthChecker 定义数据库健康检查能力。
type HealthChecker = core.HealthChecker

// StatsProvider 定义连接池统计能力。
type StatsProvider = core.StatsProvider

// Closer 定义资源释放能力。
type Closer = core.Closer

// Client 定义业务代码使用的统一数据库能力。
type Client = core.Client
