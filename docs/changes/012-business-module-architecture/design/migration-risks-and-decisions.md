# 迁移、风险、决策与未决项

## 1. 当前待确认决策

| ID | 决策 | 本地问题/证据 | 被拒绝方案 |
|---|---|---|---|
| `D-012-01` | 保留 Kernel Plan、stable facade、Lease 与候选事务 | 当前组件顺序/回滚/换代测试充分 | 整体重写或全部关闭 reload |
| `D-012-02` | application coordinator 是唯一 Loader/进程协调边界 | Kernel 自行 Load 无法覆盖未来 application owner | 两边各读配置；让 Kernel 接管所有应用对象 |
| `D-012-03` | Kernel 只治理底层资源，普通对象继续普通 Go 构造 | Plan 的价值来自资源生命周期，不是通用 DI | 业务对象进入 Plan；Fx/Resolver 第二容器 |
| `D-012-04` | Supervisor 区分 startup、runner 与 one-shot | Task nil 完成、Participant runtime error 和 HTTP 互锁均未闭合 | 继续复用模糊 Task/Participant 语义 |
| `D-012-05` | HTTP 预绑定 + blocking Serve + StopAndWait | net/http 语义和当前 Wait-before-Stop 冲突 | goroutine 吞 bind/serve error；每模块自启 Server |
| `D-012-06` | process readiness 与 Kernel candidate Ready 分离 | 当前 Ready 不代表 listener/runner/drain | 直接把 Kernel Ready 暴露为服务 ready |
| `D-012-07` | reload 排空可回滚，process termination 排空不可 Resume | 当前 Kernel.Stop 超时恢复 serving 与 Host 退出冲突 | 两种场景共用相同失败策略 |
| `D-012-08` | committed cleanup failure 持久 degraded/restart-required 并阻断 reload | 当前新代已服务、旧 handle 清除且无后续策略 | 伪装回滚或只打印日志继续换代 |
| `D-012-09` | 只保留当前三种换代策略 | 当前组件无 NativeAtomic/Handoff/自动回滚需求 | 为未来组件预建通用热换代 |
| `D-012-10` | package graph + 注册/生命周期测试形成治理闭环 | `internal`/文档不能表达同 module 方向 | 只靠 review；立即引入大型治理工具 |
| `D-012-11` | 业务模块细节冻结到基础门禁通过 | 十一项门禁中 lifecycle/governance/business 未满足 | 先建 Handler/Service/Repository/Model 再补底层 |
| `D-012-12` | CLI 在资源构造前以显式 mode/registry 选择最小 composition | 当前 `len(args)` 分支仍构造完整 Kernel/Plan，命令无副作用声明 | 所有命令拿全量 Application；直接更换框架 |
| `D-012-13` | 每个配置节统一 safe Defaults、strict Binding、Validation、change 和 sensitivity | unknown/weak decode 与默认生成/runtime 漂移已由代码和上游源码证实 | 巨型 Config、无类型 Map、Kubernetes schema runtime |

这些决策仍是 012 目标，不是现行 API。若公共接口、依赖、模块边界、配置迁移、数据或外部副作用发生实质变化，必须更新方案并重新确认；难以逆转的长期选择再进入 ADR。

## 2. 推荐实施顺序

### Phase A：先固化 CLI/Default/Config 契约

1. 冻结 command registry、Bootstrap/Service mode、I/O、退出码、required narrow capability 与 side-effect contract。
2. 建立 section owner registration，严格处理 Source/unknown/duplicate/type/missing/zero/disabled 和 Snapshot immutability/sensitivity。
3. 让默认配置生成通过运行期同一 binder/validator round-trip，并证明 Bootstrap 不构造/启动资源。

原因：它们决定 Application 能否获得一致、有效、无隐藏默认的输入；在这些契约不稳定时先细化运行或业务对象，只会把配置漂移扩散到更多 owner。

### Phase B：状态与 Supervisor 契约

1. 明确进程状态、termination cause、ready/degraded、owner ID 和 diagnostics snapshot 的内部契约。
2. 局部修订 Supervisor 的注册校验、runner completion、runtime failure、cancel、reverse StopAndWait 与 shutdown deadline。
3. 用 fake owner/runner 完成失败、nil 提前返回、不合作 runner 和多 cleanup error 的确定性测试。

### Phase C：HTTP 与健康诊断

1. 单轨调整/封装 `pkg/httpx.Server`，支持预绑定 listener 与阻塞 Serve。
2. 接入唯一 HTTP owner，验证端口失败、runtime error、活跃请求 drain、Stop/Wait timeout。
3. 将 lifecycle state 与必要 `pkg/health` checker 组合为 readiness/liveness/diagnostics；不提前加入业务路由。

### Phase D：单候选和 reload/termination 协调

1. 上移 Loader 调用权，同一 immutable candidate 提供给 Kernel 与 application owners。
2. 在 Kernel 副作用前完成所有 decode/validate/RestartRequired preflight。
3. 分离 reload drain 与 terminal drain；持久化 committed cleanup degraded 并阻断不安全 reload。
4. 单轨迁移入口并删除旧 Load/Start 路径。

Phase A 的 Config 与 Phase B 的 Supervisor 可以在不同时修改 composition root 时并行设计；HTTP 必须等待 Supervisor，single candidate 必须等待 strict Config。若实施设计发现 API 依赖相反，可调整小步顺序，但必须证明每个中间状态可测试、无新旧双轨。

