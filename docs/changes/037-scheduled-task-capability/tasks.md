# 定时调度能力任务清单

## 1. 当前状态

- 变更编号：`037`
- 研究门禁：已通过
- 计划状态：用户已于 2026-08-20 明确确认
- 非文档实施：已完成
- 暂存/Commit：本变更由单一 Conventional Commit 完成；hash 由 Git 历史和最终交付报告记录，不 push

用户已在本计划报告之后明确回复“确认按 037 计划实施”。实施只能覆盖下列 Task ID；出现公共接口、依赖选择、模块边界、数据迁移或外部副作用的实质变化时，必须退回计划并重新确认。

## 2. 工作量说明

- `S`：局部契约或文档调整，影响面明确。
- `M`：涉及一个能力链及配套测试。
- `L`：跨 Generation、并发或多能力集成，需要系统级验证。

工作量只用于拆分和审阅，不是工期承诺。

## 3. 任务清单

| Task ID | 工作量 | 依赖 | 状态 | 任务与完成条件 |
|---|---:|---|---|---|
| RES-001 | M | 无 | 已完成 | 建立 `R001`，以代码、配置和测试盘点 Module、Generation、Execution、Cache、Telemetry、Health、Supervisor 与 Runtime 事实。 |
| RES-002 | M | 无 | 已完成 | 建立 `R002`，复核 gocron、Redis 官方锁语义、go-redis 能力、维护状态、许可证和适用边界。 |
| RES-003 | M | RES-001, RES-002 | 已完成 | 建立 `R003`，形成复用/合并/最小补齐矩阵和单轨集成结论。 |
| PLN-001 | M | RES-003 | 已完成 | 完成 requirements/design/tasks，并将非文档实施保持为待确认。 |
| DEP-001 | S | 用户确认 | 已完成 | 固定经复核的 gocron/v2 版本，更新 go.mod/go.sum；记录许可证和 Go 版本验证，不引入第二个调度引擎。 |
| SCH-001 | M | DEP-001 | 已完成 | 新增 `pkg/schedule` 自有契约、构造/校验、稳定错误和单元测试；不暴露第三方或内部类型。 |
| MOD-001 | M | SCH-001 | 已完成 | 扩展 `module.Contribution.Schedules`，实现显式聚合、复制、排序、重复 ID/策略冲突校验与测试。 |
| COO-001 | M | SCH-001 | 已完成 | 新增 `pkg/coordination` 租约契约、Key/Options、稳定错误和契约测试；token 保持 Adapter 私有。 |
| CAC-001 | L | COO-001 | 已完成 | 在 Cache Redis owner 内复用现有 client，实现原子 acquire/renew/release、运行期故障分类和中断/恢复测试；没有第二套 client。 |
| CAC-002 | M | CAC-001 | 已完成 | Cache Ready 收敛为结构就绪，普通缓存、Ping 与协调调用保留真实运行期错误且不静默回退。 |
| OBS-001 | M | SCH-001 | 已完成 | 扩展现有 Telemetry facade 支持后台 work span，共享 provider/shutdown owner，并验证 trace identity 与完整 Lease 范围。 |
| EXE-001 | M | SCH-001, OBS-001 | 已完成 | 调度 occurrence 映射到 Execution Key/Policy/Record；修复错误链并以显式 ClaimTTL 提供记录保留来源。 |
| APP-001 | L | MOD-001, CAC-002, EXE-001 | 已完成 | 新增 Application Schedule Definition、单一 gocron Adapter、任务/全局并发、有界拥塞和 cron/fixedDelay 测试。 |
| APP-002 | L | APP-001 | 已完成 | 实现逐任务执行权状态机、续租/失权、skip/pause/fail/local、自动恢复与低敏诊断；严格模式拒绝本地降级。 |
| GEN-001 | L | APP-002 | 已完成 | 实现进程稳定 ScheduleHub 和 Prepare/Commit/Retire/Abort/Stop 集成；Prepare 不触发，跨 Generation 旧代准入先关闭。 |
| SUP-001 | M | GEN-001 | 已完成 | Schedule fatal error 上送现有 factory failure channel，继续由 GenerationCoordinator monitor/Supervisor 处理。 |
| CFG-001 | M | APP-002 | 已完成 | 增加 scheduler 强类型配置、默认值、环境覆盖、上限校验、未知 Task ID 拒绝和完整 Generation reload 测试。 |
| OPS-001 | M | APP-002, GEN-001 | 已完成 | 调度快照合并到现有 Ops diagnostics/readiness；disabled/pause/skip/fail/weakened 语义由原 Health/Probe 路径呈现。 |
| INT-001 | L | SUP-001, CFG-001, OPS-001 | 已完成 | 覆盖双逻辑实例唯一触发器、协调中断/恢复、租约失权、热重载、关闭、panic containment 与 fatal path。 |
| GOV-001 | M | INT-001 | 已完成 | 审计第三方类型、Redis 构造、隐式注册、生命周期/健康/执行重复机制与失效配置，没有发现残留双轨。 |
| DOC-001 | M | GOV-001 | 已完成 | 更新开发、配置、Kernel App、pkg、Ops 与运维主题文档和导航，明确声明、保证边界、诊断与恢复流程。 |
| VER-001 | L | DOC-001 | 已完成 | gofmt、全量 test、vet、核心 race、配置回环、治理搜索、文档链接和 diff check 已执行；证据见下文。 |
| COM-001 | S | VER-001 | 已完成 | 本文件随单一 Conventional Commit 提交，未 push；hash 由 Git 历史和最终交付报告记录。 |

