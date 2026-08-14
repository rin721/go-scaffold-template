# 018 研究档案

> 018 方案已于 2026-08-15 废除。下列档案只保留固定快照和历史推理，不再支撑任何当前实施计划；当前方向见 [019 HTTP API 成熟度缺口评估](../../019-http-api-maturity-gap-assessment/README.md)。

## 研究问题

1. 当前项目距离成熟的积木式插件框架还缺少哪些可验证契约？
2. DeepSeek Harness 所称“Everything is a Plugin”在代码和 Cordis 论文中具体意味着什么？
3. 哪些机制适合 Go，哪些机制会破坏项目现有的类型安全、显式依赖和资源治理？
4. 如何在不一次性引入运行期动态装载的前提下，形成可渐进实施的目标架构？

## 档案

- [R001 当前装配与生命周期基线](R001-current-composition-baseline/report.md)：核验当前 `Definition/Plan/Kernel/Contribution/composition` 的事实、可复用基础与缺口。
- [R002 DeepSeek Harness/Cordis](R002-deepseek-harness-cordis/report.md)：基于固定 Commit、vendored Cordis 源码、官方架构文档、论文与 Go 官方插件文档提取可迁移原则。

两份记录共同支撑 [需求](../requirements.md) 与 [设计](../design.md)。研究门禁通过只表示证据足以形成计划，不表示非文档实现已获确认。
