# 017 应用模块能力评估门禁

## 状态

- 当前阶段：已完成。
- 研究门禁：已通过，见 [R001](research/R001-current-module-capability-assessment/report.md)。
- 当前授权：用户在计划报告后的后续消息中明确确认“实施 017 当前方案”，授权 `AGT-001` 至 `VER-001`。
- 变更性质：本轮只实施文档与规则调整，不修改 Kernel 运行时代码，不新增具体底层能力。

## 实施结果

- `AGENTS.md` 已增加通用研究语境，只要求按项目文档评估能力、资源、生命周期和契约适配性。
- [应用模块开发指南](../../development/application-module-development.md) 已成为当前项目的唯一开发流程入口。
- 根 README、项目文档索引、模块说明、研究规范和 Kernel App 组件说明已形成单轨导航。
- 指南已覆盖消息、邮件和搜索示例，并明确当前没有选择或实现对应通用 Capability。
- 文档结构、相对链接、通用规则边界、Kernel 形态一致性和 `git diff --check` 已通过验证。

## 目标

让 AI Agent 在收到“新增应用模块”请求时，必须先显式判断现有能力能否满足需求、是否需要新的底层 Capability、资源与长期任务由谁治理、当前生命周期契约是否适用，以及何时应停止实现并转入独立研究或 ADR。

`AGENTS.md` 只保留跨项目可复用的研究语境和导航责任；`go-scaffold2` 的 Capability 分类、Kernel 组件形态、生命周期策略和实施步骤由项目当前权威文档负责。

## 阅读顺序

1. [研究索引](research/README.md)
2. [R001 当前应用模块能力评估缺口](research/R001-current-module-capability-assessment/report.md)
3. [需求](requirements.md)
4. [设计](design.md)
5. [任务](tasks.md)

## 范围摘要

- 为 `AGENTS.md` 设计一条通用的“模块、通用能力或外部系统接入”研究导航规则。
- 建立当前项目的应用模块开发权威入口和必答能力评估表。
- 从根 README、文档索引、模块目录说明、Kernel App 组件说明和研究规范形成单轨导航。
- 明确消息、邮件、搜索等典型能力的边界判断示例，但不预选具体第三方实现。
- 不把项目特有的 `Kernel`、`app.Value`、`KernelInstanceSwap`、目录清单或验收细节堆入 `AGENTS.md`。
