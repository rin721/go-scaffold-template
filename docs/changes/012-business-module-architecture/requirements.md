# 产品需求：业务模块架构前置闭环

## 1. 背景与问题

仓库在实施前已有显式 Kernel Plan、配置候选事务、stable Capability facade、Lease、Host 和可选 CLI，但还没有完整 application composition、HTTP listener、生产 readiness/diagnostics 或真实业务模块。首轮 012 在此基础上直接细化了模块分层；复核证明 Kernel 内部资源事务与全进程生命周期不是同一层面的“闭环”。

012 已回答并实施从 Bootstrap、CLI、配置输入到停止清理的底层责任、契约、状态和失败语义。实施前事实见 [current-facts-and-gaps.md](requirements/current-facts-and-gaps.md) 与 [foundation-contract-catalog.md](requirements/foundation-contract-catalog.md)，实施证据见 [R021](research/R021-foundation-closure-implementation/report.md)。

## 2. 当前结论

- **基础闭环已完成**：CLI mode/副作用、strict Config/Default round-trip、application 单候选、长期 runner、HTTP、service readiness、终止排空、degraded 诊断和架构治理已有统一 owner 与可执行证据。
- **演进策略保持**：Kernel 核心未被整体重写，也没有扩大为业务容器。
- **业务门禁继续生效**：真实用例尚未确认，Handler/Service/Repository/Model 只保留方向性约束，不确定 API、目录或实现。

## 3. 本轮目标

### 3.1 基础闭环目标

- 建立覆盖 Bootstrap/CLI/Default/Config/Composition/Capability/Resource/Lifecycle/Reload/Diagnostics/Adapter 的统一契约台账和状态语言。
- 让 help、version、默认配置生成等 Bootstrap 命令只组装所需配置契约，不构造或启动 Database、Cache、Storage、HTTP 等资源。
- 让默认配置生成、实际加载和运行期绑定共享同一配置节 owner contract，未知/重复/类型/零值/禁用语义可验证。
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
- CLI 注册/冲突、I/O、退出码、命令模式与默认配置安全生成契约。
- Config Source/priority、Default、strict Binding、Validation、Snapshot、change classification 与敏感信息契约。
- 单一候选协调、运行监督、状态/诊断、HTTP lifecycle、重载/终止语义和治理门禁的目标设计。
- 保留/补齐/局部优化/整体替换候选的比较、迁移成本、风险、验证和未决项。
- 对原 012 业务设计的状态校正及后续解锁条件。

### 4.2 不包含

- 真实业务源码、依赖、数据迁移、部署和外部系统写入。
- 无真实需求的业务实体、Handler、Service、Repository、Model、路由、命令或缓存策略。
- runtime DI、自动扫描、全局 Registry、Service Locator、动态插件、消息总线、Saga/CQRS/Event Sourcing。
- 未经证据需要的通用热重建、动态路由、listener handoff、自动观测回滚或完整服务框架。

## 5. 核心需求

### 5.1 配置与装配

- 运行模式在重资源构造前显式选择；Bootstrap、ApplicationCommand 和 Service 的完成语义、最小依赖与副作用边界可验证。
- 配置节由语义 owner 统一关联 path、safe defaults、strict typed binding、semantic validation、change classification 和 sensitivity；不建立巨型 Config。
- 未知/重复字段和不允许的弱类型转换在资源副作用前失败；missing、zero、empty、disabled 和 default 逐字段明确。
- 默认配置在写文件前通过运行期同一 binder/validator 回环校验；默认不覆盖，覆盖显式，写入失败不遗留错误目标或临时文件。
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
- 当前文档只描述真实实现；012 作为已实施变更记录，不成为第二套现行规范。

## 6. 十一项通用门禁

所有候选必须同时通过事实、目标、边界、装配、生命周期、一致性、错误、治理、演进、复杂度和业务延伸门禁。逐项当前状态及证据见 [acceptance-matrix.md](requirements/acceptance-matrix.md)。任何关键项未满足时，不得用“目录设计完成”替代底层闭环。

## 7. 成功判定

基础批次只有在以下事实全部可执行验证后才完成：Bootstrap 不启动无关资源，CLI 冲突/I/O/退出/副作用闭合，默认配置与 strict runtime binding 同源，单一候选、全进程 Supervisor、HTTP 预绑定与排空、readiness/degraded 状态、reload/终止差异、错误合并和自动治理均有成功/失败/超时证据；权威文档同步，旧入口/双轨删除，未执行项明确。

这些基础条件已通过本地验证，只解除“可以提交真实业务设计”的前置阻塞。首个真实垂直切片仍需单独确认业务 actor、用例、不变量、数据/事务、入口和验收数据。
