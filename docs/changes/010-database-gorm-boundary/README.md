# 010 数据库单轨 GORM 与稳定访问边界

## 状态

- 当前状态：**已完成**。
- 完成日期：2026-08-14。
- 最终提交父基线：`main@d8ba97231f20ce7391469ebe508c5e16b7a508f7`。
- 远端刷新结果：`origin/main@139d437e4407583f6a71afd17808e149a9663d72`；本任务不 push、不 rebase、不改写既有 `009` 提交。
- 用户将已实施的数据库工作明确定位为 `010`，并要求落实方案、记录已完成。

## 结果

本任务把数据库实现收敛为代码明确选择的 GORM 单轨：

- `internal/kernel/app/database` 直接调用 `pkg/database.NewGORM`，配置只选择 SQLite、PostgreSQL 或 MySQL 协议与连接参数；
- 上层只使用项目自有 `Schema`、`BaseRepository`、`Client`、`Tx` 和 Kernel `Access`，不接触 GORM、dialector 或 session；
- 共享连接池关闭权只属于 `Resource`，租约回调之外的 Client、Repository 和 Tx 会失效；
- Schema 只执行 additive migration，并在 DDL 前校验字段、默认值、索引和外键关系；
- SQLite、本地公开 API、事务、错误和生命周期验证已经通过，PostgreSQL/MySQL 真连接由 CI 工作流承接。

## 阅读顺序

1. [requirements.md](requirements.md)：目标、范围、约束和验收标准。
2. [design.md](design.md)：公开契约、数据流、失败语义和文件影响。
3. [tasks.md](tasks.md)：任务状态、逐轮证据、验证结果和剩余风险。

当前使用方式以 [pkg/database/README.md](../../../pkg/database/README.md) 为准；本目录只保存本次变更证据，不作为第二套现行规范。
