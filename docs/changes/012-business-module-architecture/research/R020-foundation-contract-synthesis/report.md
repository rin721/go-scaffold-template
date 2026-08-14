# R020：底层契约治理与业务设计门禁综合判定

> 状态：实施前方案综合记录，已由 [R021](../R021-foundation-closure-implementation/report.md) 替代。本文保留推荐路径和门禁依据，不代表当前代码状态。

## 1. 当前唯一结论

当前项目已经形成可保留的 Kernel 资源平面，但尚未形成从 Bootstrap、CLI、Config 到全进程运行/停止的治理闭环。因此不能继续冻结 Handler、Service、Repository、Model、Route contribution 或业务 Module API。

推荐不是整体替换架构，而是：保留显式 Plan、single composition root、stable Capability facade、Lease、默认文件安全发布和 Kernel candidate transaction；补齐 mode-specific Bootstrap、配置节单一契约、Application single candidate、Supervisor/HTTP/process state/diagnostics；优化 strict binding、终止 drain、degraded 和自动门禁。只有 Supervisor 的等待/终止协议需要基于已证实缺口重新设计。

## 2. 当前稳定、隐式与缺失

### 2.1 已稳定且应保持

- `main -> runMain -> process` 的薄入口、signal context、baseline Logger、清理后才 `os.Exit`。
- `Plan.Add/Replace/Freeze`、typed Binding/Input、唯一 production composition root。
- Fixed/Direct/Configured 与 stable facade；Logger 替换只在成功构造后发生。
- Database/Storage Lease 和 Cache backend owner；消费者不负责关闭共享资源。
- ordered DefaultBinding、全内存校验/编码、no-overwrite、显式 force、temp/Sync/Close/publish/cleanup 错误链。
- Kernel Reload 的 RestartRequired 无副作用预检、candidate prepare、drain/commit/rollback/cleanup 与旧代保持。

### 2.2 只隐式存在

- “有参数即 CLI、无参数即 Service”的运行模式和 CLI 的资源需求。
- File 后 Env 的优先级身份、nil Source、Source 值域和取消边界。
- 每字段 missing/zero/empty/disabled/default、允许的 weak conversion、validation 阶段。
- Kernel 首次启动逐组件 publish 对外不可见的假设。
- Participant/Task 完成策略、process readiness、terminal drain 与 stop timeout 的起点。
- 敏感字段的 owner 元数据、diagnostics generation 和 last failure。

### 2.3 缺失或不完整

- 完整 CLI tree 冲突、mode、required narrow capability、side-effect 与 config exit contract。
- Defaults、strict typed Binding、Validation、Change classification、Sensitivity 的同 owner Section Contract。
- 未知/重复/废弃字段治理、canonical Snapshot 值域与默认生成 round-trip。
- Application 与 Kernel 共用一次 Load/同一 digest 的 candidate coordinator。
- Supervisor 的 runtime error/nil early/uncooperative runner/StopAndWait 与全局期限。
- HTTP pre-bind/Serve/Shutdown/Close/Wait owner 和 production composition。
- process state、readiness/degraded、generation/digest/last error diagnostics。
- reload drain 与 terminal drain 分离、committed cleanup 后续 reload gate。
- package/import/constructor/registration/lifecycle 的自动化架构门禁。

## 3. 本地问题与外部做法的对应

