# 041 任务清单

## 1. 当前状态

- 研究门禁：已通过（`R001`）。
- 计划状态：已确认，实施完成。
- 实施授权：用户在计划报告后的后续消息确认“确认，实施”。
- 外部副作用：本轮无；确认后的 smoke 仅允许临时目录、loopback 临时端口和临时 SQLite，RabbitMQ 真实协议验证如本机无环境则记录未执行。

## 2. 研究与计划任务

| ID | 工作量 | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| RES-001 | M | 无 | 复核 Logger、Service、Generation、HTTP、Execution、Schedule、Messaging、Migration、Management 与文档门禁 | R001 区分当前事实、缺口、不适用场景和计划影响 | 已完成 |
| PLAN-001 | M | RES-001 | 形成需求、设计、任务和验证方案 | `README.md`、`requirements.md`、`design.md`、`tasks.md` 完整且互相引用 | 已完成 |

## 3. 待确认实施任务

| ID | 工作量 | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| EXE-001 | S | 用户确认 | 修复 Execution 异步记录失败日志脱敏 | 不再记录 `err.Error()`；Warn 字段含 owner/phase/error_type/cause_type；测试覆盖敏感错误 | 已完成 |
| MIG-001 | M | 用户确认 | 为 migration status/up one-shot 增加结构化 operation 日志 | start/completed/failed 事件低敏可关联；CLI 输出保持原职责；测试覆盖 success/failure/close error | 已完成 |
| MSG-001 | L | 用户确认 | 补齐 Messaging Consumer 非成功 disposition 与 decode reject 日志 | defer/retry/dead-letter/reject 有低敏字段、trace/message correlation 和级别测试 | 已完成 |
| MSG-002 | M | MSG-001 | 补齐 Messaging Provider/admission generation correlation | provider/admission 日志包含 generation/desired/enabled 等关联字段；不暴露业务 API | 已完成 |
| MGT-001 | M | 用户确认 | 补齐 management health/diagnostics 异常 outcome 日志 | readiness/diagnostics/operation failure 有 owner 日志；成功轮询不产生高频 Info | 已完成 |
| SCH-001 | S | 用户确认 | 为 Scheduler 现有日志补测试门禁并按需补字段 | start/drain/stop/coordination/task failure/fatal 的 level/字段/脱敏测试稳定 | 已完成 |
| GOV-001 | M | EXE-001..SCH-001 | 更新日志规范和能力文档 | execution/schedule/messaging/migration/management 的必记/不记、级别、字段和验证要求进入主题文档 | 已完成 |
| ARCH-001 | S | 用户确认 | 扩展架构禁止模式 | production Noop、direct zap/global logger、原始错误文本日志、运行日志 fmt.Print 均有门禁 | 已完成 |
| VER-001 | L | 全部实施任务 | 执行定向测试、全量工程门禁、smoke 和 Markdown 验证 | requirements 验收标准有直接证据；失败如实记录 | 已完成 |

## 4. 实施顺序

```text
EXE-001
  -> MIG-001
  -> MSG-001 -> MSG-002
  -> MGT-001
  -> SCH-001
  -> GOV-001 -> ARCH-001 -> VER-001
```

实施时保持单轨：日志字段、实现、测试、文档和任务证据一起收敛。不得只补日志不补测试，也不得只写文档宣称已覆盖。

## 5. 重新确认触发器

- 需要改变 `pkg/logger.Logger`、`pkg/messaging`、`pkg/execution` 等公共 API；
- 需要新增第三方依赖、配置格式、环境变量或外部服务；
- 需要改变 RabbitMQ topology、ack/retry/DLX 语义、migration SQL 或业务返回语义；
- 需要启动、停止、部署或写入外部系统；
- 发现当前研究结论被代码事实推翻。

## 6. 本轮证据

| 日期 | 范围 | 证据 | 结论 |
| --- | --- | --- | --- |
| 2026-08-20 | 初始状态 | `git status --short --branch` 为 `## main...origin/main` | 工作树 clean |
| 2026-08-20 | 当前快照 | `git rev-parse HEAD` 为 `f86825b52eb19e8dd807c0db7f59d5d7c7e7102a` | 研究快照固定 |
| 2026-08-20 | 研究复用 | 检索既有 metadata，复核 028 日志研究与 035/037/038 能力文档 | 可复用但以当前代码为准 |
| 2026-08-20 | 计划文档验证 | `git diff --check`；041 目录与变更索引 Markdown 本地链接检查；041 新增文件尾随空白检查 | 通过 |
| 2026-08-20 | 提交门禁 | 复核 `git-conventional-commit` skill 与仓库 4.3 计划阶段规则 | 当前为待确认计划文档，按仓库门禁不 stage、不 commit |
| 2026-08-20 | 实施确认 | 用户在计划报告后明确回复“确认，实施” | 进入已确认实施阶段 |
| 2026-08-20 | 定向测试 | `go test ./internal/kernel/app/execution ./internal/composition ./cmd/app ./internal/kernel/app/messaging ./internal/kernel/app/messaging/rabbitmq ./internal/kernel/app/schedule ./internal/kernel/composition ./internal/module/ops/... -count=1` | 通过 |
| 2026-08-20 | 全量测试 | `go test ./... -count=1` | 通过 |
| 2026-08-20 | 工程门禁 | `gofmt -l .`；`git diff --check`；`go mod tidy -diff`；`go vet ./...`；`go build ./cmd/app` | 全部通过 |
| 2026-08-20 | 完整质量脚本 | `./scripts/Verify-Quality.ps1` | 通过；覆盖 gofmt、tidy diff、generate clean diff、全量测试、race 测试、vet、CGO-free build 与 artifact 检查 |
| 2026-08-20 | 残留搜索 | `rg 'String\("error"' cmd internal pkg -g '*.go'`；`rg 'fmt\.Print|log\.Print|println\(|zap\.' cmd internal pkg -g '*.go'` | production 源码未发现新增原始错误文本日志、全局 logger、直接 zap 或运行日志打印；匹配项仅为门禁测试/合法实现边界/文档示例 |
