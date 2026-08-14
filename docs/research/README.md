# 研究档案与报告

本目录保存面向架构判断的阶段性研究。研究报告描述特定时间点的仓库事实、外部样本和推导结论，不替代根 [README](../../README.md) 或主题文档中的当前实现说明，也不等同于已经确认的实施方案。

## 任务研究门禁

所有仓库任务先研究、再计划、后实现。完整门禁以根 [AGENTS.md](../../AGENTS.md) 为准；本节统一研究档案的检索、记录和复用格式。

开始研究时先执行以下步骤：

1. 明确需要回答的问题和完成计划所缺少的事实，不先选方案再寻找佐证。
2. 使用 `rg --files docs -g metadata.yaml` 列出既有档案，再按 question、topic、keyword、technology 和 scenario 检索 metadata。
3. 检查命中记录的状态、证据快照、验证日期、适用边界、刷新触发器和替代关系；只有仍适用的结论才能直接复用。
4. 内部研究从代码、配置、测试、运行入口和 Git 事实出发；外部研究优先官方源码、官方文档、标准、官方示例和测试，二手材料只作为导航。
5. 打开命中记录的 `report.md` 核对完整证据，不只引用 metadata 的一行摘要。易漂移或会决定当前计划的事实必须重新验证。

新增应用模块、通用能力或外部系统接入时，还必须按 [应用模块开发指南](../development/application-module-development.md) 完成能力评估。研究报告必须显式说明真实用例和外部副作用、现有 Capability 复用、新 Capability 与第三方边界、资源/运行/配置 owner、生命周期与 Reload 分类、当前契约适配性，以及 composition、Host、入口、迁移和验证影响。结论为“无新能力”时也要列出核对证据；当前契约无法表达真实需求时，研究门禁不得通过。

每个 `docs/changes/<seq-num-name>/research/` 至少包含一个 `Rxxx-<semantic-name>/`，其中固定包含 `metadata.yaml` 和 `report.md`。任务级 `research/README.md` 负责说明研究范围、检索方式和记录索引。

`metadata.yaml` 使用以下最小结构；任务可以增加语言、框架、技术等检索字段，但不得删除这些语义：

```yaml
schema_version: 1
id: R001
title: "研究标题"
question: "需要回答的问题"
research_type: current-project
topics: ["主题"]
keywords: ["检索词"]
applicable_scenarios: ["适用场景"]
non_applicable_scenarios: ["不适用场景"]
status: active
summary: "一行结论"
evidence:
  types: ["source-code", "tests"]
  primary_sources:
    - title: "来源"
      locator: "仓库路径或 URL"
      snapshot: "Commit、版本或日期"
researched_at: "YYYY-MM-DD"
verified_at: "YYYY-MM-DD"
validity:
  mode: snapshot
  review_after: "YYYY-MM-DD"
  refresh_triggers: ["需要重新研究的条件"]
relations:
  extends: []
  related: []
  supersedes: []
  superseded_by: []
report: report.md
```

`report.md` 必须说明研究问题、方法与范围、证据、当前事实、推断或比较、适用与不适用场景、局限、剩余未知和对当前任务的影响。事实、推断、用户决策和目标设计必须分别标识；框架能力不能写成项目已实现能力。

研究状态使用 `draft`、`active`、`partial`、`needs-refresh`、`superseded`、`rejected`。过时记录不删除；新记录通过 `supersedes` 与旧记录的 `superseded_by` 双向关联，形成单轨当前结论。研究门禁通过只表示证据足以形成计划，不表示计划已确认或代码已经授权实施。

## 报告

- [001 Go 脚手架底层能力装配架构对比](001-go-capability-composition/README.md)：比较当前 Kernel Capability 模型与 Kratos/Wire、go-zero、Uber Fx、go-clean-template 的装配方式，并给出分轨演进建议。
- [002 Kernel 底层组件手动装配与安全重载](002-kernel-app-manual-composition/README.md)：解释当前扩展路径为何复杂，提出所有底层能力统一进入 `pkg -> kernel/app -> composition -> Kernel/Host`、按 `Fixed/Configured` 与 `Direct/Leased` 选择治理强度的多态装配模型，以及组件级安全重载目标；不提前设计尚未建设的业务层。
- [012 业务模块架构研究档案](../changes/012-business-module-architecture/research/README.md)：任务级、可检索的结构化研究档案，覆盖当前仓库、手工/生成/运行时装配、模块化单体、分布式组件和插件机制；其中底层闭环已实施，业务详细设计仍被真实用例阻塞。
- [013 研究优先任务门禁](../changes/013-research-plan-implementation-gate/research/README.md)：从 012 提炼可复用的结构化研究方法，并审计原“计划 -> 确认 -> 实施”流程缺少独立研究门禁的问题。

项目级研究使用递增三位序号和语义名称建立独立目录；任务级研究在所属变更的 `research/` 下使用 `Rxxx`。两者都必须说明研究快照、样本范围、事实与推断边界、适用性、时效和来源。
