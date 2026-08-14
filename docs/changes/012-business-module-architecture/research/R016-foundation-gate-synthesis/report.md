# R016：底层闭环综合判定

> 状态：历史记录，已由 [R020](../R020-foundation-contract-synthesis/report.md) 单轨替代。本文保留 2026-08-14 前一轮证据，不再代表 012 当前完整结论。

## 1. 当前唯一结论

当前底层装配和治理 **尚未形成全进程闭环，不能进入 Handler、Service、Repository、Model 的详细设计或实现**。但问题集中在 Kernel 上层的进程控制面，而不是 Kernel 核心方向错误：应保留显式 Plan、stable facade、Lease 和候选事务，实施局部优化与缺口补齐，不整体替换架构。

## 2. 十一项门禁判定

| 门禁 | 当前判定 | 证据与缺口 |
|---|---|---|
| 事实 | 部分满足 | Kernel 源码/测试充分；完整进程链尚无实现/运行证据 |
| 目标 | 部分满足 | 资源换代解决真实问题；既有业务细节早于底层门禁，需冻结 |
| 边界 | Kernel 内满足、应用层缺失 | Capability/Adapter 边界清晰；application owner 尚不存在 |
| 装配 | 底层满足、全应用缺失 | Plan 显式可审查；HTTP/runner/module 没有统一 composition |
| 生命周期 | 不满足 | Supervisor 等待顺序、运行错误、终止排空、HTTP 未闭合 |
| 一致性 | 部分满足 | Kernel 候选事务强；整份 snapshot 与 cleanup degraded 未闭合 |
| 错误 | 部分满足 | Kernel 原因链较好；nil 提前退出、cleanup/stop 最终语义不足 |
| 治理 | 不满足 | 有局部测试，无 import、注册、全链路和运行证据门禁 |
| 演进 | 满足方向 | 可在现有边界上分阶段单轨修改，不需一次重写 |
| 复杂度 | 推荐方案满足 | 薄协调层补真实缺口；通用容器/动态机制收益不足 |
| 业务延伸 | 不满足 | 前十门禁尚未有可验证证据，VSL 必须阻塞 |

## 3. 候选方案比较

| 候选 | 收益 | 代价与风险 | 结论 |
|---|---|---|---|
| A. 保持现状，直接加业务模块 | 最快出现目录/Handler | 配置撕裂、HTTP 假启动、停止互锁和无 readiness 会被带入业务 | 拒绝 |
| B. 把 HTTP/业务全部放入 Kernel Plan | 复用现有事务 API | 混淆资源平面和对象图，把 Kernel 扩成业务容器 | 拒绝 |
| C. 用 Fx/Kratos/controller-runtime/dskit 整体替换 | 获得成熟生命周期部件 | 第二容器/运行时、迁移大、破坏现有 Lease/reload 价值 | 拒绝 |
| D. 取消 reload，全部 RestartRequired | 语义简单 | 丢弃已实现验证的换代能力，不能解决监督/HTTP/治理 | 不推荐 |
| E. Kernel 上方薄 application coordinator，局部强化 Supervisor/httpx/health/governance | 保留兼容基线，缺口与修改一一对应，可分步验收 | 需触及启动与监督核心，必须严格测试 | 推荐 |

E 优于其他方案的原因不是抽象更“先进”，而是它同时满足证据、目标、边界、演进和复杂度门禁：每个新增责任都对应本地已复现问题，且不引入第二个依赖解析/生命周期中心。

## 4. 推荐目标闭环

1. application coordinator 成为 Loader 唯一调用者，所有配置 owner 对同一不可变候选先解码/校验/预检，再交给 Kernel。
2. Kernel 继续唯一拥有底层资源换代；普通业务对象不进入 Plan。
3. Supervisor 明确 startup、blocking run、runtime failure、cancel、reverse stop、wait 和总期限；Service 与 one-shot 模式区分完成语义。
4. HTTP 预绑定 listener，Serve 作为受监督运行单元，Shutdown/Close/Wait 由唯一 owner 管理。
5. 进程状态至少可观察 starting、ready/running、draining、stopped、failed/degraded；readiness 与 Kernel candidate Ready 分离。
6. reload 排空可回滚；终止排空不 Resume。Committed cleanup failure 持久 degraded，并在重启/明确处置前阻断后续 reload。
7. 基于真实 package graph、注册表和事件序列建立可执行治理；文档不是唯一门禁。

## 5. 业务延伸前的可验证条件

- 同一启动/reload 候选只加载一次，application 不可变配置变化在 Kernel 副作用前返回 RestartRequired。
- Supervisor 对启动失败、运行期 error、关键 runner nil 提前完成、signal、超时、Stop/Wait error 都有确定事件序列和退出结果。
- HTTP 端口占用同步失败；正常排空等待活跃请求；运行异常触发全局失败；不存在“Wait Task 才 Stop Server”的互锁。
- readiness 在全体必需单元运行后才为 true，drain/失败时先变 false；诊断暴露 generation、digest 与 last reload/cleanup 结果且不泄密。
- reload/终止排空与 committed cleanup degraded 的状态、后续动作和错误链通过测试。
- 非空唯一 ID、依赖方向、composition-only construction 和无隐藏 fallback 有自动门禁。
- 现行文档与真实实现同步，运行证据可复核，所有未执行项明确标注。

这些条件全部通过后，只能解锁“以首个真实用例继续细化业务设计”，并不自动批准任何虚构业务实现。

## 6. 风险与未决

主要迁移风险是 Loader 所有权/Kernal API、Supervisor 等待顺序、HTTP 外部消费者兼容和终止排空语义。应按诊断状态 → Supervisor → HTTP → 单快照/重载协调 → 治理的依赖顺序落地，或在详细设计中证明更安全的顺序；每步必须保持单轨并删除旧入口。

未决问题：管理/业务端口是否分离、ready 检查集合与超时预算、hijacked connection 支持、cleanup error 是否可按资源重试、Application CLI 是否有首个真实需求。它们不能用通用框架默认值代替项目决策。
