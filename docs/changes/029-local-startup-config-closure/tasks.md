# 任务：本地启动与配置闭环

## 1. 门禁状态

- 研究门禁：已通过，证据为 `R001`。
- 当前计划状态：已确认并实施。
- 当前授权：用户已在计划报告后的后续消息确认“实施 029”；允许执行本任务的源码、测试、当前文档和临时进程验证。
- 实施前提：已满足。
- 外部副作用：确认后的进程测试只使用临时目录、SQLite 与 loopback 临时端口；不连接外部服务，不授权覆盖用户 `config.yaml`、push、tag、Release 或部署。

## 2. 任务清单

| ID | 工作量 | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `RES-001` | M | 无 | 复核用户错误、配置 producer/consumer、测试缺口和文档 authority | R001 区分运行事实、代码事实、推断和局限 | 已完成 |
| `PLAN-001` | M | `RES-001` | 冻结 binding 单轨、文档信息架构、用户旅程和验收 | README、requirements、design、tasks 完整互引 | 已完成 |
| `CFG-001` | M | 用户确认 | 建立唯一 application-owned binding 构造并迁移全部调用方 | Bootstrap/Service/Migration/Todo 不再复制集合 | 已完成 |
| `MIG-001` | M | `CFG-001` | 修复 generated config 被 Migration 拒绝 | Management/Observability 合法；unknown root 仍失败 | 已完成 |
| `E2E-001` | L | `MIG-001` | 建立生成、迁移、启动、ready、停止闭环测试 | 临时资源旅程通过且资源全部释放 | 已完成 |
| `DOC-001` | L | `MIG-001` | 重构根 README 与 docs 导航，新增本地开发/配置 authority | 一个推荐流程，类别关系明确，无第二套 authority | 已完成 |
| `DOC-002` | M | `DOC-001` | 同步配置示例、migration 运维与模块链接 | 命令顺序、配置角色和错误处理无冲突 | 已完成 |
| `VER-001` | L | 全部实施任务 | 执行定向/进程/full/race/vet/build/tidy/docs/diff 门禁 | requirements 第 6 节有直接证据；范围外 race 不稳定项已单独复跑并记录 | 已完成 |

## 3. 实施顺序

```text
CFG-001 -> MIG-001 -> E2E-001
                    -> DOC-001 -> DOC-002
                    -> VER-001
```

不能只改文档绕过 generated config 缺陷，也不能只修 Migration 而继续保留冲突入口。

## 4. 本轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-16 | `RES-001`、`PLAN-001` | HEAD `ba50c21`；初始工作树 clean；用户运行记录；Application/Bootstrap/Service/Migration/Todo binding、ValidateCandidate、config init/进程测试、根 README、docs 导航、配置示例与 migration 文档静态复核 | 计划阶段不提交 | 非文档实施待确认；尚未执行 generated config 真实闭环 |
| 2 | 2026-08-16 | `CFG-001`、`MIG-001`、`E2E-001`、`DOC-001`、`DOC-002`、`VER-001` | 新增 `applicationOwnedConfigurationBindings()`；Bootstrap/Service/Migration/Todo/生成测试共用同一集合；`go test ./cmd/app -run TestProcessGeneratedConfigurationSupportsMigrationAndServiceStartup -count=10 -v` 通过；`go test ./internal/composition ./cmd/app -count=1` 通过；`go test ./... -count=1`、`go test -race -p 1 ./... -count=1`、`go vet ./...`、`go build ./cmd/app`、`go mod tidy -diff`、`git diff --check` 通过；Markdown 本地链接排除 fenced/inline code 后 274 文件通过 | `fix(config): close generated startup workflow` | `go test -race ./... -count=1` 两次并发全量运行分别撞到范围外既有不稳定项：`internal/kernel/app/observability TestBoundedProcessorCountsExporterFailureWithoutSensitiveText` 一次缺失 `LastErrorType`，单测 race `-count=5` 复跑通过；`internal/kernel/composition TestHostReloadsRealSQLiteAndKeepsCrossComponentTransactionAtomic` 一次 Windows 临时配置文件删除被占用，单测 race `-count=5` 复跑通过。本任务未修改这两个包；串行全量 race 已通过。 |

## 5. 确认后的验证

```powershell
gofmt -l .
go mod tidy -diff
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/app
git diff --check
```

实施轮验证结果：

- `gofmt -l internal/composition cmd/app`：无输出。
- `go mod tidy -diff`：无输出。
- `go test ./internal/composition ./cmd/app -count=1`：通过。
- `go test ./cmd/app -run TestProcessGeneratedConfigurationSupportsMigrationAndServiceStartup -count=10 -v`：通过，日志包含 `application generation started`、`application ready`、draining/stopped。
- `go test ./... -count=1`：通过。
- `go vet ./...`：通过。
- `go build ./cmd/app`：通过；生成的本地 `app.exe` 已删除，未纳入提交。
- `git diff --check`：通过。
- Markdown 本地链接检查：排除 fenced/inline code 后 274 个 Markdown 文件通过。
- `go test -race -p 1 ./... -count=1`：通过。
- `go test -race ./... -count=1`：并发全量运行未取得一次性绿灯；失败均为范围外既有不稳定测试，且对应单测 race `-count=5` 复跑通过，详见本轮证据。

另需验证：

- binding ID/path 顺序和调用方残留搜索；
- generated config 被 Migration/Todo/Service 消费；
- 真 unknown root 仍失败；
- 临时 SQLite、两个临时 listener、ready 与 graceful stop；
- 本地启动命令只在一个 authority 定义，其余位置只链接；
- Markdown 本地链接排除 fenced/inline code 后通过；
- staged file list、完整 staged diff 和敏感信息检查。

## 6. 提交边界

确认后，研究/计划、源码、测试、当前使用文档和实施证据作为一个聚焦 Conventional Commit 提交；不得 push。

建议提交信息：

```text
fix(config): close generated startup workflow
```

## 7. 停止条件

- 用户尚未确认当前 029 计划；
- 命中 design 第 9 节重新确认触发器；
- 工作区出现无法分离的用户修改；
- 必须放宽 strict config、删除合法 section、自动 migration、固定端口或连接外部服务才能通过；
- 相关测试、资源 cleanup 或文档门禁失败且无法在确认范围内修复。
