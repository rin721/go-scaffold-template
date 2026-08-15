# 任务：业务模块边界收口

## 1. 门禁状态

- 研究门禁：已通过，证据为 `R001`。
- 当前计划状态：已完成。
- 当前授权：用户已在计划报告后的后续消息中明确确认“确认实施 025 当前方案”，授权 `GOV-001` 至 `VER-001` 的本地实施、验证和聚焦提交。
- 实施前提：已满足；若命中设计第 12 节触发器则恢复待确认。
- 外部副作用：本任务不需要 push、tag、Release、部署、数据库写入或外部服务操作。

## 2. 任务清单

| ID | 工作量 | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `RES-001` | M | 无 | 复核既有研究、当前模块、HTTP、composition、入口和 package graph | R001 区分事实、用户决策、推断、局限和任务影响；研究门禁通过 | 已完成 |
| `PLAN-001` | M | `RES-001` | 冻结需求、目标结构、窄端口、文件影响、失败语义和验证矩阵 | README、requirements、design、tasks 完整引用 R001；状态为待确认 | 已完成 |
| `GOV-001` | M | `PLAN-001`、用户确认 | 扩展 package graph 的通用 module-owner 规则和正反 fixture | 当前合法图通过；transport 外溢、cmd 直连模块、跨模块 import、模块反向依赖均失败 | 已完成 |
| `HTTP-001` | L | `GOV-001` | 把手写 Todo HTTP Adapter 与测试迁入 `internal/module/todo/binding/http` | 新路径拥有 Handler/DTO/error/request metadata；旧顶层文件和 package 零残留 | 已完成 |
| `PORT-001` | M | `HTTP-001` | 建立 Todo-owned RequestAccess 和 composition Auth Adapter | Todo HTTP 不导入 Auth；401/403/未知依赖错误链保持；对象授权 port 不被合并 | 已完成 |
| `MOD-001` | M | `PORT-001` | 增加 Todo HTTP profile 与显式完成品输出 | core/local profile 无 HTTP 占位；HTTP profile 返回非 nil Handler、Service、Contribution；构造无 I/O/goroutine | 已完成 |
| `COMP-001` | M | `MOD-001` | 简化 Generation 与 application Router | composition 只连接窄端口并安装完成 Handler；无 Todo DTO/presenter/Handler 构造逻辑 | 已完成 |
| `ENTRY-001` | S | `GOV-001` | 用 composition-owned BuildInfo 收口 `cmd/app` 入口 | cmd/app 不导入 module/kernel；`/build` 与 ldflags 行为不变 | 已完成 |
| `DOC-001` | M | `HTTP-001`、`MOD-001`、`COMP-001`、`ENTRY-001` | 同步当前权威文档并记录 025 实施证据 | 当前说明只描述模块内 binding；历史记录不成为第二套 authority；025 状态真实 | 已完成 |
| `VER-001` | L | 全部实施任务 | 执行残留、生成、协议、测试、race、vet、build、tidy、文档和 Diff 门禁 | requirements 第 5 节十项验收均有直接证据，无未解释失败与旧轨残留 | 已完成 |

## 3. 实施顺序

```text
GOV-001
  -> HTTP-001 -> PORT-001 -> MOD-001 -> COMP-001
  -> ENTRY-001
  -> DOC-001
  -> VER-001
```

`GOV-001` 与迁移在同一施工轮完成，不能把“预期失败”的 architecture test 单独提交。`HTTP-001..COMP-001` 是一个单轨替换，不保留旧入口。`ENTRY-001` 不改变 Ops 行为，只关闭 cmd 的直接 module import。

## 4. 逐轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-15 | `RES-001`、`PLAN-001` | HEAD `02a1768`；Git 工作树初始无修改；当前代码、权威文档、016/017/024-R005/R006 与 package graph 静态复核 | 未提交（计划阶段禁止暂存与提交） | 第二个 HTTP 模块的 OpenAPI binding 分区不在本任务证明范围；实施尚未确认 |
| 2 | 2026-08-15 | `GOV-001` 至 `VER-001` | Todo HTTP 新路径与 401/403/依赖脱敏测试；真实/fixture package graph；OpenAPI 生成 clean diff；`gofmt -l .`、`go mod tidy -diff`、`go test ./...`、`go test -race ./...`、`go vet ./...`、`go build ./cmd/app`、Markdown 链接和 `git diff --check` 通过 | 见本行所在聚焦提交 | 第二个 HTTP 模块仍须独立研究 route binding 分区；本任务未授权 push |

## 5. 精确验证清单

确认实施后，至少执行：

```powershell
gofmt -l .
go generate ./...
go mod tidy -diff
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/app
git diff --check
```

并补充：

- OpenAPI 与 `internal/transport/http/api` 生成前后内容无变化；
- 定向运行 Todo HTTP、Auth、composition、cmd/app 与 architecture tests；
- 搜索旧 `httptransport.NewTodoHTTPHandler`、顶层 `TodoHandler`、旧 import path 和 compatibility wrapper；
- 检查所有 `internal/module/*` import owner；
- 审阅完整 diff、staged diff 和 staged file list。

## 6. 提交边界

计划阶段没有暂存或提交。用户确认后已完成实施与验证，本任务使用一个聚焦的 Conventional Commit：

```text
refactor(module): contain Todo HTTP binding
```

只提交 025 Go、测试、权威文档和任务证据；不带入本地配置、生成临时文件、artifact 或其他任务修改。不得 push。

## 7. 停止条件

- 用户尚未确认当前计划；
- 命中 design 第 12 节重新确认触发器；
- 工作区出现无法可靠分离的用户修改；
- 生成、测试、race、vet、build、tidy 或架构门禁失败且无法在已确认范围内修复；
- 迁移必须保留双轨或改变 API/配置/数据语义。

停止时保留研究和计划事实，不用 alias、白名单、空实现或降低门禁冒充完成。
