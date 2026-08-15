# 026 研究档案

本目录记录 Handler-first HTTP 路由绑定计划所依据的当前代码与外部工具证据。

## 检索与复核

研究前已检索 `docs/**/research/**/metadata.yaml`。025-R001 可复用“模块拥有手写 HTTP Adapter、OpenAPI 保持应用级 authority”的结论，但它明确不适用于第二个 HTTP 业务模块，并把该场景列为刷新触发器；因此 026 新增独立研究，不改写 025 历史。

## 记录

- [R001 当前 Handler 与 route binding 耦合](R001-current-handler-route-coupling/report.md)：复核 HEAD `a42703f` 的真实构造链、单模块可用性和第二模块扩展摩擦。
- [R002 oapi-codegen 路由分区能力](R002-oapi-codegen-route-partitioning/report.md)：复核仓库固定的 `oapi-codegen v2.8.0` 官方能力，并比较静态聚合、按 tag 分包和运行时路由注册。

两项研究均只授权形成计划，不代表 026 已获实施确认。