| 本地问题 | 外部一手做法与适用条件 | 当前建议 |
|---|---|---|
| CLI command tree 与 I/O 依赖框架细节 | Cobra 显式 AddCommand/Args/Group/RunE；Kubernetes cli-runtime 显式 IOStreams | 保留 Adapter，在构树前冻结项目 registry；不换框架 |
| nil/弱参数、资源选择和副作用无契约 | 外部框架不替项目表达 mode/resource/side effect | 增加项目专用 mode 与窄依赖 metadata |
| unknown 字段忽略、weak conversion | mapstructure 明确 ErrorUnused 默认关闭且 WeaklyTypedInput 会转换空值/类型 | 开 strict unused，收敛 hook，owner 明确字段语义 |
| 重复 YAML 字段可能丢失 | Kubernetes strict serializer 在原始 YAML 上检查 duplicate，再 strict unmarshal | 借鉴边界失败原则，不引入 Kubernetes runtime |
| 候选失败不能影响 current | Caddy 完整 provision/validate 后 use，失败 cleanup candidate | 保留 Kernel transaction，补 Application single candidate |
| HTTP/runner 无运行监督 | net/http Serve/Shutdown 和 errgroup 行为要求 owner 主动协调；controller-runtime/dskit 有 blocking run/StopAndWait/state | 局部强化现有 Supervisor/httpx，不引入第二 runtime |

## 4. 方案比较

| 方案 | 收益 | 代价与风险 | 结论 |
|---|---|---|---|
| A. 保持现状直接设计业务模块 | 快速产出目录/API | 把配置漂移、假 readiness、HTTP 互锁和隐藏资源带入业务 | 拒绝 |
| B. 所有命令/业务对象/HTTP 都进入 Kernel Plan | 复用现有 API | Kernel 变业务容器，边界和 reload 复杂度失控 | 拒绝 |
| C. 以 Fx/Kratos/controller-runtime/dskit/Kubernetes runtime 整体替换 | 获得成熟部件 | 第二 container/runtime、迁移大、丢失现有 Lease/candidate 价值 | 拒绝 |
| D. 所有 config change 一律 RestartRequired | 简单 | 丢弃已验证资源换代，仍不解决 CLI/监督/HTTP/诊断 | 不推荐 |
| E. 保留 Kernel，在同一 composition root 增加 mode-specific Bootstrap、section contracts 和薄 Application coordinator | 缺口与责任一一对应，可分阶段测试和迁移 | 需 strict config 迁移与 Supervisor 核心测试 | 推荐 |

## 5. 推荐协作链路

```text
显式定义/注册/冻结
  -> 选择 Bootstrap 或 Service mode
  -> ordered Sources 只加载一次
  -> canonical immutable candidate
  -> 所有 section strict bind + semantic validate
  -> 所有 owner change preflight
  -> Kernel 资源候选 + Application/HTTP runner prepare
  -> all ready 后 Supervisor 启动并进入 process ready
  -> runtime error/signal/reload 由同一 state owner 协调
  -> reload 可 rollback；terminal 不 Resume
  -> bounded Stop/Wait/Cleanup
  -> structured redacted diagnostics + exit code
```

默认配置命令复用同一 section registrations 和 bind/validate，但在 Bootstrap mode 截止于内存编码/安全文件发布，不创建运行资源。业务对象未来仍用普通构造函数和窄 port；它们不加入 Config/Kernel registry 作为万能 provider。

## 6. 分阶段实施与依赖

| 阶段 | 目标 | 主要依赖 | 完成证据 |
|---|---|---|---|
| P0 契约冻结 | 稳定 ID/status/error/state/mode/owner 术语与现状回归 | 当前测试 | 契约测试锁定现状，所有目标仍待确认 |
| P1 Config/Bootstrap | section registration、strict bind、default round-trip、Bootstrap no-resource | P0 | unknown/duplicate/zero/default/Snapshot/CLI exit matrix |
| P2 Supervisor/State | 非空唯一 unit、runtime completion、stop total deadline、state diagnostics | P0 | event-sequence、nil early、uncooperative、error aggregation |
| P3 HTTP lifecycle | pre-bind、blocking Serve、Shutdown/Close/Wait、readiness | P2 | bind/runtime/drain/timeout/once tests 与运行证据 |
| P4 Single candidate | Application 唯一 Load、全 owner preflight、reload/terminal/degraded | P1、P2、P3 | same digest/generation、no-side-effect、rollback、degraded gate |
| P5 Governance | package/constructor/registry/lifecycle/docs 自动门禁 | P1-P4 | 常规 CI 可执行证据、旧入口/符号零残留 |
| P6 Business re-entry | 只用首个真实用例重开业务细节 | P1-P5 全过 + 用户确认真实用例 | use case/data/protocol/acceptance，新的 012 报告与确认 |

