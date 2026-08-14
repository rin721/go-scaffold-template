# 任务计划：业务模块架构

## 1. 当前门禁

- 012 当前状态：**待确认**。
- 本轮授权：仅研究与文档；不得修改源码、配置、脚本、依赖、测试或生成物，不得启动服务、暂存、提交或 push。
- 后续实施：用户必须在阅读本方案报告后的新消息中明确确认 012 和当前任务范围。
- 首个业务切片：还必须提供真实用例；确认架构不等于授权 Agent 虚构业务。

任务状态含义：`已完成（文档）` 只表示本轮文档证据完成；`待确认` 表示尚未获实施授权；`阻塞` 表示存在必须由真实需求或外部事实回答的问题。

## 2. 总体批次

| 批次 | 目标 | 任务文件 | 前置 | 当前状态 |
|---|---|---|---|---|
| DOC | 事实、研究、需求、设计和任务归档 | 本文件 | 无 | 已完成（文档） |
| FND | 单快照、contribution、运行模式、HTTP lifecycle | [foundation.md](tasks/foundation.md) | 012 明确确认 | 待确认 |
| VSL | 首个真实业务垂直切片 | [first-vertical-slice.md](tasks/first-vertical-slice.md) | FND + 真实用例确认 | 阻塞 |
| GOV | 架构门禁、文档同步和完整验证 | [governance-and-verification.md](tasks/governance-and-verification.md) | FND/VSL | 待确认 |

## 3. 本轮文档任务

| ID | 工作量 | 任务 | 完成条件 | 状态 | 证据 |
|---|---:|---|---|---|---|
| DOC-001 | M | 当前代码与文档取证 | 启动、Kernel、Host、Capability、HTTP/CLI/DB/Cache 事实与缺口可追溯 | 已完成（文档） | `requirements/current-facts-and-gaps.md`、R001 |
| DOC-002 | L | 外部代表项目源码研究 | 至少覆盖手工/生成/runtime DI/HTTP/参考架构/平台/分布式/插件 | 已完成（文档） | R002-R009 |
| DOC-003 | M | 综合比较与目标设计 | 明确选择、拒绝项、边界、生命周期和迁移 | 已完成（文档） | `design.md`、R010 |
| DOC-004 | M | 研究档案规范与任务拆分 | metadata schema、检索/更新规则、稳定任务 ID 完整 | 已完成（文档） | `research/README.md`、本目录 |
| DOC-005 | S | 文档验证与范围审阅 | metadata 可解析、链接存在、Diff 无空白错误、仅授权文档变化 | 已完成（文档） | 10 份 metadata 完整 schema、29 份 Markdown 本地链接、`git diff --check` 与状态审阅通过 |

## 4. 实施依赖图

```text
FND-001 single snapshot ─┬─> FND-004 application composition ─┐
FND-002 contributions ──┤                                    │
FND-003 HTTP lifecycle ─┘                                    ├─> VSL-001..007 -> GOV-001..004
FND-005 CLI modes ────────────────────────────────────────────┘
```

FND 任务可在设计层并行分析，但涉及同一入口、Kernel/Host API 或公共契约时必须按实际 Diff 串行落地并重新审阅。不得为了并行保留新旧两条启动路径。

## 5. 逐轮证据

### Round 1：2026-08-14，方案阶段

- 基线：`main@2daf47a`；任务开始前存在用户未跟踪 `tmp/`，未触碰。
- 完成：本地静态取证、九类外部样本研究、综合比较、目标设计、模块指南和任务拆分。
- 未执行：任何源码/配置/测试/依赖修改；服务启动；数据库/外部系统操作；Git stage/commit/push。
- 验证：10 份 `metadata.yaml` 均通过 YAML 解析、必填字段/枚举/唯一 ID/关系引用/report 存在性检查；29 份本任务及导航 Markdown 的本地链接检查通过；`git diff --check` 通过（仅提示 Windows 后续可能转换两个导航文件的行尾）；Git 状态确认只有本任务文档导航、012 文档目录和预存 `tmp/`。
- 未运行 Go build/test：本轮没有实现变更，仓库规则要求只执行适用的文档验证；不把前期只读包扫描描述为测试通过。
- Commit：无；方案阶段禁止暂存或提交。

### Round 2：2026-08-14，方案发布

- 授权：用户在方案报告后的独立消息中明确要求为当前 012 方案创建规范分支、提交并 push；该授权只覆盖文档发布，不确认 FND、VSL 或 GOV 实施。
- 分支：`codex/012-business-module-architecture-plan`，从已 fetch 且与本地一致的 `origin/main@2daf47a` 创建。
- 范围：012 的 37 个任务文件与 2 个必要导航文件；预存 `tmp/` 明确排除且未触碰。
- 验证：10 份 metadata schema 与关系引用、29 份 Markdown 的 61 个相对链接、敏感信息扫描、完整暂存 Diff 范围和 `git diff --cached --check` 通过；仅文档变更，未运行 Go build/test。
- Commit：本次文档提交。

## 6. 完成与提交原则

后续实施完成不等于只通过单元测试。必须满足 [acceptance-matrix.md](requirements/acceptance-matrix.md)，同步当前权威文档，搜索并删除失效入口，审阅完整 Diff，并只提交 012 已确认任务文件。按仓库规则，确认后任务文档、实现、测试和权威文档应形成同一任务提交；除非用户明确要求不提交。

方案发布提交不等于后续实施提交，也不改变“待确认”状态。`tmp/` 和任何无法确认归属的改动永远不进入 012 暂存范围。
