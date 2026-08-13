# 任务账本：数据库单轨 GORM 与稳定访问边界

## 1. 确认与基线

- 当前状态：**已完成**。
- 用户已明确要求把本次数据库任务定位为 `010`、落实方案并记录已完成。
- 最终提交父基线：`main@d8ba97231f20ce7391469ebe508c5e16b7a508f7`；该提交是已经完成的 `009`，本任务不修改其历史。
- 2026-08-14 已执行 `git fetch --prune origin`；`origin/main` 仍为 `139d437e4407583f6a71afd17808e149a9663d72`。
- 本任务形成一个独立本地 commit，不 push。

## 2. 任务清单

| ID | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- |
| `DB-001` | 单轨 GORM 与三 Driver | Engine/SQLX/旧入口删除；Kernel 代码固定选择 GORM | 已完成 |
| `DB-002` | Resource/Client/Tx/Access 所有权 | Client 无 Close；Borrowed 对象租约后失效；Access 提供窄 Ping | 已完成 |
| `DB-003` | Schema 与 additive migration | 字段、Default、索引、关系校验；不修改既有列 | 已完成 |
| `DB-004` | 通用 Repository | CRUD、分页、事务重绑、乐观锁、软删除和全表修改保护 | 已完成 |
| `DB-005` | 错误、取消与资源安全 | sentinel、敏感信息脱敏、panic/错误回滚、context 约束 | 已完成 |
| `DB-006` | SQLite 与公开 API 契约 | 权限、pragma、单连接内存库和外部包测试通过 | 已完成 |
| `DB-007` | 三数据库 CI | SQLite 门禁与 PostgreSQL/MySQL service contract 定义完成 | 已完成 |
| `DOC-001` | 权威文档与 010 记录 | 配置、README、边界、四件套与导航同步 | 已完成 |
| `VER-001` | 全量验证与提交审阅 | 本地门禁通过；真实未执行项明确；创建独立 commit | 已完成 |

## 3. 完成证据

- `internal/kernel/app/database.build` 明确调用 `database.NewGORM`；配置只含 Driver、DSN、Pool 和 Ping。
- Engine、SQLX/sqldb、旧 gormdb 公开入口、测试和不再使用的直接依赖已经删除，没有兼容别名。
- Resource 和 Client 是不同动态对象；Kernel Access 发布 Borrowed Client，并验证逃逸 Client、Repository、Tx 统一失效。
- Schema 在 DDL 前验证字段、Default、索引、外键和受管字段；SQLite 真连接证明迁移只添加缺失列、不修改既有列。
- Repository 覆盖 Create、First、Find、Count、Update、SoftDelete、事务重绑、分页、乐观锁和无条件修改保护。
- 驱动错误树经过脱敏；事务错误、panic、根 context 取消和 Resource 关闭均有回归测试。
- `pkg/database/public_test.go` 仅使用导出 API 完成迁移、CRUD、事务和关闭权隔离验证。
- `.github/workflows/database.yml` 定义 SQLite 全门禁、PostgreSQL 18 和 MySQL 8.4 真连接契约。

## 4. 验证结果

以下检查在本任务工作区实际通过：

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go mod tidy -diff`
- `CGO_ENABLED=0 go build ./...`
- Database CI YAML 解析
- Markdown 相对链接检查
- 公开 API、GORM 边界和旧轨残留搜索
- `git diff --check`

## 5. 未执行项与剩余风险

- 当前机器未配置 `TEST_DATABASE_POSTGRES_DSN` 或 `TEST_DATABASE_MYSQL_DSN`。
- Docker、Podman 不可用，127.0.0.1:5432 和 127.0.0.1:3306 没有服务监听。
- 因此 PostgreSQL/MySQL 真连接契约未在本机执行；CI 工作流已经把两项设为强制任务，但工作流在本任务提交前尚未远端运行。
- “已完成”表示 010 方案、实现、本地验证、CI 门禁与交付记录完成，不表示上述两个未执行的远端数据库契约已经通过。

## 6. Commit

- Commit：以承载本记录的 `010` 最终提交为准；短哈希和标题在交付报告中记录。
- Push：未执行。
