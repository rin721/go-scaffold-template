# 任务：Handler-first HTTP 路由绑定

## 1. 门禁状态

- 研究门禁：已通过，证据为 `R001`、`R002`。
- 当前计划状态：待确认。
- 当前授权：只完成研究、需求、设计、任务文档和变更索引；未授权源码、配置、生成物或测试实现。
- 实施前提：用户必须在计划报告之后的后续消息明确确认“确认实施 026 当前方案”。
- 外部副作用：不需要服务启动、数据库操作、push、tag、Release 或部署。

## 2. 任务清单

| ID | 工作量 | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- |
| `RES-001` | M | 无 | 复核当前 Handler、generated binding、Router 与新增模块摩擦 | R001 区分当前正确行为、耦合事实、推断和局限 | 已完成 |
| `RES-002` | M | `RES-001` | 复核 oapi-codegen v2.8.0 分区能力并比较方案 | R002 使用版本固定的官方主源，说明采用/暂缓/拒绝路径 | 已完成 |
| `PLAN-001` | M | `RES-001..002` | 冻结 Handler-first 需求、设计、文件影响、风险与验证 | README、requirements、design、tasks 完整且相互引用 | 已完成 |
| `GOV-001` | M | `PLAN-001`、用户确认 | 建立通用 HTTP binding owner 与完整接口实现门禁 | 正常图通过；模块自建 route binding、transport 导入模块、多个完整实现等反例失败 | 待确认 |
| `HANDLER-001` | M | `GOV-001` | 把 Todo HTTP 收敛为窄 operation Handler | 不创建 Router/validator，不满足完整应用接口；协议映射测试通过 | 待确认 |
| `AUTH-001` | M | `HANDLER-001` | 拆分 application operation gate 与 Todo actor access | 401/403、actor、对象授权和未知错误链保持，Auth 类型不泄漏 | 待确认 |
| `API-001` | M | `HANDLER-001`、`AUTH-001` | 建立 application-owned static strict API aggregate | 唯一满足完整接口；只转发；nil 依赖确定失败 | 待确认 |
| `ROUTE-001` | L | `API-001` | 建立单一 OpenAPI route binding | 规范、validator、strict middleware、错误边界和 generated routes 只装配一次 | 待确认 |
| `COMP-001` | M | `ROUTE-001` | 调整 Generation 与 application Router 构造顺序 | 代码可读地呈现 Handler -> aggregate -> routes -> Router -> Server | 待确认 |
| `DOC-001` | M | `COMP-001` | 同步当前权威文档与 026 证据 | 新模块步骤和职责只保留一套当前说明；025 保持历史事实 | 待确认 |
| `VER-001` | L | 全部实施任务 | 执行结构、生成、协议、测试、race、vet、build、tidy、文档和 Diff 门禁 | requirements 第 7 节全部有直接证据，无旧轨残留 | 待确认 |

## 3. 实施顺序

```text
GOV-001
  -> HANDLER-001 -> AUTH-001
  -> API-001 -> ROUTE-001 -> COMP-001
  -> DOC-001 -> VER-001
```

结构门禁与迁移必须在同一施工轮完成，不能把预期失败长期留在分支。`HANDLER-001..COMP-001` 是单轨替换，不保留旧构造器、旧完整接口断言、双 route binding 或兼容 wrapper。

## 4. 逐轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-16 | `RES-001`、`RES-002`、`PLAN-001` | HEAD `a42703f`；初始工作树 clean；当前代码/生成物/025 与 v2.8.0 官方文档复核；定向 Go tests 通过 | 见本轮纯文档提交 | 没有真实第二模块；实施尚未确认；按 tag/spec 分包仅保留为规模触发后的研究路径 |

## 5. 确认后的精确验证

至少执行：

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

- 生成前后 `api/openapi.yaml` 与 `internal/transport/http/api` clean diff；
- 定向 Todo HTTP、application route binding、composition、Auth、Ops、cmd/app 和 architecture tests；
- 搜索 `chi.NewRouter`、`GetSwagger`、`NewStrictHandler`、`HandlerWithOptions` 的 owner 与调用计数；
- 搜索模块对完整 `StrictServerInterface` 的断言、旧 `RequestAccess`、旧 `httpbinding.New` 和 compatibility wrapper；
- 审阅完整 diff、staged diff、staged file list 与敏感信息。

## 6. 提交边界

本轮纯文档计划按仓库例外可以提交。用户确认实施后，源码、测试、权威文档与 026 实施证据应作为一个聚焦 Conventional Commit 提交；不得 push。

建议实施提交信息：

```text
refactor(http): bind routes after composing handlers
```

## 7. 停止条件

- 用户尚未确认当前计划；
- 命中 design 第 14 节重新确认触发器；
- 工作区出现无法可靠分离的用户修改；
- 生成、测试、race、vet、build、tidy 或架构门禁失败且无法在确认范围内修复；
- 为继续工作必须保留双轨、501 fallback、手写路径或动态 registry。

停止时保留研究和计划事实，不以占位实现或降低门禁冒充完成。
