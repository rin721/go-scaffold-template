# 任务：开发日志基线与启动可见性

## 1. 门禁状态

- 研究门禁：已通过，证据为 `R001`。
- 当前计划状态：已确认并完成。
- 实施授权：用户在计划报告后的独立消息明确确认“实施 028”。
- 外部副作用：进程 smoke 仅使用测试临时目录、loopback 临时端口和临时 SQLite，测试结束后已清理；未连接外部服务，未 push、tag、发布 Release 或部署。

## 2. 任务清单

| ID | 工作量 | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `RES-001` | M | 无 | 复核 Logger、默认级别、启动/重载/HTTP 调用链和文档门禁 | R001 区分事实、判断、局限和实施影响 | 已完成 |
| `PLAN-001` | M | `RES-001` | 冻结默认、事件、级别、字段、错误边界、文件影响和验证方案 | README、requirements、design、tasks 完整且互相引用 | 已完成 |
| `GOV-001` | M | `PLAN-001`、用户确认 | 建立开发日志 authority、模块评估项、Agent 门禁与文档导航 | 日志必须项、例外、级别、字段、去重、脱敏和验证只有一份当前 authority | 已完成 |
| `DEF-001` | M | `GOV-001` | 实施 development=debug、production=info 的环境化默认 | nil/缺失/显式 level 语义确定，config init 与示例同步，无兼容 alias | 已完成 |
| `START-001` | L | `DEF-001` | 补齐 Service 启动、generation、资源、listener ready 和停止事件 | 健康路径形成连续 Debug/Info 链，owner/phase/generation/地址可关联且无 Warn/Error | 已完成 |
| `FAIL-001` | M | `START-001` | 建立初始失败与 Supervisor 失败的唯一低敏结构化边界 | 一条分类 Error + 一条最终 stderr；下层不重复，取消不误报 | 已完成 |
| `RELOAD-001` | M | `START-001` | 校正 no-op/success/rejected/degraded reload 分级 | 分别为 Debug/Info/Warn/Error，旧代保持与 cleanup debt 语义可见 | 已完成 |
| `HTTP-001` | M | `GOV-001` | 按最终 HTTP outcome 校正 access log 级别和字段 | success/rejection/5xx/panic 分级正确，关联字段保留且无敏感内容 | 已完成 |
| `ARCH-001` | S | `GOV-001` | 禁止 production Noop 与绕过项目 Logger 的直接实现 | 正常代码通过；Noop/direct zap/global logger fixture 失败 | 已完成 |
| `DOC-001` | M | `DEF-001..ARCH-001` | 同步权威文档与 028 实施证据 | 默认值、运行输出和开发步骤与真实实现一致，历史记录不冒充 authority | 已完成 |
| `VER-001` | L | 全部实施任务 | 执行默认、生命周期、HTTP、架构、进程 smoke 和全量工程门禁 | requirements 第 6 节全部有直接证据，无旧默认和旧分级残留 | 已完成 |

## 3. 实施顺序

```text
GOV-001 -> DEF-001
  -> START-001 -> FAIL-001 -> RELOAD-001
  -> HTTP-001 -> ARCH-001
  -> DOC-001 -> VER-001
```

默认、事件、文档和测试作为同一单轨变更实施；不能只把 level 改成 debug 而继续缺少事件，也不能只补大量日志而保留默认过滤和无开发规范状态。

## 4. 本轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-16 | `RES-001`、`PLAN-001` | HEAD `a700d25248b483bb5cb844ee235d4e9bbb6b37d4`；初始工作树 clean；Logger/Manager/Application/Generation/Kernel/Supervisor/HTTP/Auth、默认配置、测试、004/007 和当前文档静态复核 | 与实施同一提交 | 当轮尚未运行真实 Service；HTTP 最终 outcome 的可观察位置待实施测试冻结 |
| 2 | 2026-08-16 | `GOV-001..VER-001` | 用户确认“实施 028”；定向测试、真实进程日志 smoke、`gofmt -l .`、`go mod tidy -diff`、一次 `go test ./... -count=1`、`go test -race ./... -count=1`、`go vet ./...`、`go build ./cmd/app`、14 份变更 Markdown 的 90 个本地链接和 `git diff --check` 通过 | 见本行所在聚焦提交 | 后续 full 重跑复现了范围外 `TestBoundedProcessorCountsExporterFailureWithoutSensitiveText` 的既有时序竞态，10 次隔离运行失败 2 次；另有一次 Windows SQLite 文件共享偶发失败。028 定向包与 `cmd/app` 10 次均通过，未修改范围外 observability 实现，未授权 push |

## 5. 验证基线

至少执行：

```powershell
gofmt -l .
go mod tidy -diff
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/app
git diff --check
```

并补充：

- `pkg/logger` development/production default 与显式 level 测试；
- application/GenerationCoordinator 健康、no-op、reload reject、cleanup debt、initial failure、cancellation 事件测试；
- HTTP success/4xx/429/503/5xx/panic 分级、request/trace correlation 和 secret redaction 测试；
- architecture 正反 fixture：production Noop、direct zap/global logger；
- `config init` golden、`config.example.yaml` strict binding；
- 测试临时目录 + loopback 临时端口 + SQLite 的真实进程日志 smoke；
- Markdown 链接检查，解析时排除 fenced/inline code；
- 搜索旧 `level: info` 默认声明、无 owner 的 startup message、production `logger.Noop()` 与重复完整错误日志；
- 完整 diff、staged diff、staged file list 和敏感信息审阅。

## 6. 提交边界

研究、计划、源码、配置、测试、权威文档和 028 实施证据作为一个聚焦 Conventional Commit 提交；不得 push。

实施提交信息：

```text
feat(logging): make service lifecycle observable
```

## 7. 停止条件

- 用户尚未确认当前计划；
- 命中 design 第 10 节重新确认触发器；
- 工作区出现无法可靠分离的用户修改；
- 必须引入第二套 Logger、全局状态、完整错误重复输出、敏感字段或健康路径伪 Warn/Error；
- 相关测试、race、vet、build、tidy、进程 cleanup 或架构门禁失败且无法在确认范围内修复。

停止时保留研究与计划事实，不用 Noop、吞错、降低级别或删除验证冒充完成。