## 4. 实施顺序与检查点

### 阶段 A：契约与资源边界

执行 `DEP-001`、`SCH-001`、`MOD-001`、`COO-001`、`CAC-001`、`CAC-002`。完成后检查：

- 模块只依赖项目自有契约；
- Redis 连接只有一个 owner；
- 协调错误、运行期健康和严格模式边界可单测；
- 没有启动 scheduler 或业务任务。

### 阶段 B：现有治理能力接入

执行 `OBS-001`、`EXE-001`。完成后检查：

- 后台 span 复用现有 provider；
- 每次运行只走现有 Execution；
- 错误链、记录 TTL、trace identity 和低敏日志语义完整。

### 阶段 C：调度与生命周期

执行 `APP-001`、`APP-002`、`GEN-001`、`SUP-001`。完成后检查：

- Prepare 不触发、Commit 才开放准入；
- fixedDelay 是完成后延迟；
- 严格任务失权即关闭准入；
- 旧、新 Generation 不重叠；
- 终态故障进入现有 Supervisor 路径。

### 阶段 D：配置、运维与系统验证

执行 `CFG-001`、`OPS-001`、`INT-001`、`GOV-001`、`DOC-001`、`VER-001`、`COM-001`。完成后检查全部验收标准，只有证据完整且无隐藏失败时才能声明完成。

## 5. 逐轮证据

### 2026-08-20 研究与计划轮

- Git 基线：`a675f3351d80f7591ca1cb071c83e65125eb4d92`。
- 研究档案：`R001`、`R002`、`R003`。
- 计划产物：`README.md`、`requirements.md`、`design.md`、`tasks.md`。
- 已验证事实：当前没有 Schedule Binding、调度引擎或专用租约契约；Application Generation、Execution、Cache、Telemetry、Health、Supervisor 与 Runtime 均存在可复用路径。
- 文档验证：9 份 Markdown 的 49 个本地相对链接有效；3 份 metadata 必填字段、唯一 ID 与 report 目标有效；12 份任务文件均有末尾换行且无行尾空白；`git diff --check` 通过。
- 范围验证：Git 仅显示 `docs/changes/README.md` 与新建 `037-scheduled-task-capability/`，没有源码、配置、依赖或测试变更。
- 当前动作：只修改本变更文档和必要导航；未修改源码、配置、依赖、测试，未启动服务，未暂存或提交。
- 未执行：计划阶段没有 Go 实现变化，因此未运行 Go build/test/race/vet；不以未执行检查证明后续实现。
- 下一门禁：用户明确确认本计划。