P1 与 P2 可在公共术语冻结后并行设计，但实施顺序需避免同时改动 process root。P3 依赖 P2 的监督协议，P4 依赖 Config 和所有运行 owner；P5 伴随每阶段增量建立，最后统一闭合。

## 7. 进入业务详细设计的硬门禁

必须同时具备以下可复核证据：

1. Bootstrap help/version/config init 不构造或启动 DB/Cache/Storage/HTTP；CLI conflicts、I/O、取消、副作用和退出码闭合。
2. 默认生成与 runtime 使用同一 section bind/validate；unknown/duplicate/type/missing/zero/empty/disabled/default 全部有 owner 测试。
3. Snapshot canonical、不可变、脱敏；同次 startup/reload Loader 恰好一次，所有 owner 看到同一 digest/generation。
4. Plan/registry 的空值、重复、冲突和依赖方向在副作用前失败；无 Container/Resolver/旁路构造。
5. Supervisor 对启动、running acknowledgement、runtime error、nil early、cancel、不合作 runner、Stop/Wait/timeout 有确定事件序列。
6. HTTP 端口占用同步失败、Serve 异常上报、active request 有界排空、Shutdown/Close/Wait 无互锁且 exactly once。
7. readiness 在全体 required unit running 后才 true，drain/failure 前先 false；diagnostics 有 state/reason/generation/last failure 且无 secret。
8. reload candidate 失败保持旧代；RestartRequired 无资源副作用；terminal drain 不 Resume；committed cleanup failure 进入 degraded 并限制 reload。
9. 自动治理和权威文档已与实现同步，必要 listener/signal/reload/shutdown 运行证据真实执行并记录。
10. 首个真实业务用例、数据所有权、入口协议、事务/一致性边界和验收标准已由用户确认。

前九项只解锁研究，第十项决定业务设计内容。通过门禁不自动授权实现业务模块。

## 8. 风险与迁移策略

| 风险 | 影响 | 控制/迁移 |
|---|---|---|
| strict binding 使现有宽松配置失败 | 兼容与发布 | 先盘点真实配置；只对确认的 deprecated 字段建立有期限迁移，不永久双轨 |
| Loader owner 从 Kernel 上移 | Kernel API/测试面 | 先增加 candidate 输入 seam，再单轨删除旧 Loader 入口；禁止长期双加载 |
| Supervisor 等待顺序改变 | shutdown/runtime semantics | 用事件序列和故障注入锁定，再接 HTTP；不保留两套 Supervisor |
| HTTP owner 接入 | 外部 `pkg/httpx` 消费者兼容 | 先搜索全部调用方，提供单轨迁移；不在 Start 中隐藏 goroutine |
| initial publish 一致性 | 启动失败可见性 | 先证明 Host ready 前无 borrower，或改为 all-ready publish；需实施设计确认 |
| cleanup degraded policy | 运维可用性 | 默认 readiness false、阻断 reload；是否允许部分服务需真实运维决策 |
| 文档先于实现 | 错误完成声明 | 所有目标明确“待确认”；实现后更新权威主题文档和证据 |

## 9. 未决事项

- ApplicationCommand 的首个真实需求与 capability 集合。
- strict config 迁移中哪些现有 `0` 是显式值，哪些是 missing fallback。
- Kernel initial publish 是保持逐组件但证明不可见，还是调整为批量发布。
- HTTP 管理/业务端口、timeout budget、hijacked connection 支持。
- process degraded 时 readiness、自动退出与人工处置策略。
- stop 总期限如何分配 owner 子预算，以及 cleanup 是否可安全重试。
- schema version、字段级 provenance 和远程 Source 均无当前需求，不进入首轮。

这些未决项不能以框架默认值代替项目决策；实质改变公共 API、依赖、边界或副作用时必须回到待确认。
