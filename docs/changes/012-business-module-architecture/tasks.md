# 任务计划：底层闭环与业务模块架构

## 1. 当前门禁

- 012 状态：**待确认**。
- 研究边界：只修改 012 方案文档、研究档案和必要导航；不得修改实现、启动或生成。Round 4 的独立后续授权仅允许暂存、提交并 push 这些文档。
- 实施授权：用户必须在本轮方案报告后的新消息中，明确确认当前基础闭环方案和任务范围。
- 业务实施：即使 FND/GOV 获确认，也不授权虚构业务；只有 `AC-BIZ-GATE-001` 通过并另行确认真实用例后才解锁 VSL。

状态说明：`已完成（文档）` 只代表研究/方案证据完成；`待确认` 代表没有实施授权；`阻塞` 代表前置验收或真实需求尚不存在。

## 2. 总体批次

| 批次 | 目标 | 前置 | 状态 |
|---|---|---|---|
| DOC | 深化当前闭环审计、外部研究和单轨方案 | 无 | 已完成（文档） |
| FND | 单候选、监督、HTTP、状态、重载/终止闭环 | 用户明确确认 012 当前方案 | 待确认 |
| GOV-F | 基础 package/注册/lifecycle 门禁和权威文档同步 | FND | 待确认 |
| BIZ-D | 以真实用例恢复业务模块详细设计 | `AC-BIZ-GATE-001` + 真实用例确认 | 阻塞 |
| VSL | 首个真实业务垂直切片 | BIZ-D 再确认 | 阻塞 |
| GOV-B | 业务边界门禁与最终验证 | VSL | 阻塞 |

详细任务见 [foundation.md](tasks/foundation.md)、[first-vertical-slice.md](tasks/first-vertical-slice.md) 和 [governance-and-verification.md](tasks/governance-and-verification.md)。

## 3. 本轮文档任务

| ID | 工作量 | 任务 | 完成条件 | 状态 | 证据 |
|---|---:|---|---|---|---|
| `DOC-001` | M | 复用研究 metadata | 先检索 R001-R010，标出仍有效/被替代记录 | 已完成（文档） | `research/README.md`、R001/R010 状态 |
| `DOC-002` | L | 全链路代码事实审计 | config 到 validation 每段 owner/state/failure 可追溯 | 已完成（文档） | R011、`current-facts-and-gaps.md` |
| `DOC-003` | L | 针对性外部研究 | HTTP/errgroup、supervision/state、reload、governance 有版本化主源 | 已完成（文档） | R012-R015 |
| `DOC-004` | M | 十一门禁综合与单轨设计 | 明确保留/补齐/拒绝、迁移和业务解锁条件 | 已完成（文档） | R016、requirements/design |
| `DOC-005` | M | 任务重排 | FND/GOV 先于业务，风险/未决/证据明确 | 已完成（文档） | 本文件及 tasks 子目录 |
| `DOC-006` | M | 本轮文档验证 | metadata/关系/链接/Diff/范围检查通过 | 已完成（文档） | Round 3 验证记录 |

## 4. 实施依赖图

```text
FND-001 lifecycle state/error semantics
   ├─> FND-002 Supervisor Start/Run/StopAndWait ─> FND-003 HTTP lifecycle ─┐
   ├─> FND-004 single immutable candidate ─> FND-005 reload/terminal ────┤
   └──────────────────────────────────────────────────────────────────────┤
                                                                          v
                    FND-006 application composition/modes -> FND-007 diagnostics
                                                                          |
                                                                          v
                         GOV-F-001..004 -> AC-BIZ-GATE-001
                                                  |
                                      real use case + reconfirmation
                                                  v
                                          BIZ-D -> VSL -> GOV-B
```

实施可调整小步次序，但不得让旧/新 Supervisor、两次 Loader 或两套 HTTP owner 长期并存。若公共 API、依赖、边界、迁移或副作用实质变化，任务状态回到待确认。

## 5. 逐轮证据

### Round 1：2026-08-14，原业务架构方案

- 完成 R001-R010 和首轮业务模块方向设计；代码未改。
- 局限：只确认 Kernel/业务图分离方向，未证明全进程生命周期闭环。

### Round 2：2026-08-14，原方案文档发布

- 分支 `codex/012-business-module-architecture-plan`，已发布文档 commit `110426434d3d62666e0969a5efba8c8a015eef7d`。
- 该提交只发布方案，不确认任何实现；预存 `tmp/` 未纳入。

### Round 3：2026-08-14，底层闭环复核

- 基线：实现仍为 `main@2daf47ad111141b27a1d8e100bb3d6e4cc1ea743`；当前工作树起始除未跟踪 `tmp/` 外无本轮改动。
- 完成：逐段审计 Loader/Plan/Kernel/Lease/Host/Supervisor/Watcher/httpx/health/tests；发现 HTTP Stop/Wait 互锁、runner nil/不合作语义、整份 candidate、terminal drain、cleanup degraded 和治理缺口。
- 外部主源：Go 1.25.7 net/http、x/sync v0.21.0、controller-runtime v0.24.1、dskit services、Caddy v2.11.2、Go module layout、Kubernetes import-boss v1.34.0。
- 单轨更新：R001/R010 标为 superseded，R011/R016 成为当前事实与综合结论；业务详细设计标记为冻结/阻塞。
- 验证：16 份 metadata 均通过 YAML、必填字段、枚举、唯一 ID、report 和 supersedes 双向关系检查；35 份 Markdown 的 62 个本地链接存在；新增/修改文档行尾空白检查、`git diff --check` 和 Git 范围检查通过。
- 范围：变更仅在 `docs/changes/012-business-module-architecture/` 与必要导航 `docs/changes/README.md`；预存 `tmp/` 仍未跟踪且未触碰。
- 未执行：任何实现修改、Go build/test、进程启动、生成、stage、commit、push 或外部写入。本轮只有文档，未用 Go 测试冒充文档验收。

### Round 4：2026-08-14，底层闭环方案发布

- 授权：用户在 Round 3 方案报告后的独立消息中明确要求提交并推送当前分支；该授权只覆盖本轮文档发布，不确认 FND、GOV-F、业务设计或实现。
- 远端预检：`git fetch origin` 成功；当前分支 `codex/012-business-module-architecture-plan` 与同名远端分支均位于 `110426434d3d62666e0969a5efba8c8a015eef7d`，左右差异为 `0/0`。
- 范围：只包含 012 研究、需求、设计、验收、任务和必要导航；预存 `tmp/` 明确排除。
- 验证：16 份 metadata、35 份 Markdown、62 个本地链接、行尾空白、疑似凭据赋值、完整暂存 Diff 和 `git diff --cached --check` 均通过；本轮只有文档，未运行 Go build/test。
- Commit/Push：以本轮 Git 操作结果和最终交付报告为准，不在创建前预写成功。

## 6. 完成与提交原则

实施完成必须逐项关闭 [acceptance-matrix.md](requirements/acceptance-matrix.md)，同步根权威文档，搜索删除旧入口，审阅完整 Diff，并只纳入已确认任务。Round 4 的 Git 授权只发布当前文档；后续即使用户确认实施，也只能覆盖确认的 FND/GOV-F 任务，不能顺带进入业务实现。
