# 035 需求规格：后台任务能力装配（幂等 / 重试 / 执行记录）

## 产品目标

为需要幂等、失败重试、执行记录的业务模块（以订单、支付、库存为示例）提供一套由进程统一装配、可注入、可插拔的底层能力：

1. **幂等**：对带幂等键的操作，多次提交只执行一次，重复完成返回可识别的"已完成/重复"结果。
2. **重试**：对可重试失败按受控策略（次数/退避/超时/可重试判断/熔断）重试，禁止无限重试与调用点散写重试循环。
3. **执行记录**：记录每次执行的键、状态、结果、原因、耗时、触发者与时间，保留错误链，供诊断与审计。

业务模块只消费注入的最小稳定契约（`OperationExecutor`），不感知具体 backend 与第三方实现。

## 范围

- 新增底层能力链：`pkg/execution`（契约 + 引擎 + 状态机 + 错误）→ `internal/kernel/app/execution`（组件）→ `internal/kernel/composition`（装配 + 加入 `Capabilities`）。
- 基于既有 `pkg/resilience`（重试/超时/熔断）、`pkg/fault`（可重试分类）、`pkg/concurrency`（singleflight）、`pkg/clock`、`pkg/idgen`、`pkg/database`（backend），**不重复造轮子、不引入新第三方后台/消息库**。
- 提供持久化后端：幂等占用表 + 执行记录表，使用当前 `pkg/database`。
- 通过 composition 把 `OperationExecutor` 注入，作为可选的 `Capabilities.Execution` 出口。

## 非目标

- 不实现订单/支付/库存业务模块本身（仅作为能力消费示例出现在文档/示例）。
- 不实现跨进程分布式锁 / 严格一次（at-most-once is available，at-least-once 不在本任务）；不做消息队列、死信、定时调度、跨节点任务分发。
- 不重建重试/超时/熔断引擎（复用 `pkg/resilience`）。
- 不改动 Kernel 公共生命周期/依赖模型。

## 约束

- 对齐 AGENTS：错误保留原因链、不吞错；重试有上限；记录写入失败须向上返回，不静默成功；日志低敏。
- 对齐 032 配置边界：`pkg/execution` 只提供通用语义与基础默认；`kernel/app/execution` 集中声明应用默认与装配，不隐式依赖 `pkg/*/DefaultConfig()`。
- `pkg/execution` 不 import `internal`；业务模块不得读取 backend 具体类型。
- 幂等键/执行记录的状态、错误码使用场景内命名常量或专用类型，不散写魔法字符串。
- 文档与实现一致；本能力是可注入能力，业务模块接入走 composition 显式接线。

## 验收标准

1. `go build ./...`、`go vet ./...`、`gofmt -l .`、`go test ./... -count=1` 全部通过。
2. `go mod tidy -diff`、`go generate ./...`、`git diff --check` 干净。
3. `pkg/execution` 单测覆盖：幂等判重、重复提交返回已完成、重试耗尽、可重试 vs 不可重试、超时、记录写入、backend 失败语义、错误链保留。
4. `internal/kernel/app/execution` 组件测试：Definition 组装、Lease 切换、配置默认值、backend 生命周期。
5. `internal/kernel/composition` 测试：`Compose` 装配后 `Capabilities.Execution` 可用且不泄漏 backend 类型。
6. 门禁：现有架构测试（`architecture_test.go` 各 validator）与新增能力相关约束通过，无循环/反向依赖。
7. 文档同步：`pkg/README.md` 能力清单、`internal/kernel/app/README.md` "当前组件"、能力评估表、权威文档与实现一致。
