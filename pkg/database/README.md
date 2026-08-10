# database

`pkg/database` 是项目统一数据库基础能力。根包定义稳定协议、配置、Driver、Engine 和唯一推荐构造入口；`gormdb` 与 `sqldb` 是两个明确实现，分别覆盖成熟 ORM 开发和显式 SQL 开发。

## 技术选型

- `gormdb` 使用 `gorm.io/gorm`、`gorm.io/driver/postgres`、`gorm.io/driver/mysql`。GORM 是 Go 生态成熟 ORM，适合有实体建模、关联、Hook、批量写入和插件需求的业务项目。
- `sqldb` 使用 Go 标准库 `database/sql`、`github.com/jmoiron/sqlx`、`github.com/jackc/pgx/v5/stdlib` 和 `github.com/go-sql-driver/mysql`。这条路线适合复杂 SQL、报表、批处理、性能敏感查询和需要精确控制 SQL 的场景。
- 根包不把 GORM 全量 ORM API 抽象成统一接口。统一协议只覆盖应用层和仓储层都能稳定依赖的基础能力，避免为了兼容两种实现而制造难用的大接口。

## 推荐入口

业务入口优先使用 `database.New`，通过 `Config.Engine` 明确选择实现：

```go
package main

import (
	"context"
	"time"

	"github.com/rin721/go-scaffold2/pkg/database"
)

func main() {
	ctx := context.Background()
	db, err := database.New(ctx, &database.Config{
		Engine: database.EngineGORM,
		Driver: database.DriverPostgres,
		DSN:    "postgres://user:password@localhost:5432/app?sslmode=disable",
		Pool: database.PoolConfig{
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 30 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
		},
		PingTimeout: 5 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	defer db.Close()
}
```

`Engine`、`Driver` 和 `DSN` 必须显式配置。数据库连接是高风险外部资源，包内不会偷偷选择默认数据库或默认实现。

## 统一协议

`database.Client` 组合了以下能力：

- `Exec(ctx, query, args...)`：执行写入或 DDL，返回 `Result`。
- `Query(ctx, query, args...)`：返回标准库 `*sql.Rows`。
- `QueryRow(ctx, query, args...)`：返回标准库 `*sql.Row`。
- `Get(ctx, dest, query, args...)`：查询单条记录并映射到结构体。
- `Select(ctx, dest, query, args...)`：查询多条记录并映射到切片。
- `WithinTx(ctx, opts, fn)`：统一事务边界，回调返回错误时回滚，成功时提交。
- `Ping(ctx)`、`Stats()`、`Close()`：健康检查、连接池统计和资源释放。
- `ValidateConfig(cfg)`：只校验配置和默认值语义，不建立数据库连接。

统一协议面向应用层和仓储层，不暴露 `*gorm.DB` 或 `*sqlx.DB`。如果明确需要 GORM 的 ORM 专用能力，应把依赖限定在基础设施实现层，并使用 `gormdb.Client.DB(ctx)`。

## Engine 选择

| Engine | 适用场景 | 注意事项 |
| --- | --- | --- |
| `EngineGORM` | 成熟业务项目默认 ORM 能力；实体建模、关联、Hook、批量操作和插件扩展。 | 不建议把 `*gorm.DB` 传到领域层；复杂 SQL 仍可通过统一协议执行。 |
| `EngineSQL` | 复杂 SQL、报表、批处理、高性能读模型和需要精确控制 SQL 的仓储。 | 不提供 ORM 关联和模型生命周期能力，结构体映射由 `sqlx` 完成。 |

两种实现都共享 `Config`、连接池配置、启动 `Ping`、事务入口和关闭责任。应用入口创建数据库客户端后，应通过构造函数显式注入业务组件。

需要由 kernel 管理配置切换时，不修改本包的通用契约；composition root 使用 `internal/adapter/database` 注册能力，并把稳定 Access 注入业务构造函数。

## 配置项

| 字段 | 说明 | 默认值 |
| --- | --- | --- |
| `Engine` | 底层实现，支持 `gorm` 或 `sql` | 无，必须显式配置 |
| `Driver` | 数据库驱动，支持 `postgres` 或 `mysql` | 无，必须显式配置 |
| `DSN` | 数据库连接串 | 无，必须显式配置 |
| `Pool.MaxOpenConns` | 最大打开连接数 | `25` |
| `Pool.MaxIdleConns` | 最大空闲连接数 | `5` |
| `Pool.ConnMaxLifetime` | 单连接最大生命周期 | `30m` |
| `Pool.ConnMaxIdleTime` | 空闲连接最大保留时间 | `5m` |
| `PingTimeout` | 构造和健康检查默认超时 | `5s` |

错误信息不得包含完整 DSN、密码或 Token。调用方需要记录配置问题时，应记录数据库类型和环境标识，不记录连接串原文。

## 事务示例

```go
err := db.WithinTx(ctx, nil, func(ctx context.Context, tx database.Tx) error {
	_, err := tx.Exec(ctx, "UPDATE accounts SET balance = balance - ? WHERE id = ?", 100, 1)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE accounts SET balance = balance + ? WHERE id = ?", 100, 2)
	return err
})
```

事务回调返回错误时自动回滚，返回 `nil` 时提交。业务代码不要在回调外保存 `Tx`，也不要自行提交或回滚内部事务。

## 不做范围

v1 不内置迁移框架、读写分离、分库分表、多租户、审计字段插件、链路追踪或指标采集。需要这些能力时，应在明确业务场景后扩展，并继续保持根包协议稳定。
