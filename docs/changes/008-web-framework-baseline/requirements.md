# 产品需求：Web 框架基线建设

## 1. 文档性质

本文件保存 008 原始规划的历史范围，不是当前实现说明。Database 需求已经迁入并完成于 [010](../010-database-gorm-boundary/requirements.md)；其余需求处于暂停状态，恢复时必须重新核对和确认。

## 2. 原规划范围

- Kernel 支持受监督的长期 Runner 与明确的启动、取消和反向停止顺序。
- 建立静态生成的 Plugin Manifest、Factory、Access 和 registry，不在运行时扫描。
- 建设 Web Kernel App、稳定插件代理、`/livez`、`/readyz` 和安全中间件。
- 建设 Password、Mail 项目能力，并保持第三方类型不进入公开签名。
- 建设 `user` 插件纵切，包括注册验证、会话、RBAC、密码重置和软删除。
- 建立跨平台、生成器、安全和端到端验证门禁。

## 3. 当前归属

- Database 单轨 GORM、Schema、Repository、事务和三数据库 CI：由 010 完成。
- 配置重载与生命周期修复：由 [009](../009-config-reload-lifecycle-repair/README.md) 独立完成。
- Web、Plugin、Runner、Password、Mail、User：未实施，不得写成当前能力。

## 4. 恢复门禁

若以后恢复任一剩余主题，应按语义拆成新的递增任务，重新记录当前事实、需求、设计、验收和副作用，并取得实施确认；不以 008 的历史确认跨任务授权。
