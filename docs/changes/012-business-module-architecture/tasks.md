# 任务计划：底层闭环与业务模块架构

## 1. 当前门禁

- 012 状态：**FND/GOV-F 已实施并通过本地验证；BIZ-D/VSL 阻塞**。
- 实施授权：用户在 Round 5 方案报告后的独立消息中明确要求“开始实施 `012` 方案计划”。
- 实施范围：授权覆盖 FND-001..010 与 GOV-F-001..004；没有授权业务对象、业务协议或外部部署。
- 业务实施：即使 FND/GOV 获确认，也不授权虚构业务；只有 `AC-BIZ-GATE-001` 通过并另行确认真实用例后才解锁 VSL。

状态说明：`已完成（实现）` 代表代码、测试与权威文档已同步；`阻塞` 代表真实业务需求或后续确认尚不存在。

## 2. 总体批次

| 批次 | 目标 | 前置 | 状态 |
|---|---|---|---|
| DOC | 深化当前闭环审计、外部研究和单轨方案 | 无 | 已完成（文档） |
| FND | CLI/Config 契约、单候选、监督、HTTP、状态、重载/终止闭环 | 用户明确确认 012 当前方案 | 已完成（实现） |
| GOV-F | 基础 package/注册/lifecycle 门禁和权威文档同步 | FND | 已完成（实现） |
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
| `DOC-007` | L | 底层契约逐项清点 | BOOT/CLI/CFG/CMP/CAP/RES/LIFE/REL/OBS/Adapter 的 owner、调用方、状态和不变量完整 | 已完成（文档） | R017、契约清单 |
| `DOC-008` | L | CLI/Default/Config 针对性研究与单轨校正 | 外部主源、可选方案、收益/代价/迁移/验收齐全，R020 替代 R016 | 已完成（文档） | R018-R020、新设计文档 |
| `DOC-009` | M | 本轮文档验证 | R001-R020 metadata/关系、链接、术语、Diff 与范围检查通过 | 已完成（文档） | Round 5 验证记录 |

## 4. 实施依赖图

```text
[FND-008, FND-009] -> FND-010 default round-trip
[FND-001, FND-009, FND-010] -> FND-004 single candidate -> FND-005 reload/terminal/degraded
FND-001 lifecycle state/error -> FND-002 Supervisor -> FND-003 HTTP lifecycle
[FND-002, FND-003, FND-004, FND-005, FND-008, FND-010] -> FND-006 application composition
FND-006 -> FND-007 diagnostics -> GOV-F-001..004 -> AC-BIZ-GATE-001
AC-BIZ-GATE-001 + real use case + reconfirmation -> BIZ-D -> VSL -> GOV-B
```

实施已按依赖链单轨汇合：旧 weak binding、Kernel 自行 Load、组件 CLI contribution、旧 Supervisor 等待顺序和旧 HTTP Start/Shutdown 入口均已删除，没有保留双轨兼容层。

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
- Commit/Push：文档 commit `7eaba7174fb54c06c8bd31c7f7bc345d3f161936` 已发布到当前同名远端分支；它不确认任何实现。

### Round 5：2026-08-14，底层契约优先级校正

- 基线：当前分支 HEAD `7eaba7174fb54c06c8bd31c7f7bc345d3f161936`；实现事实仍取 `main@2daf47ad111141b27a1d8e100bb3d6e4cc1ea743`，预存未跟踪 `tmp/` 未触碰。
- 新事实：补审 `cmd/app`、`pkg/cli`、DefaultManager/File writer、Source/Decode/Snapshot、Plan/Compose、Kernel/Supervisor/Adapter 真实调用链与测试。
- 外部主源：Cobra v1.10.2、Kubernetes cli-runtime v0.34.1、mapstructure v2.5.0、Go `os`/`encoding/json`、Kubernetes v1.34.1 strict config/apimachinery v0.34.1；HTTP/Supervisor/Reload 复用 R012-R014。
- 单轨更新：新增 R017-R020，R020 替代 R016；新增统一契约清单、CLI/默认配置、Config 和运行状态机权威目标，业务细节继续阻塞。
- 范围：只修改 012 文档、研究档案和 `docs/changes/README.md`；没有修改源码、配置、依赖、脚本或测试，没有启动、生成、stage、commit、push。
- 验证：20 份 metadata 的 schema/枚举/唯一 ID/report/supersedes 双向关系通过；43 份 Markdown 的 96 个本地链接存在；契约总览 28 个 ID 与验收矩阵 46 个 ID 唯一；63 份 012 与必要导航 Markdown/YAML 的 UTF-8、行尾空白和末尾换行检查通过；`git diff --check`、凭据赋值扫描和 Git 范围检查通过。
- 未执行：Go build/test、进程启动、默认配置生成或其他实现门禁；本轮只验证文档，不把静态检查描述为目标能力已通过。

### Round 6：2026-08-14，FND/GOV-F 实施

- 授权：用户在 Round 5 方案报告后的独立消息中明确要求开始实施 012；范围限定为 FND-001..010 与 GOV-F-001..004。
- 单轨实现：新增 Bootstrap composition、严格 section registration、全应用 Coordinator、Supervisor runner readiness/异常完成/有界停止、HTTP Start/Run/Stop owner、Host health/diagnostics 与 package graph 门禁；删除组件 CLI contribution、服务 `Capabilities.CLI/Configuration`、Kernel 自行 Start/Reload Load 和旧 HTTP lifecycle 入口。
- 运行边界：默认 Service mode 启动 HTTP listener，但只使用 `http.NotFoundHandler`；没有创建业务 Handler、Service、Repository、Model、Module SDK 或管理端点。
- 定向证据：Source/duplicate/unknown/type/default round-trip、CLI tree/exit、single candidate/restart preflight、runner error/nil/uncooperative、HTTP bind/drain、terminal drain、cleanup degraded、health readiness 与合法/非法 package graph fixtures 均有测试。
- 本地验证：`go test ./...`、`go test -race ./...`、`go vet ./...`、`go build ./cmd/app`、`git diff --check` 与旧符号搜索；最终结果记录在 R021 和任务提交报告。
- 业务结论：`AC-BIZ-GATE-001` 的基础条件通过；`AC-BIZ-GATE-002` 因真实用例未确认继续阻塞，所以 BIZ-D/VSL 不启动。

## 6. 完成与提交原则

FND/GOV-F 已逐项关闭 [acceptance-matrix.md](requirements/acceptance-matrix.md) 的基础门禁并同步根权威文档。任务提交只纳入已确认实现与 012 既有未提交方案文档；既有 `.agents/skills` 迁移和 `tmp/` 不属于本任务，不纳入提交。业务实现仍需真实用例、业务方案和后续独立确认。
