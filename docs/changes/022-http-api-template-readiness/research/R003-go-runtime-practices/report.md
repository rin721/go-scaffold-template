# R003：成熟 Go 项目的装配、运行与清理实践对照

## 1. 研究方法

本轮只选与当前具体缺口直接相关、仍在维护或具有官方地位的一手资料，不做框架名单：Go 标准库、Uber Fx、controller-runtime、Grafana dskit、Caddy 和 Google Wire 官方仓库。比较维度是 owner、ready、长期运行、停止终态、配置代际和失败诊断。

## 2. 主源事实

### 2.1 Go `net/http`：优雅与强制终止是两个显式动作

[Server.Shutdown](https://pkg.go.dev/net/http#Server.Shutdown) 先关闭 listener、再关闭 idle connection，并等待 active connection 回到 idle；context 到期时返回 context error。[Server.Close](https://pkg.go.dev/net/http#Server.Close) 是立即关闭 active connection 的另一条语义，而且 hijacked connection 仍需协议 owner 用 `RegisterOnShutdown` 自行处理。

影响：有界退出不意味着可以遗忘未结束 owner。项目需要明确“宽限期已过”之后是保留待清理、执行资源专用 force、还是由进程监督器非零退出；不能把这些状态都叫 stopped。

### 2.2 Uber Fx：正序 Start、反序 Stop 与 hard timeout

[Fx Lifecycle](https://uber-go.github.io/fx/lifecycle.html) 规定 OnStart 按追加顺序执行、OnStop 反序执行，并对两类 hook 强制 timeout；长期任务不能同步阻塞 hook，而应由 hook 启动并在 Stop 中结束。

相同点：当前 Supervisor 的正序启动、反序停止和 timeout 方向正确。不同点：Fx 的生命周期 hook 不表达本项目的不可变配置候选、跨组件原子提交、Lease drain 和旧代清理。替换为 Fx 只会换装配机制，不会自动关闭当前缺口。

### 2.3 controller-runtime：长期 Runnable 与健康贡献是一等契约

[Manager](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/manager) 的 Runnable `Start(ctx)` 必须阻塞到 context 关闭或失败；Manager 提供 GracefulShutdownTimeout，并分别提供 AddHealthzCheck 与 AddReadyzCheck。

影响：未来业务后台 consumer 若出现，应以 composition 可治理的长期任务接入，而不是在 Participant.Start 中私起 goroutine。Health/Ready 也应由显式 owner 贡献；但当前没有真实后台业务时，不必照搬 controller-runtime 的完整 Manager API。

### 2.4 Grafana dskit：terminal state 必须与真实结束一致

[dskit services](https://pkg.go.dev/github.com/grafana/dskit/services) 使用 New/Starting/Running/Stopping/Terminated/Failed 显式状态；Manager 只有在全部 service 都进入 Terminated 或 Failed 时才算 stopped，并可按状态列出 service，FailureWatcher 观察失败。

影响：当前 Kernel 在某些组件尚未关闭时先标 stopped，与这一终态原则不一致。项目不必引入 dskit，但应保留 pending owner 并让 diagnostics 能回答“谁还没结束”。

### 2.5 Caddy：不可变配置代际方向正确，清理结果影响退出语义

[Caddy Architecture](https://caddyserver.com/docs/architecture) 将配置视为不可变原子单元：新配置先 provision/validate/start，成功后再清理旧配置；失败保留旧配置。这与当前 Kernel 的候选、提交和旧代清理模型高度一致。

[Caddy command lifecycle](https://caddyserver.com/docs/command-line#signals) 还区分 graceful exit、第二次信号 force exit，以及 cleanup failed 的退出码。它没有把“退出有界”和“资源已清理”混成一个布尔值。

影响：应保留当前代际切换，不改成每个配置字段热更新；同时补齐清理债务的 owner、状态和最终政策。

### 2.6 Wire 与 Fx 的 DI 选择不改变当前判断

[Go Wire 介绍](https://go.dev/blog/wire) 说明生成代码可保持依赖图静态可读；但 [Wire 官方仓库](https://github.com/google/wire) 已于 2025-08-25 归档且明确不再维护。当前手工装配规模可控，不应为省几段 composition code 引入停更生成器。

[Fx](https://pkg.go.dev/go.uber.org/fx) 仍是成熟 v1 Runtime DI 方案，适合大量可复用模块与生命周期 hook；但当前项目明确追求静态、可追踪、copy-owned 边界，且已经有项目特有的配置事务。没有证据支持承担全量迁移和反射式容器复杂度。

## 3. 方案比较

| 方案 | 收益 | 代价/风险 | 对当前缺口的作用 | 结论 |
| --- | --- | --- | --- | --- |
| 保留手工 composition + Kernel/Supervisor，局部加固 | 保护已验证代际、边界和业务代码；改动集中 | 需要自行完善状态机和失败测试 | 直接解决 owner/cleanup/diagnostics | **推荐** |
| 用 Fx 替换装配与生命周期 | 通用模块和 hook 生态成熟 | 运行期容器、迁移面大、双生命周期过渡复杂 | 不提供原子 reload/Lease cleanup | 不推荐 |
| 引入 Wire 生成装配 | 静态生成、编译期图 | 官方已停更；生成流程增加复制与升级责任 | 不解决生命周期 | 不采用 |
| 扩成通用 DAG/插件 Runtime | 理论上适配任意组件 | 抽象面和治理成本显著增加 | 真实需求未知，易形成万能容器 | 不采用 |
| 删除 reload，全部 restart-only | 大幅简化代际状态 | 丢失已验证的 Logger/Database 等热切换价值 | 可绕开而非解决；且是破坏性变化 | 无新证据不采用 |

## 4. 适合本项目的最小原则

1. **显式 owner**：资源、participant、runner、health check 和 cleanup debt 都能定位 owner。
2. **真实终态**：只有资源和长期责任已结束，或已被明确归类为 failed/forced，才能报告 stopped。
3. **两阶段终止**：先停止接入并优雅等待；期限后进入显式失败/强制政策，不静默遗忘。
4. **不可变代际**：保留“准备新代、原子提交、清理旧代”，不要转成热路径字段锁。
5. **场景化扩展**：同步 HTTP 模块沿当前 Contribution；后台任务或动态 health 出现时才扩展窄契约。
6. **不换容器治生命周期**：DI 工具只解决装配图，不替代资源事务和错误语义。

## 5. 证据强度与局限

- Go 标准库和各项目官方文档：高；可证明公开语义，不证明内部所有 corner case。
- Caddy 与当前项目的相似性：中高；两者都使用不可变代际，但 Caddy 是 server platform，不应复制其插件模型。
- Fx/Wire 选型结论：中高；维护状态和能力边界明确，但未来 composition 规模可能变化。
- controller-runtime/dskit 的模块扩展启示：中；它们面向控制器/服务组，当前项目只应提取 owner、terminal、health 原则。

未知包括具体部署预算、资源 force-close 安全性和首个后台业务需求。后续实现必须用本地故障注入验证，不能仅凭外部项目类比宣布通过。
