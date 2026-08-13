# 开发设计：数据库单轨 GORM 与稳定访问边界

## 1. 选择位置与配置

唯一实现选择发生在 Kernel Database App 的构造函数：

```text
internal/kernel/app/database.build
  -> pkg/database.NewGORM
     -> sqlite / postgres / mysql dialector
```

配置中的 Driver 只表示数据库协议和方言，不表示 ORM。删除 Engine 后，运行时无法切换为 SQLX 或另一套实现。

## 2. 所有权与租约

```go
type Resource interface {
    Client() Client
    Ping(context.Context) error
    Stats() Stats
    Close() error
}

type Client interface {
    WithinTx(context.Context, func(context.Context, Tx) error) error
    Migrate(context.Context, ...Schema) error
}
```

`gormResource` 持有连接池和 `gormClient`。两者是不同动态对象：只有构造所有者和 Kernel 生命周期能够取得 Resource；业务只能获得 Client。

Kernel `Access.Use` 在当前 Resource 租约内创建 Borrowed Client。借用状态持有生命周期 context，回调结束后关闭状态并取消所有派生操作。Repository 只保存私有 session provider，因此逃逸 Repository 和 Tx 与 Client 一样返回 `ErrClientUnavailable`。`Access.Ping` 在 Resource 租约内完成 readiness 检查，不泄漏 Stats 或 Close。

## 3. Schema 与迁移

Schema 使用项目元数据，在包内生成 GORM 动态模型。创建 DDL 前完成整批校验：

- 标识符、字段和列唯一性；
- 字段类型、长度、nullable 与类型化 Default；
- 主键、自增、Version 和 SoftDelete 不变量；
- 索引字段与全局索引名；
- 外键两端类型、目标唯一约束、`SET NULL` nullable 和目标 Schema 存在性。

迁移不调用会调整已有列的 `AutoMigrate`。实现只创建缺失表、列、索引和外键。SQLite 不能给既有表追加外键，因此在任何 DDL 前拒绝该候选；PostgreSQL/MySQL 等全部表列准备完成后再添加外键，避免输入顺序影响。

## 4. Repository 与事务

`BaseRepository[T]` 构造时验证实体字段与 Schema 完全匹配。查询和修改只接受 Schema 字段名：

```text
Query / Changes
  -> Schema 字段与值类型校验
  -> 项目字段映射为数据库列
  -> 私有 GORM session 执行
  -> 项目实体或 sentinel error
```

Create 忽略调用方传入的自增值，并把 Version、SoftDelete 初始化为 1、nil。Update 与 SoftDelete 必须含筛选条件；启用 Version 时还必须含版本等值条件，并原子执行 `version = version + 1`。

Tx 是不可提交、不可回滚、不可取得第三方 session 的重绑定令牌。事务回调成功时提交，错误时回滚；panic 时先回滚再维持原 panic。每次事务操作 context 同时绑定事务根 context，调用方不能用新的 Background context 绕过取消。

## 5. 错误与敏感信息

GORM 使用 Silent logger 和 `TranslateError`。包内把可识别错误映射为项目 sentinel；其他驱动错误映射为 `ErrOperationFailed`。安全错误对象保留 `errors.Is` 判定，但不返回原驱动文本，也不暴露可展开的原始错误链。

SQLite 文件系统错误同样经过脱敏。DSN 只用于构造连接，不出现在日志、错误和测试输出中。

## 6. SQLite 边界

- 默认路径 `.data/app.db`，父目录自动创建并加入 Git ignore。
- Unix 目录权限为 0700、文件权限为 0600；拒绝符号链接和非普通文件。
- DSN 统一增加 foreign keys、5 秒 busy timeout 和 WAL pragma。
- `:memory:` 固定 `MaxOpenConns=1`、`MaxIdleConns=1`。

## 7. 文件影响

- `pkg/database/**`：公开契约、GORM 资源、Schema、迁移、Repository、Borrowed Client、测试和使用文档。
- `internal/kernel/app/database/**`：固定 GORM 构造、Access Ping/Use/WithinTx 与租约测试。
- `config.example.yaml`、根 README、Kernel/pkg 导航：移除 Engine 并同步当前使用方式。
- `go.mod`、`go.sum`：增加 pure-Go SQLite，删除 SQLX 路线依赖。
- `.github/workflows/database.yml`：SQLite、本地门禁和 PostgreSQL/MySQL service contract。

## 8. 验证策略

本地执行 SQLite、公开 API、全仓 Test/Race/Vet、模块一致性、无 CGO 构建、YAML、文档链接和 Diff 检查。PostgreSQL/MySQL 使用与本地同一 `TestConfiguredServerDrivers` 契约；当前机器没有 DSN、容器运行时或端口服务，因此只记录 CI 门禁已定义，不把它写成已运行通过。
