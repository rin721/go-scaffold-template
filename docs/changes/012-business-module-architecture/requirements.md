# 产品需求：业务模块架构前置闭环

## 1. 背景与问题

仓库已经实现显式 Kernel Plan、配置候选事务、stable Capability facade、Lease、Host 和可选 CLI，但还没有完整 application composition、HTTP listener、生产 readiness/diagnostics 或真实业务模块。首轮 012 在此基础上直接细化了模块分层；本轮复核证明 Kernel 内部资源事务与全进程生命周期不是同一层面的“闭环”。

因此 012 当前先回答：从配置输入到验证的底层责任、契约、状态和失败语义是否完整，是否已有足够证据允许继续业务设计。事实详见 [current-facts-and-gaps.md](requirements/current-facts-and-gaps.md)，外部比较与推荐见 [R016](research/R016-foundation-gate-synthesis/report.md)。

## 2. 当前结论

- **局部闭环**：Kernel 管理的既有资源在构造、启动、发布、重载、排空、回滚和清理方面有较完整代码与测试。
- **全进程未闭环**：application 配置、长期 runner、HTTP、service readiness、终止排空、degraded 诊断和架构治理仍缺少统一 owner 与可执行证据。
- **演进策略**：保持 Kernel 核心，局部补齐其上层控制面；不整体重写，不扩大 Kernel 为业务容器。
- **业务门禁**：底层闭环验收全部满足前，Handler/Service/Repository/Model 只保留方向性约束，不继续确定 API、目录或实现。

## 3. 本轮目标

### 3.1 基础闭环目标

- 让同一不可变配置候选覆盖 Kernel 与 application-owned 配置，唯一 Loader 调用者和各节 owner 可定位。
- 让启动、阻塞运行、运行期异常、取消、排空、反序停止、等待和超时形成单一 Supervisor 闭环。
- 让 HTTP listener 的绑定失败、Serve 退出、Shutdown/Close/Wait 和活跃请求排空由唯一 owner 管理。
- 区分 Kernel candidate `Ready`、进程 readiness、liveness 和 degraded，形成可查询的状态与安全诊断。
- 区分 reload 可回滚排空和进程终止排空；明确 committed cleanup failure 的持久状态与后续动作。
- 用 package graph、唯一注册校验、事件序列测试和运行证据约束架构，而不只依赖文档。

### 3.2 业务延伸目标

本轮不设计具体业务 API。基础门禁通过后，012 才恢复以下方向的详细化：按业务能力纵向组织、普通 Go 构造函数、使用方定义窄 port、项目 Adapter、HTTP/CLI 复用 Service。真实需求不支持的层、接口和机制不创建。

## 4. 范围

### 4.1 包含

- 配置输入、Plan、Kernel、Host、Supervisor、Watcher、Lease、httpx、health 和相关测试的静态审计。
- 单一候选协调、运行监督、状态/诊断、HTTP lifecycle、重载/终止语义和治理门禁的目标设计。
- 保留/补齐/局部优化/整体替换候选的比较、迁移成本、风险、验证和未决项。
- 对原 012 业务设计的状态校正及后续解锁条件。

### 4.2 不包含

- 本轮任何源码、配置、依赖、脚本、测试或生成物修改，以及启动、生成、stage、commit、push 或外部写入。
- 无真实需求的业务实体、Handler、Service、Repository、Model、路由、命令或缓存策略。
- runtime DI、自动扫描、全局 Registry、Service Locator、动态插件、消息总线、Saga/CQRS/Event Sourcing。
- 未经证据需要的通用热重建、动态路由、listener handoff、自动观测回滚或完整服务框架。

## 5. 核心需求

### 5.1 配置与装配

- Loader 只有一个进程级调用者；Kernel 与 application owner 必须从同一候选解码和校验。
- 所有 RestartRequired 判定在 Kernel/外部资源副作用之前完成；候选只能整体接受或拒绝。
- Kernel Plan 继续只治理当前底层资源；普通业务对象和协议入口不进入 Plan。
- dependency、config source、constructor、runner 和 resource owner 都可从唯一 composition root/显式契约追溯。

### 5.2 监督与生命周期

- 区分启动项、长期阻塞 runner 和 one-shot operation，不用“返回 nil”表达多种隐式语义。
- 非空唯一 ID、固定启动顺序和严格反向停止在构造/启动前校验。
- 关键 runner 任何非预期完成都上报 process owner；错误触发取消，但取消不代替 owner Stop/Wait。
- startup 与 shutdown 有明确总期限；超时指出未退出 owner，不宣称已经安全清理。
- reload 排空失败可回滚；终止排空进入 terminal 路径，不恢复 ready/serving。

### 5.3 HTTP 与就绪

- listener 在启动阶段预绑定，端口/地址失败同步返回；Serve 是受监督的阻塞运行单元。
- Shutdown/Close/Wait、活跃请求和未来 hijacked connection 的责任明确；没有需求时不承诺 WebSocket 管理。
- readiness 只有在必需资源、runner 和 listener 都运行后才为 true；drain、关键 runner 失败或不可接受 degraded 时先变 false。
- liveness 只回答进程监督循环是否仍能工作，不把外部依赖失败一律变成重启风暴。

### 5.4 重载、异常与诊断

- 诊断至少可关联 lifecycle state、generation、snapshot digest、last reload 和 committed cleanup 状态，且不泄露敏感配置。
- 候选失败保持旧代；提交后清理失败不得伪装回滚，必须标记 degraded/restart-required 并阻断不安全的后续 reload。
- 主错误、运行错误、取消、超时、Stop/Close/Wait 错误均保留原因链和阶段/owner 上下文。
- Logger 只在真正决定策略的边界记录一次；baseline Logger 始终覆盖早期失败和最终停止诊断。

### 5.5 治理

- 基于解析后的 Go package graph 验证禁止依赖方向；`internal` 和 grep 不能作为唯一证据。
- contribution/Participant/Task/runner 的 ID、重复、顺序和规范化使用生产同源规则验证。
- 每条架构规则有违规样例和合法样例；生命周期测试使用 channel/event，不用 sleep 碰运气。
- 当前文档只描述真实实现；012 作为待确认变更记录，不成为第二套现行规范。

## 6. 十一项通用门禁

所有候选必须同时通过事实、目标、边界、装配、生命周期、一致性、错误、治理、演进、复杂度和业务延伸门禁。逐项当前状态及证据见 [acceptance-matrix.md](requirements/acceptance-matrix.md)。任何关键项未满足时，不得用“目录设计完成”替代底层闭环。

## 7. 成功判定

基础批次只有在以下事实全部可执行验证后才完成：单一候选、全进程 Supervisor、HTTP 预绑定与排空、readiness/degraded 状态、reload/终止差异、错误合并和自动治理均有成功/失败/超时证据；权威文档同步，旧入口/双轨删除，未执行项明确。

满足这些条件只解除业务设计阻塞。首个真实垂直切片仍需单独确认业务 actor、用例、不变量、数据/事务、入口和验收数据。
