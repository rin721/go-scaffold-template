# 产品需求：数据库单轨 GORM 与稳定访问边界

## 1. 目标

数据库技术必须在 `internal/kernel/app/database` 的构造代码中明确选择 GORM，不允许通过配置文件在 GORM、SQLX 或其他底层实现之间切换。`pkg/database` 应隐藏连接池、GORM session、动态模型、迁移和错误翻译细节，为业务提供简单、稳定、通用的项目自有数据库契约。

## 2. 范围

### 2.1 单轨实现与配置

- 删除 `Engine`、SQLX、`sqldb/**`、旧 `gormdb/**` 入口及其测试和依赖，不保留兼容别名。
- Kernel Database App 在 `build` 中直接调用 `database.NewGORM`。
- 配置仅包含 `driver`、DSN、连接池和 Ping 参数；Driver 只允许 `sqlite`、`postgres`、`mysql`。
- 默认使用 pure-Go SQLite，默认路径为 `.data/app.db`，不依赖 CGO。

### 2.2 公开能力边界

- `Resource` 拥有 `Ping`、`Stats`、`Close`；`Client` 和 `Tx` 不拥有共享资源关闭权。
- Resource 与 Client 必须是不同动态对象，业务不能通过类型断言恢复所有权。
- Kernel `Access` 提供 `Ping`、`Use` 和 `WithinTx`；回调取得 Borrowed Client，租约结束后逃逸对象统一返回 `ErrClientUnavailable`。
- 公开签名只能包含标准库和项目类型，不返回 `*gorm.DB`、dialector、`sql.DBStats` 或其他第三方 named type。

### 2.3 Schema、迁移与 Repository

- 项目 Schema 描述表、字段、主键、nullable、长度、类型化 Default、索引和单列外键；业务实体不写 GORM tag。
- Reference 两端使用 Schema 字段名，外键动作使用项目专用类型和常量。
- Migrate 只承诺 additive migration：创建缺失的表、列、索引和约束，不删除、重命名或修改既有列。
- `BaseRepository[T]` 提供 Create、First、Find、Count、Update、SoftDelete 和事务重绑定。
- Filter、Order、Page、Changes 必须先按 Schema 校验，再映射为列名；Update/SoftDelete 拒绝空筛选。
- 自增、Version、SoftDelete 初值由 Repository 管理；乐观锁零影响行返回 `ErrOptimisticConflict`。

### 2.4 错误、事务和资源

- not found、duplicate key、foreign-key violation、optimistic conflict 使用项目 sentinel，可通过 `errors.Is` 判断。
- 驱动错误文本和可展开错误树不得泄漏 DSN、密码或 Token。
- 事务回调错误或 panic 必须先回滚；操作同时受事务根 context 和单次操作 context 约束。
- SQLite 文件目录与文件使用受控权限，拒绝符号链接；开启 foreign keys、busy timeout 和 WAL。
- 私有 `:memory:` SQLite 固定为单连接，避免连接池取得不同内存数据库。

## 3. 验收标准

- SQLite 真连接覆盖资源创建、权限、Schema、CRUD、分页、索引、外键、事务、乐观锁、软删除和错误分类。
- `package database_test` 仅通过公开 API 完成迁移、CRUD 和事务，并验证 Client 无法恢复 Resource。
- PostgreSQL/MySQL 契约测试使用环境变量 DSN；CI service containers 强制执行两种真连接测试。
- `go test ./...`、`go test -race ./...`、`go vet ./...`、`go mod tidy -diff`、`CGO_ENABLED=0 go build ./...`、文档链接与 `git diff --check` 通过。
- 搜索确认旧 Engine/SQLX/sqldb/gormdb 当前入口已删除，GORM 只存在于 `pkg/database` 实现内部。

## 4. 非目标

- 不提供任意 SQL 或 GORM session 逃逸口。
- 不实现破坏性或版本化迁移、读写分离、分库分表、多租户和业务模型。
- 不在本任务中启动 PostgreSQL/MySQL 服务、部署应用或推送远端分支。
