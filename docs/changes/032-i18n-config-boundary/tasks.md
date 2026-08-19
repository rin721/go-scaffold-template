# 任务：i18n 配置职责边界与集中声明

## 1. 门禁状态

- 研究门禁：已通过，证据为 `R001`、`R002`。
- 当前计划状态：已完成。
- 当前授权：用户确认 032 方案，授权本地实施、验证与聚焦提交；不授权 push、tag、Release、部署或外部写入。
- 实施前提：已满足；cache 的 `redisstore.DefaultTagPrefix` 作为豁免例保留（基础默认常量回退），见 design。
- 外部副作用：无。实施只修改仓库内源码、测试、配置示例与文档目录，不写外部数据库。

## 2. 任务清单

| ID | 工作量 | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `RES-001` | M | 无 | 复核当前 i18n 组件默认值来源、路径声明与业务消费 | R001 区分事实、推断与消费方 | 已完成 |
| `RES-002` | M | `RES-001` | 审计全部 kernel/app 组件默认配置来源与边界 | R002 区分有问题/正确模板与边界 | 已完成 |
| `PLAN-001` | M | `RES-001..002` | 冻结 032 需求、设计、文件影响、风险与验证 | README、requirements、design、tasks 完整且相互引用 | 已完成 |
| `I18N-001` | M | 用户确认 | `kernel/app/i18n` 集中声明默认配置与 `./locales`，`defaults{}/defaultConfig` 不调用 `pkg/i18n.DefaultConfig()` | 组件内单一集中声明；`./locales` 语义落地（`LocalesDir=./locales`、默认语言、缺失行为集中声明） | 已完成 |
| `I18N-002` | M | `I18N-001` | 更新 `i18n_test.go` 与 `config.example.yaml` i18n 段；新增 `./locales/messages.zh-CN.yaml` | 单元测试与配置示例反映集中默认 + `./locales`；消息文件按约定维护 | 已完成 |
| `ALIGN-001` | M | `I18N-001` | 对齐 logger/database 默认值来源非 `pkg/*.DefaultConfig()`（自声明）；cache 保留 `redisstore.DefaultTagPrefix` 基础默认回退（用户豁免例） | logger/database 组件内集中自声明；cache 仅作为未声明时的基础默认回退保留 | 已完成 |
| `GOV-001` | M | `ALIGN-001` | 新增架构门禁 `validateKernelAppConfigOwnership`，阻塞 `kernel/app` 默认配置整体复用 `pkg/*.DefaultConfig()`；允许基础默认常量引用 | 可执行门禁通过；反例失败；redisstore.DefaultTagPrefix 这类基础常量不被误伤 | 已完成 |
| `DOC-001` | M | `ALIGN-001`、`GOV-001` | 同步业务 i18n 接入规范（模块开发指南 8.4）、`pkg/README` 边界、配置说明、`config.example.yaml` | 权威文档单轨描述；`./locales` 语义一致 | 已完成 |
| `VER-001` | L | 全部实施任务 | 执行单元/集成/完整 gate 与文档审阅 | requirements 第 7 节全部有直接证据；无旧路径/依赖残留 | 已完成 |

## 3. 实施顺序

```text
I18N-001 -> I18N-002
  -> ALIGN-001 -> GOV-001 -> DOC-001 -> VER-001
```

`I18N-001` 未落地前不得改 logger/database/cache 的门禁；对齐与门禁必须同轮完成，避免长期红灯。

## 4. 实施结果

1. 用户在计划报告后的后续消息中确认目标方向与方案（不调整），并明确 `pkg/*` 基础默认常量（如 `redisstore.DefaultTagPrefix`）可作为未声明时的回退保留。
2. 实施按 `I18N-001 -> I18N-002 -> ALIGN-001 -> GOV-001 -> DOC-001 -> VER-001` 完成，见 `VER-001` 验证矩阵。
3. 实施提交只包含 032 范围文件；不 push、不 tag、不 release。

## 5. 逐轮证据

| 轮次 | 完成任务 | 证据 | 剩余风险 |
| --- | --- | --- | --- |
| 1 | `RES-001`、`RES-002`、`PLAN-001` | 研究结论、需求、设计、任务文档完成（HEAD `49409c3` 提交计划文档前） | 未实施 |
| 2 | `I18N-001..VER-001` | 见下方验证矩阵 | 无真实第二个业务模块 |

## 6. 验证矩阵（VER-001）

- `go build ./...`：通过。`go vet ./...`：通过。`gofmt -l .`：0 文件。
- `go test ./... -count=1`：通过。`observability` 的 `TestBoundedProcessorCountsExporterFailureWithoutSensitiveText` 在同一完整套件运行中出现一次异步边界抖动，单测隔离连续 3 次通过，与 032 无关（032 未改动 observability）。
- 架构门禁 `TestProductionPackageGraphRespectsCompositionBoundaries`（含新增 `validateKernelAppConfigOwnership`）：通过；反例（`kernel/app` 默认配置调用 `pkg/*.DefaultConfig()`）会被识别。
- cache 的 `redisstore.DefaultTagPrefix` 作为基础默认常量回退保留，不被门禁误伤（门禁只拦截 `DefaultConfig` 选择器，不拦截 `DefaultTagPrefix`）。
- `go test -race ./internal/kernel/app/... ./internal/kernel/composition/... ./internal/composition/...`：通过。
- `go mod tidy -diff`、`go generate ./...`（幂等）、`git diff --check`：通过。

## 7. 停止条件

- 命中 design 第 11 节重新确认触发器；
- 工作区出现无法可靠分离的用户修改；
- 测试、race、vet、build、tidy 或门禁失败且无法在确认范围内修复；
- 为继续工作必须保留双默认、散落路径或兼容 wrapper。

## 8. 建议实施提交信息（已实施）

```text
refactor(kernel): centralize kernel/app defaults and standardize i18n locales
```