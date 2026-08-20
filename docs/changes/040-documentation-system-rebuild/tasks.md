# 040 任务清单

## 1. 当前状态

- 研究门禁：已通过（`DOC-R01`）。
- 计划状态：已确认。
- 实施类型：纯文档实施。
- 非文档变更：无。

## 2. 研究与计划证据

| ID | 工作量 | 状态 | 完成条件 | 证据 |
| --- | --- | --- | --- | --- |
| DOC-R01 | M | 完成 | 审计正式文档、历史证据和局部 README，识别 authority、缺口和代码事实冲突 | `research/R001-documentation-system-audit/metadata.yaml`、`report.md`、`audit-matrix.md` |
| DOC-P01 | M | 完成 | 形成连续项目手册、主题 authority 和维护约束设计 | `requirements.md`、`design.md`、本文件 |

## 3. 实施任务

| ID | 工作量 | 状态 | 完成条件 | 证据 |
| --- | --- | --- | --- | --- |
| DOC-001 | M | 完成 | `docs/README.md` 按真实使用路径连续展开，不按身份分流 | `docs/README.md` |
| DOC-002 | M | 完成 | architecture/development/operations 三个入口分别收口架构、开发和运维主题 | `docs/architecture/README.md`、`docs/development/README.md`、`docs/operations/README.md` |
| DOC-003 | S | 完成 | pkg 能力清单补齐 `execution` 局部入口，包级 README 明确不承担项目级 authority | `pkg/README.md`、`pkg/execution/README.md` |
| DOC-004 | S | 完成 | Kernel 局部 README 与当前 Application Generation、业务模块现实一致，不保留“尚无业务层”的失效说明 | `internal/kernel/README.md`、`internal/kernel/app/README.md` |
| DOC-005 | S | 完成 | 040 任务账本完整并更新变更索引 | `docs/changes/040-documentation-system-rebuild/**`、`docs/changes/README.md` |
| DOC-006 | S | 完成 | Markdown 链接、一致性扫查、代码事实抽样和空白检查通过或记录限制 | 见第 4 节 |

## 4. 验证证据

| 日期 | 范围 | 证据 | 结论 |
| --- | --- | --- | --- |
| 2026-08-20 | Markdown 本地链接 | 本轮新增与修改 Markdown 的相对链接检查通过 | 完成 |
| 2026-08-20 | 文档一致性扫查 | 搜索旧方案、未来规划、待确认、已废除、唯一权威、当前权威、目标设计、尚未、未验证；正式文档保留必要边界说明，历史命中仅作为证据保留 | 完成 |
| 2026-08-20 | 代码事实抽样 | 静态核对 `cmd/app`、`internal/composition`、`internal/module/contracts.go`、`internal/kernel/app/*`、`pkg/*` 与正式文档核心声明 | 完成 |
| 2026-08-20 | 空白与冲突标记 | `git diff --check` 通过 | 完成 |
| 2026-08-20 | 变更范围 | diff 审阅显示仅 Markdown 文档变更 | 完成 |

本文档任务未运行 Go 测试、构建或服务启动。
