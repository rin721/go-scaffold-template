# R013：运行监督与状态模型

## 1. 当前问题

本项目已有轻量 Supervisor，但没有 Participant 运行期失败通道、Service readiness 或持久生命周期状态。研究目标是比较成熟实现的责任拆分，而不是寻找替代 Kernel 的框架。

## 2. 外部项目真实做法

controller-runtime v0.24.1 的 `Runnable.Start(ctx)` 是长期阻塞调用。Manager 把 runnable error 送入统一错误通道，触发 coordinated shutdown；runnable group 负责启动确认、停止信号和 StopAndWait，Manager 同时管理 healthz/readyz 与 graceful shutdown timeout。其 HTTP server 支持预绑定 listener，运行错误返回 owner。

Grafana dskit `services` 把 New、Starting、Running、Stopping、Terminated、Failed 建模为可查询状态，提供 AwaitRunning/AwaitTerminated、Manager 和 FailureWatcher。它说明“Start 返回过”与“服务正在运行/健康”不是同一事实。

来源：[controller-runtime manager 源码](https://github.com/kubernetes-sigs/controller-runtime/tree/v0.24.1/pkg/manager)、[dskit services 文档](https://pkg.go.dev/github.com/grafana/dskit/services)。

## 3. 适用条件与不可照搬部分

- 可迁移：阻塞 runner、统一错误通道、启动确认、停止后等待、显式状态、ready 在 drain 前立即撤销。
- 需适配：本项目已有 Kernel 的资源候选事务和 Host 顺序，不能再引入第二套 lifecycle owner。
- 不适用：controller-runtime 的 leader election、webhook/cache/controller 分组；dskit 的完整 Service/Manager API。当前没有这些需求，照搬会增加重复抽象。

## 4. 方案与推荐

| 方案 | 收益 | 代价 | 判定 |
|---|---|---|---|
| 继续只有 Participant.Start/Stop + Task | API 小 | 运行错误、ready、Stop/Wait 不能闭合 | 不足 |
| 引入 controller-runtime 或 dskit | 现成监督/状态 | 与 Kernel/Host 重叠，迁移和认知成本高 | 拒绝 |
| 局部扩充现有 Supervisor | 保留兼容基线，只补真实缺口 | 需严谨定义模式与状态 | 推荐 |

目标语义最少应包含：非空唯一 ID；启动阶段与阻塞运行阶段分离；关键 runner 的任何提前完成都是事件；运行失败触发全局取消；stop 与 wait 在统一总期限内完成；状态至少可区分 starting、ready/running、draining、stopped、failed/degraded；主错误与清理错误全部保留。

状态不是为了实现通用编排器，而是为 readiness、退出码、reload 诊断和测试提供单一事实源。Health checker 只读取状态/执行有界检查，不反向拥有生命周期。

证据强度：高（官方源码/文档）；迁移收益为中高（与本地确定性缺口直接对应）。验证应使用事件序列而非 sleep，覆盖启动失败、运行失败、signal、nil 提前完成、Stop error、Wait timeout 和 ready/drain 转换。具体类型名和是否扩展现有接口仍是实施细节。
