# 研究档案

## 1. 目的与边界

本目录保存 012 方案使用的可复核证据。研究档案用于回答“某个结论依据什么、适用于什么场景、何时需要刷新”，不是当前架构的第二套权威说明。当前实现以代码和主题文档为准，目标决策以本变更的 `design/` 为准。

仓库已有 [Go 能力装配研究](../../../research/001-go-capability-composition/README.md) 和 [Kernel App 手工装配研究](../../../research/002-kernel-app-manual-composition/README.md)。本次不复制其全文，而是把相关结论纳入带 metadata 的任务级索引；后续可另行治理旧研究目录，但不在 012 范围内迁移历史文件。

开始本次研究前已使用 `rg --files docs -g metadata.yaml` 搜索仓库，未发现既有结构化 metadata；因此引用上述两份可复用的非结构化研究，同时为本次独立问题建立 R001-R010，不伪造对旧文件的替代关系。

## 2. 记录格式

每个研究目录固定包含：

- `metadata.yaml`：供人和工具检索的结构化元数据；
- `report.md`：问题、证据、事实、适用性、局限和对 012 的影响。

`metadata.yaml` schema version 1：

```yaml
schema_version: 1
id: R001
title: "研究标题"
question: "需要回答的问题"
research_type: current-project
topics: ["composition"]
keywords: ["composition root"]
languages: ["Go"]
frameworks: []
technologies: []
applicable_scenarios: []
non_applicable_scenarios: []
status: active
summary: "一行结论"
evidence:
  types: ["source-code", "official-docs"]
  primary_sources:
    - title: "来源"
      locator: "路径或 URL"
      snapshot: "commit、tag 或日期"
researched_at: "2026-08-14"
verified_at: "2026-08-14"
validity:
  mode: snapshot
  review_after: "2027-02-14"
  refresh_triggers: ["上游主要版本变化"]
relations:
  extends: []
  related: []
  supersedes: []
  superseded_by: []
report: report.md
```

## 3. 枚举与状态演进

- `research_type`：`current-project`、`framework`、`reference-architecture`、`runtime-platform`、`plugin-system`、`comparative`。
- `evidence.types`：`source-code`、`official-docs`、`examples`、`tests`、`local-static-analysis`。
- `validity.mode`：`snapshot` 表示结论绑定版本；`durable-principle` 表示原理较稳定，但仍需按触发器复核。
- `status`：`draft`、`active`、`partial`、`needs-refresh`、`superseded`、`rejected`。

状态只按以下方向演进：

```text
draft -> active | partial
active | partial -> needs-refresh
needs-refresh -> active | partial（复核后）
any -> superseded | rejected
```

研究记录不因过时而删除。替代记录通过 `supersedes`/`superseded_by` 双向关联；`rejected` 仅表示不适合当前决策，不表示上游技术无价值。

## 4. 检索方法

1. 先列出 metadata：`rg --files docs -g metadata.yaml`。
2. 按 question/topic/keyword/framework/scenario 搜索：`rg -n -i "关键词" docs -g metadata.yaml`。
3. 检查 `status`、`verified_at`、snapshot、`review_after` 和 refresh trigger。
4. 只打开命中的 `report.md`，核对事实与适用性；不要只使用 summary。
5. 需要做当前决策时重新验证易漂移事实，并在新记录中声明关系。

## 5. 本次索引

| ID | 主题 | 类型 | 结论用途 |
|---|---|---|---|
| [R001](R001-current-project-facts/report.md) | 当前仓库事实 | current-project | 决定能复用什么、必须先补什么 |
| [R002](R002-kratos-wire/report.md) | Kratos + Wire | framework | 静态装配、分层和 server lifecycle 参照 |
| [R003](R003-go-zero/report.md) | go-zero | framework | 生成式 Handler/Logic/ServiceContext 取舍 |
| [R004](R004-uber-fx/report.md) | Uber Fx | framework | runtime DI、Module 与 lifecycle 对照 |
| [R005](R005-cloudwego-hertz/report.md) | CloudWeGo Hertz | framework | HTTP 路由与 shutdown 参照 |
| [R006](R006-wild-workouts/report.md) | Wild Workouts | reference-architecture | port/adapter、Repository 和事务闭包 |
| [R007](R007-encore/report.md) | Encore | runtime-platform | 编译器发现资源与服务图的成本 |
| [R008](R008-dapr/report.md) | Dapr | runtime-platform | 进程外 building blocks 的适用边界 |
| [R009](R009-hashicorp-go-plugin/report.md) | go-plugin / Mattermost | plugin-system | 真实插件协议与 host 成本 |
| [R010](R010-comparative-synthesis/report.md) | 综合比较 | comparative | 形成 012 推荐方案与拒绝项 |

## 6. 复用规则

- 新任务可以按 metadata 找到并引用本记录，但必须检查 version/日期和适用场景。
- 如果只需同一问题的新版本验证，创建新记录并 `supersedes` 旧记录；不要直接抹掉旧快照。
- 报告中的框架能力不能直接写成项目已实现事实。
- 外部来源优先使用官方源码、官方文档、官方示例和测试；二手文章只能作为导航，不作为关键结论唯一依据。