### Phase E：可执行治理与基础关闭

1. 对真实 package graph、唯一 composition、注册冲突和隐藏 fallback 建门禁。
2. 执行 race/lifecycle/loopback 集成、静态检查和文档同步。
3. 逐项关闭 acceptance matrix；形成可复核运行证据。

### Phase F：业务设计（当前阻塞）

只有 Phase A-E 全部通过，才收集首个真实业务用例并继续 012 的模块边界、Handler/Service/Repository/Model 设计。没有用例不创建骨架。

## 3. 主要风险与缓解

| 风险 | 影响 | 缓解/停止条件 |
|---|---|---|
| Loader 所有权和 Kernel API 变化 | 触及启动/reload 核心 | candidate identity、旧代保留和单次 Load 测试；公共 API 实质变化重新确认 |
| strict binding 拒绝既有宽松配置 | 兼容与启动风险 | 先盘点真实配置；只对已确认 deprecated 字段给出有期限迁移，完成后单轨删除 |
| Bootstrap composition 拆分不完整 | help/config init 仍受资源图影响 | constructor spy 与 goroutine/listener/connection 断言；不把“未 Start”当无副作用证明 |
| Supervisor 修改形成竞态或泄漏 | 进程假 ready、无法退出 | channel/event 测试、race、owner/runner 计数；不用 sleep |
| HTTP 外部消费者依赖旧阻塞 Start | 破坏兼容 | 实施前搜索真实调用方；无消费者单轨替换，有消费者先补迁移范围 |
| ShutdownTimeout 仍不能终止不合作 goroutine | Host 永久阻塞 | deadline 覆盖 Stop+Wait；定位 owner并失败退出，不宣称强杀成功 |
| terminal drain 强关活跃资源 | use-after-close/数据损坏 | 先阻止新借用，超时失败退出；不在活跃 Lease 下强制 Close |
| cleanup degraded 处理过度泛化 | 重试 Close 可能不安全 | 默认阻断 reload并要求 restart；按资源证明可重试后再扩展 |
| diagnostics 泄露配置/错误细节 | 安全和运维风险 | 只输出安全 ID/digest/classification，redaction 测试 |
| coordinator 演变为巨型容器 | 依赖/配置重新隐藏 | schema 和 resource 仍由 owner 定义；禁止 Resolver/`any`/全量 Capabilities 注入 |
| 治理规则绑定虚构业务路径 | 误报和维护成本 | 先覆盖现有 Kernel/composition；真实模块出现后扩展 |
| 文档领先实现 | 使用者误判 | 012 始终标待确认，实施后同步权威主题文档 |

## 4. 兼容、删除与回滚边界

- 当前架构是兼容基线；优先保持 Capabilities/Lease/配置语义，局部调整 owner 与入口。
- 尚无真实外部消费者时直接单轨替换 Loader/Host/httpx 旧路径，不保留 `legacy`、feature flag 或静默 fallback。
- 若搜索发现外部 consumer，先记录兼容范围、截止、观测和删除计划并重新确认，不能默认双轨。
- 实施失败可以回到上一个完整 commit；不得在一个进程中永久保留两套 Supervisor 或两次 Load 路径作为“回滚”。
- 数据库数据、迁移、部署和外部协议不在本任务授权范围。

## 5. 当前未决项

### 基础闭环必须在实施前/实施中回答

1. 现有 Supervisor 接口是直接单轨修改还是由内部受管单元替代；真实调用方有哪些？
2. process state/diagnostics 放置在哪个现有边界，如何避免 health 反向拥有 lifecycle？
3. startup/shutdown 总期限沿用何处配置，是否需要阶段预算而不预设每 owner 配置？
4. HTTP 管理端点与业务端点是否共 listener；当前没有需求时是否只提供内部 probe/test seam？
5. 是否有 hijacked connection 的真实协议需求；没有则明确不支持/不保证排空。
6. committed cleanup failure 的资源是否有可重试 Close 证明；默认重启策略是否满足运维要求？
7. `pkg/httpx`、`pkg/health` 和 Kernel APIs 是否存在仓库外 consumer/兼容承诺？
8. 各 Config 字段中显式 `0`、空字符串、null 与 missing 的真实兼容语义分别是什么？
9. 默认文件 force 并发、目录 fsync 与支持的 OS/filesystem 保证范围是什么？
10. Kernel initial Start 是证明逐组件 PublishInitial 在 process ready 前不可见，还是改成 all-ready 后统一发布？

### 业务门禁后再回答

首个真实业务能力、actor、数据/事务 owner、HTTP/CLI 入口、缓存、I18n 和公共错误协议。它们不应反向改变底层 owner；若确实需要公共 API/依赖/边界变化，重新确认。

## 6. 明确拒绝项

- Fx/Kratos/controller-runtime/dskit/Caddy 作为第二套进程 runtime；
- Kernel 业务对象容器、运行时 Resolver、扫描、`init` Registry；
- 通用动态对象图、路由/listener 热重建、自动观测回滚；
- 只靠 health check 掩盖 lifecycle state；
- 只写日志后吞掉 runner/cleanup/stop error；
- 为业务门禁创建空模块、占位 Handler/Repository 或假数据。
