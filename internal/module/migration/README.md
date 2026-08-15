# Migration 模块

`internal/module/migration` 只编排显式 `status/up` 用例与 CLI contract，不拥有 Todo 表或业务 SQL。通用 golang-migrate Adapter 位于 `pkg/database/migrate`；Todo 三 driver SQL、checksum、legacy owner completion 与 Service readiness 位于 `internal/module/todo/binding/migration`。

```text
cmd/app db migrate
  -> internal/module/migration
  -> pkg/database/migrate
  -> internal/module/todo/binding/migration.Set/Completion
```

`status` 不执行 DDL。`up` 使用 invocation-owned 独立连接、锁等待和总操作期限，只允许前滚；不提供 `down`、`force` 或自动 repair。Service 启动只读校验 exact version、dirty 与 Todo owner completion，不能替 migration command 改 schema。

本地首次启动命令关系见 [本地启动指南](../../../docs/getting-started/local-development.md)；部署、回滚和失败处理见 [数据库迁移与回滚](../../../docs/operations/migration-and-rollback.md)。
