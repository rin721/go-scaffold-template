# 目标设计：底层闭环与业务解锁

## 1. 文档性质

本文保存 012 的 **已确认基础设计与业务候选边界**。FND/GOV-F 已按本设计实施；实现快照见 [R021](research/R021-foundation-closure-implementation/report.md)，实施前事实见 [current-facts-and-gaps.md](requirements/current-facts-and-gaps.md) 和 [R017](research/R017-current-contract-inventory/report.md)。业务对象图仍是未确认候选，不是当前实现说明。

## 2. 决策摘要

当前架构选择“**保持核心、局部优化、补齐闭环**”：

1. 保持现有 Kernel 资源平面：显式 Plan、stable facade、Lease、候选事务和三种当前换代策略。
2. 在资源构造前选择 Bootstrap/Service mode；CLI registry 显式声明 command path、最小依赖、I/O、退出和副作用。
3. 让每个配置节 owner 统一关联 safe Defaults、strict typed Binding、Validation、change class 与 sensitivity，生成和运行期复用同一契约。
4. 在 Kernel 上方建立薄 application coordinator，成为单一配置候选、进程模式、Supervisor 和诊断状态的唯一协调边界。
5. 局部修订现有 Supervisor/httpx/health，使启动、长期运行、失败、排空、停止和等待可组合。
6. 用当前项目自己的小型契约和测试实现，不引入 Fx、Kratos、controller-runtime、dskit、Kubernetes runtime 或 Caddy 作为第二套 runtime。
7. 在基础闭环验收前冻结业务对象图细节；通过后仍按普通 Go 构造和模块化单体方向演进。

## 3. 目标责任图

```text
cmd/app
  └─ application coordinator（唯一进程协调边界）
       ├─ mode selector：Bootstrap / Service
       ├─ command registry：conflict / IO / side effect
       ├─ Loader：一次生成 canonical immutable candidate
       ├─ section owners：default / strict bind / validate / preflight
       ├─ Kernel：底层资源 Plan 与 generation transaction
       ├─ application composition：当前只连接已存在入口和运行单元
       ├─ Supervisor：start / run / fail / drain / stop / wait
       └─ diagnostics：state / ready / generation / reload outcome
            ├─ config watcher runner
            └─ HTTP lifecycle unit（已接入；当前只返回 404）
```

唯一性不是把所有逻辑塞进 coordinator。各配置节、资源、runner 和检查仍由语义 owner 定义；coordinator 只保证同一候选、阶段顺序、状态提交与失败传播。

## 4. 关键状态与顺序

进程最少区分：`new -> starting -> running -> reloading/draining -> stopping -> stopped`，任一阶段可进入 `failed`；新代已提交但旧代清理失败进入 `degraded`。`degraded` 是否仍 ready 由失败分类决定，但必须可观察；当前推荐把 committed cleanup degraded 标记为 restart-required 并阻断后续 reload。

Service mode 的目标顺序：加载一次候选 → 全部 owner 校验/预检 → 构建/冻结 Kernel → 建立监督与 HTTP 单元 → 启动 Kernel → 启动其他 owner → 预绑定 HTTP → 启动阻塞 runners → ready。终止时先撤销 ready/停止接单，再取消 runner，按依赖反序 StopAndWait，最后停止 Kernel 与 baseline Logger。

Reload 是另一事务：整份候选预检 → Kernel prepare → reload drain → commit/resume → cleanup。reload drain 失败可回滚；process termination drain 不回滚成 serving。

详细契约见 [CLI 与默认配置](design/cli-and-default-config-contracts.md)、[Config 契约](design/config-contracts.md) 和 [运行责任图与状态机](design/runtime-state-machine.md)；原底层专题见 [foundation-closure.md](design/foundation-closure.md) 与 [composition-and-lifecycle.md](design/composition-and-lifecycle.md)。

## 5. 保留与调整

| 现有能力 | 决策 | 原因 |
|---|---|---|
| Kernel Plan / Freeze / typed Binding | 保留 | 依赖显式、当前测试充分 |
| stable facade / Lease | 保留 | 支持候选准备时旧代持续服务及安全排空 |
| Direct / KernelInstanceSwap / RestartRequired | 保留 | 已覆盖当前组件；高级 reload 没有真实需求 |
| Host 正序 Start/反序 Stop | 保留原则、调整监督实现 | 顺序正确，但 Task Wait 与 Stop 组合不闭合 |
| Kernel 自行调用 Loader | 已替换为 Coordinator 唯一调用 | 全应用需要同一候选和全 owner 预检 |
| `pkg/cli` + Cobra Adapter | 保留并补项目 registry/mode | 框架不表达资源、副作用与项目退出契约 |
| ordered DefaultContract / file writer | 保留并补 typed round-trip | 写入较强，但生成与运行期契约尚未闭合 |
| `DecodeSection(WeaklyTypedInput)` | 已单轨替换为 strict binding | unknown/type 必须在副作用前失败 |
| `httpx.Server.Start` 阻塞 ListenAndServe | 已替换为 Start/Run/Stop owner | 预绑定、运行错误回传和 StopAndWait 已闭合 |
| `pkg/health.Registry` | 已接入 Host seam | 不引入新 health 框架或未授权管理端点 |
| 业务模块细节 | 冻结 | 底层业务延伸门禁未通过 |

## 6. 候选比较

- 直接加业务模块会固化配置撕裂、HTTP 假启动和停止互锁，拒绝。
- 把业务对象加入 Kernel 会混淆资源事务与普通对象构造，拒绝。
- 整体引入 Fx/controller-runtime/dskit 等会形成第二套 owner，迁移收益不足，拒绝。
- 全部改为进程重启虽更简单，却丢弃现有已验证 reload，且不解决监督/诊断，拒绝。
- 薄 coordinator + 局部增强与每个已证实缺口一一对应，迁移边界最小，推荐。

## 7. 主题设计状态

- [底层闭环](design/foundation-closure.md)：已实施设计记录。
- [CLI 与默认配置契约](design/cli-and-default-config-contracts.md)：已实施的 CLI/Bootstrap 设计记录。
- [Config 契约](design/config-contracts.md)：已实施的 Config/Default/Binding/Snapshot/Reload 设计记录。
- [运行责任图与状态机](design/runtime-state-machine.md)：已实施的 Composition/Capability/Lifecycle 设计记录。
- [装配、配置与生命周期](design/composition-and-lifecycle.md)：既有专题细节；与上述新契约冲突时以上述三份单轨结论为准。
- [模块边界](design/module-boundaries.md)：底层门禁后的方向性约束，冻结公共形态。
- [HTTP 与 CLI](design/inbound-http-and-cli.md)：HTTP lifecycle 属于基础闭环；业务 Handler/Command 细节阻塞。
- [基础设施与跨模块](design/infrastructure-and-cross-module.md)：候选业务约束，阻塞。
- [错误、诊断与 I18n](design/errors-observability-and-i18n.md)：底层诊断先行，业务呈现规则阻塞。
- [新模块黄金路径](design/module-development-guide.md)：门禁通过前不可执行。
- [迁移、风险与决策](design/migration-risks-and-decisions.md)：单轨批次、回滚和未决项。

## 8. 业务解锁条件

[acceptance-matrix.md](requirements/acceptance-matrix.md) 的基础门禁已有本地可复核证据，因此允许用户把首个真实用例带回 012 继续细化模块设计。基础门禁通过不自动授权实现任何业务模块；必须先满足 `AC-BIZ-GATE-002` 并重新确认业务方案。
