# 040 项目文档体系系统重构需求

## 目标

建立一套以当前真实代码、架构和实际能力为准的项目文档体系，消除重复描述、结构不一致、历史方案与现行能力并列、代码已有能力无正式入口、文档声明能力不存在等问题。

阅读路径必须按项目自然生命周期连续展开：

```text
认识项目 -> 启动项目 -> 使用能力 -> 开发业务 -> 接入基础设施
-> 理解架构 -> 扩展能力 -> 调试排障 -> 运行维护 -> 深入底层设计
```

## 功能要求

| ID | 要求 |
| --- | --- |
| REQ-001 | 新建 `docs/changes/040-documentation-system-rebuild/`，记录研究、需求、设计、任务和验证证据；039 保留为轻量入口整理历史。 |
| REQ-002 | 审计根 README、`docs/**`、`api/README.md`、`internal/**/README.md`、`pkg/**/README.md`，建立可复核文档审计矩阵。 |
| REQ-003 | 标记概念多定义、描述冲突、已落地能力仍被写成未来规划、旧方案并列、代码能力缺失正式文档、文档声明不存在能力、局部 README 承载项目级公共知识等问题。 |
| REQ-004 | 根 README 只承担项目自然入口、项目概览、最短启动路径和文档体系入口。 |
| REQ-005 | `docs/README.md` 作为项目手册总目录，按真实使用路径连续展开，不按读者身份分流。 |
| REQ-006 | `docs/architecture/README.md` 收口 composition、Application Generation、Kernel App、module boundary、pkg capability 和生命周期治理。 |
| REQ-007 | `docs/development/README.md` 收口业务模块开发、Binding 契约、execution、schedule、messaging、logging 和 API contract。 |
| REQ-008 | `docs/operations/README.md` 收口构建、迁移、发布、复制、安全、调度、消息、排障和运行维护。 |
| REQ-009 | 项目级公共知识只保留一个正式 authority，局部 README 只保留包/模块局部实现细节并链接权威入口。 |
| REQ-010 | 明确 `docs/changes/**` 与 `docs/research/**` 的历史边界，避免历史任务或研究快照成为第二套当前规范。 |
| REQ-011 | 建立后续维护约束：新增或修改能力时同步更新对应权威文档，禁止在就近 README 随意新增重复项目级说明。 |

## 非目标

- 不修改代码 API、CLI、配置格式、OpenAPI、数据库迁移、脚本或运行行为。
- 不大规模移动历史 `docs/changes/**`，避免破坏历史证据链接。
- 不把所有历史任务记录改写成当前文档。
- 不运行 Go 测试、构建或服务启动；本任务不声明运行时验证通过。

## 验收标准

- 040 任务账本包含研究、需求、设计、任务和验证证据。
- 正式入口文档职责清晰，互相可达，不重复定义项目级公共契约。
- `docs/README.md` 形成连续阅读路径，不出现按身份分流的导航。
- 包级和模块级 README 不再承担项目级 authority，缺失局部入口得到补齐。
- Markdown 本地链接检查覆盖本轮新增与修改 Markdown，并尽量覆盖全仓 Markdown。
- 一致性扫查覆盖旧方案、未来规划、待确认、已废除、唯一权威、当前权威等关键词，正式文档不把历史方案写成当前规范。
- 代码事实抽样核对覆盖核心能力、路径、配置节、composition 入口、Binding 和模块能力。
- `git diff --check` 通过。
