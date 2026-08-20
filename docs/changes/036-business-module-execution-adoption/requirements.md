# 036 需求规格：业务模块接入 execution（Todo 落地）

引用支撑研究：R001。

## 目标

让一个真实业务模块（Todo）以**统一、声明式**的方式接入 `execution` 底层能力（幂等 / 重试 / 执行记录），
证明「kernel/app/execution → composition → 业务模块」装配链可被业务侧真实消费，并沉淀为可复用的接入文档。

## 范围

1. **纯文档**：新增 `docs/development/execution-capability.md` 权威接入文档；模块开发指南建立入口链接。
2. **非文档（待确认）**：Todo `service` 新增执行窄 port；`complete` 关键写操作经 composition 注入的
   Adapter（底层 `Capabilities.Execution` + `todo` 命名策略）执行；补测试与门禁。
3. 配置：在 `execution.policies.todo` 声明 Todo 命名策略（应用默认），供 `PolicyName` 解析。

## 约束

- 复用现有架构，不新建平行治理体系；不反向依赖 backend 具体类型（AGENTS 3.1/3.2/3.5）。
- Service 继续走「自有窄 port」：模块与 Adapter 不感知内核/第三方实现。
- `Complete` 已有业务幂等保留；执行治理在其外层叠加（去重 / 重试 / 记录）。
- 不引入分布式锁 / 消息；不改变其它业务模块。

## 非目标

- 不为其它业务模块接入；不实现真实外部主存储（Redis/DB，属 035 NEXT-002）。
- 不改变 Todo 其它用例（Create/Get/List）行为。

## 验收

- 接入文档描述与实现一致，且能从模块开发指南进入。
- Todo `Complete` 经执行 port 执行：重复提交同幂等键只执行一次写；可重试仓库失败按 `todo` 策略重试；
  留下执行记录并可观测（`Recovery`/`Health`/日志）。
- `go build ./...`、`go vet ./...`、`gofmt`、`go test ./...`（受影响 `-race`）、`go mod tidy -diff`、`git diff --check` 通过。
