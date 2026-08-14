# 任务：仓库身份迁移

## 1. 门禁状态

- 研究门禁：已通过。
- 计划状态：用户已明确确认，021 实施与验证已完成。
- 020 状态：已先行完成于 `cc20b62`；021 保留其改名前 `bba1802` 隔离证据，不重复验证或重写。
- 实施范围：已完成 `REMOTE-001` 至 `COMMIT-001`；未 push、tag、release、修改 GitHub 设置或本地父目录。

## 2. 已完成任务

| ID | 工作量 | 依赖 | 内容 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `RES-001` | M | 无 | 核验 GitHub canonical/redirect、独立相邻仓库、本地 remote、module、运行品牌、020 隔离证据和文档边界 | R001 事实与推断分离，冲突证据已解释 | 已完成 |
| `RES-002` | S | `RES-001` | 按扩展名和所有者清点旧 identity，复核 CI、Agent 与脚本 | 当前实现、文档、历史和无需修改的交付文件边界明确 | 已完成 |
| `PLAN-001` | M | `RES-001` | 形成单轨 identity 映射、文件边界、失败语义和验证方案 | requirements/design/tasks 一致且可执行 | 已完成 |

## 3. 已确认并完成的实施任务

| ID | 工作量 | 依赖 | 内容 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `REMOTE-001` | S | 用户确认 | 验证 canonical URL，更新 `origin` 并 fetch/prune | fetch/push URL 均为新地址；远端分歧已核对 | 已完成 |
| `MODULE-001` | L | `REMOTE-001` | 迁移 `go.mod`、当前 Go imports、测试/架构常量和包示例 | 旧 module path 在当前实现与使用文档中归零 | 已完成 |
| `BRAND-001` | M | `MODULE-001` | 迁移应用名、入口测试、根/主题文档和配置示例标题 | 当前产品身份统一为 `go-scaffold-template` | 已完成 |
| `HISTORY-001` | M | `BRAND-001` | 分类历史记录，记录 020 完成结果与新 identity 关系 | 旧名称仅保留为可解释历史事实；020 既有证据不丢失 | 已完成 |
| `VERIFY-001` | L | `HISTORY-001` | 执行 remote、module、残留、Go、Markdown 和 Diff 验证 | requirements 的全部验收命令通过 | 已完成 |
| `COMMIT-001` | S | `VERIFY-001` | 仅暂存并提交本任务文件 | Conventional Commit 完成，不包含 `AGENTS.md`，不 push | 已完成，本次提交收口 |

## 4. 逐轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-15 | `RES-001`、`PLAN-001` | `main@bba1802`；GitHub 三个仓库页面；本地 remote/module/identity 扫描；020-R001、任务状态与隔离目录复核 | 未提交，计划阶段禁止暂存/提交 | 尚未 fetch、迁移 remote/module/品牌或运行 Go 验证 |
| 2 | 2026-08-15 | `RES-002` | 88 个 Go 文件、17 个 Markdown、`go.mod` 的完整 module path 分类；两个拆分 boundary prefix；CI/Agent/脚本无命中 | 未提交，计划阶段禁止暂存/提交 | 文件清单尚未实施，远端状态仍未 fetch |
| 3 | 2026-08-15 | `REMOTE-001`、`MODULE-001`、`BRAND-001`、`HISTORY-001`、`VERIFY-001`、`COMMIT-001` | [R002](research/R002-post-migration-verification/report.md)；canonical fetch；105 个 module identity 文件；117 个实现/当前文档 Diff 闭包；Go 与文档全门禁 | 本次提交 | 未 push；Linux 与正式 release baseline 仍不在 021 范围 |

## 5. 完成边界

021 已完成本地 canonical remote、Go module/import、运行品牌、架构门禁和当前使用文档迁移。旧 identity 只保留在明确历史与迁移证据中；push、tag、release、GitHub 设置、本地父目录改名、正式复制指南和 Linux baseline 均未执行。
