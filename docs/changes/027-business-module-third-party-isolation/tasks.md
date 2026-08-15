# 任务：第三方封装与分轨装配

## 1. 门禁状态

- 研究门禁：已通过，证据为 `R001`。
- 当前计划状态：待确认。
- 当前授权：仅 `RES-001`、`PLAN-001` 与当前权威文档修订；禁止非文档实施。
- 实施前提：用户在本计划报告后的后续消息中明确确认 027 当前方案。
- 外部副作用：不需要 push、tag、Release、部署、数据库或外部系统写入。

## 2. 任务清单

| ID | 工作量 | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- |
| `RES-001` | M | 无 | 复核第三方 import、导出签名、owner、生命周期与现有门禁 | R001 区分合法模块 Adapter、真实泄漏和非业务底层能力 | 已完成 |
| `PLAN-001` | M | `RES-001` | 冻结两轨规则、Observability 边界、迁移、验证与重新确认触发器 | README、requirements、design、tasks 与权威文档一致 | 已完成 |
| `CONTRACT-001` | L | `PLAN-001`、用户确认 | 建立 `pkg/observability` 窄契约与 contract tests | 不复制第三方 API；导出零第三方类型；职责可独立测试 | 待确认 |
| `METRICS-001` | L | `CONTRACT-001` | 建立 process-stable Metrics 底层组件 | registry identity/累计值稳定；输出 facade/Access；无 Close 泄漏 | 待确认 |
| `TRACE-001` | XL | `CONTRACT-001`、`METRICS-001` | 建立 generation-safe Telemetry 底层组件 | candidate/commit/rollback/flush/drop/diagnostics 语义通过；无裸 provider 逃逸 | 待确认 |
| `COMP-001` | M | `METRICS-001`、`TRACE-001` | 接入 kernel composition 与 application composition | typed Binding/Capabilities 明确；上层无 Prometheus/OTel/具体 Adapter | 待确认 |
| `OPS-001` | L | `COMP-001` | 收口 Ops 模块为项目自有 Observability 消费者 | 删除具体 Metrics/Telemetry 导出；management/diagnostics 行为保持 | 待确认 |
| `AUTH-001` | M | `PLAN-001` | 加固业务模块私有 Adapter 门禁 | Auth/JWT 留在模块内；jwx/具体 Verifier 不越过 Adapter；composition 不穿透 | 待确认 |
| `GOV-001` | L | `CONTRACT-001`、`OPS-001`、`AUTH-001` | 建立分轨 package graph/AST 正反门禁 | model/service、Adapter export、module root、pkg、composition 五类违规均失败 | 待确认 |
| `CLEAN-001` | M | `OPS-001`、`GOV-001` | 单轨删除旧 Ops 技术实现与失效文档 | 旧路径/字段/import/wrapper/fallback 零残留 | 待确认 |
| `DOC-001` | M | `CLEAN-001` | 同步全部当前权威文档和实施证据 | 已实现、目标和历史记录明确分离 | 待确认 |
| `VER-001` | XL | 全部实施任务 | 执行结构、生命周期、行为、生成、test/race/vet/build/tidy 与 Diff 门禁 | requirements 第 5 节全部有直接证据，无未解释失败 | 待确认 |

## 3. 实施顺序

```text
CONTRACT-001
  -> METRICS-001 -> TRACE-001 -> COMP-001 -> OPS-001
  -> AUTH-001 -> GOV-001 -> CLEAN-001 -> DOC-001 -> VER-001
```

组件迁移与旧实现删除必须在同一确认任务内完成，不保留双轨。若 `TRACE-001` 命中 design 第 12 节触发器，立即停止并把计划恢复为待确认。

## 4. 逐轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-16 | `RES-001`、`PLAN-001` | HEAD `80bb57f`；初始工作树 clean；当前 Auth/Ops import、导出签名、composition、pkg/Kernel App 与 architecture tests 复核 | 见本轮纯文档提交 | Metrics/Telemetry 精确 Definition 仍需实施期定向证明；非文档实施尚未确认 |

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

- OpenAPI/生成物 clean diff；
- 定向 Auth/JWT、Ops、Observability、Kernel App、composition、generation/reload tests；
- 搜索第三方 imports、exported selectors、具体 Adapter 字段、旧 Ops 路径和旁路 provider/registry 构造；
- 审阅完整 diff、staged diff、staged file list 与敏感信息。

## 6. 提交边界

本轮纯文档方案使用聚焦 Conventional Commit。确认实施后，源码、测试、权威文档和 027 实施证据作为另一个聚焦提交；不得 push。

建议实施提交信息：

```text
refactor(observability): hide third-party runtime details
```

## 7. 停止条件

- 用户尚未确认当前计划；
- 命中 design 第 12 节重新确认触发器；
- 工作区出现无法可靠分离的用户修改；
- 必须暴露第三方类型、保留双轨或引入万能 Wrapper 才能继续；
- 验证失败且无法在已确认范围内修复。