### 2026-08-20 已确认实施轮

- 确认：用户在计划报告之后明确回复“确认按 037 计划实施”。
- 依赖：`github.com/go-co-op/gocron/v2 v2.22.0` 为唯一触发引擎；MIT 许可证与 Go 版本适配依据保存在 R002，`go mod tidy` 后为 direct dependency。
- 契约与边界：新增 `pkg/schedule`、`pkg/coordination`；`module.Contribution.Schedules` 是唯一业务接入点，gocron 只存在于 `internal/kernel/app/schedule`，Redis client 仍只由 Cache resource owner 创建。
- 运行治理：Scheduler Component 复用项目 Clock、Logger、Telemetry、Execution、Cache Coordination、Health/Ops、Application Generation 与既有 failure channel；没有新增 Runtime、Supervisor、Health Registry、Execution Store 或隐式 Registry。
- 行为测试：覆盖 cron 语法/时区、completion-based fixedDelay、任务/全局并发、panic containment、严格 skip/pause/fail、best-effort local、两个逻辑实例竞争、租约失权、协调自动恢复和跨 Generation 旧代准入关闭。
- Redis Adapter：miniredis 验证 token owner 的 acquire/renew/release、TTL、失权、后端中断分类和同一已构建 resource 恢复后重新获取。
- 配置与重载：`config.example.yaml`、Bootstrap/one-shot/Service 配置契约已对齐；`config init` 在 ignored `tmp/` 输出中生成 scheduler 完整配置并通过回环检查；逐配置节 reload 测试包含 scheduler。
- `go test ./... -count=1`：通过。
- `go vet ./...`：通过。
- `go test -race`：`pkg/schedule`、`pkg/coordination`、`pkg/execution`、`pkg/observability`、`internal/module`、Cache/Observability/Schedule App、`internal/composition` 与 Ops 全部通过。
- 更宽的首次 race 尝试中，既有 `internal/kernel/composition/TestHostReloadsRealSQLiteAndKeepsCrossComponentTransactionAtomic` 在 Windows 删除测试临时 `config.yaml` 时遇到 fsnotify 文件占用；该包普通测试通过，本次相关并发包随后独立 race 全部通过，未使用更强删除方式重试。
- 文档：24 份变更 Markdown 的 112 个本地相对链接有效；fenced/inline code 已排除。
- 治理搜索：业务模块没有 gocron/go-redis/internal schedule import；没有 schedule `init`/扫描/Register 流程；production Redis client 构造仍只有 Cache owner；`git diff --check` 通过。
- 提交：64 个任务文件已按路径显式暂存并通过 staged diff 检查；未 push。

## 6. Commit 与剩余风险

- Commit：本文件位于 037 的单一最终任务提交中；实际 hash 由 Git 历史和最终交付报告记录，未 push。
- 剩余风险：
  - Redis 部署拓扑和故障转移保证必须在目标部署环境中核实；本计划不把 Redis 租约扩大解释为业务 exactly-once。
  - Execution 当前仅有内存 Store，进程重启后的执行记录持久化不在本次范围。
  - 真实多实例网络分区、长 GC pause 和 Redis failover 需要环境级故障演练；单元/集成测试只能覆盖可控模型。
  - gocron v2.22.0 已在仓库 Go 版本下通过 cron/fixedDelay 与全量测试；未来升级仍需重新复核其触发、关闭和项目 Clock 输入语义。
  - Windows fsnotify race 全包尝试存在上述临时文件占用限制；当前变更的核心并发包 race 已通过。
