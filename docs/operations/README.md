# 交付与运维

本目录是当前构建、容器、迁移、发布、复制和安全运维的唯一使用入口。`docs/changes/024-production-ready-one-shot-completion` 保存施工证据，不替代这里的现行操作说明。

- [构建与容器](build-and-container.md)
- [数据库迁移与回滚](migration-and-rollback.md)
- [本地候选与正式发布](release.md)
- [复制为独立项目](copying.md)
- [安全响应](security.md)
- [定时任务运维](scheduled-tasks.md)

所有命令都从仓库根目录运行。Linux、容器、PostgreSQL/MySQL 和 keyless release 的最终证据来自 CI；没有对应日志时不得用 cross-build 或未运行的 workflow 代替。
