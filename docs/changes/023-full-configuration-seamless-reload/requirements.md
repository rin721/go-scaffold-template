# 需求：全配置无感重载

## 1. 背景

当前 Logger、Database、I18n 与 Storage 可以通过 Kernel component generation 切换；Cache、HTTP 与 Todo 使用 `RestartRequired`。022 只修复了 restart latch 的恢复能力，没有让后三者热生效。

[R001](research/R001-current-reload-boundaries/report.md) 证明 HTTP/Todo 位于 Kernel reload 事务外，Cache L1 也不随 Redis generation 变化；[R002](research/R002-generation-listener-patterns/report.md) 证明需要不可变 Application Generation 和进程级 listener handoff，而不是字段原地更新或逐 section 松绑。

## 2. 产品目标

用户编辑当前正式配置文件后，只要候选合法且所需外部资源 Ready，长期 Service 必须在同一进程中完整应用全部配置，不要求人工停止或重新执行 `go run ./cmd/app`，且普通同步 HTTP 请求不因切换失败或被中断。

## 3. 范围

### 3.1 包含

- 当前七个配置节的完整候选构建与生效。
- 文件稳定采样、strict decode、owner validation 与 File < Env 有效配置。
- 完整 Application Generation、资源 slot、HTTP connection generation 和单一提交点。
- 同 HTTP 地址 transport 变化、HTTP 地址变化、Todo Policy 变化和 Cache backend 变化。
- 候选失败、提交后清理失败、长请求排空与 watcher 连续变更。
- Windows amd64 与 Linux amd64 的真实 listener/runtime 验收。
- 当前同步 HTTP/CLI profile 的文档、诊断与单轨迁移。

### 3.2 不包含

- 二进制热升级、多进程滚动发布或外部负载均衡控制面。
- TLS certificate、HTTP/3、WebSocket、SSE、hijacked connection 或后台 consumer；当前配置没有这些能力。
- 自动数据库复制、Schema 数据迁移、对象存储搬迁、跨 bucket 一致性或业务双写。
- management API、远程配置中心、配置写 API、审批流或自动部署。
- 运行时 DI 容器、service locator、反射构造或自动扫描。

## 4. 核心需求

### SEM-001 无感语义

- reload 期间 Host 与物理 listener owner 保持运行。
- 已被旧 HTTP generation 接受的连接与请求使用旧完整对象图直到结束。
- 提交线性化点之后接受的连接只使用新完整对象图。
- 任何请求不得同时观察两个配置 generation 的依赖。
- 地址变化时新地址在 commit 前已成功 bind；旧地址不再接受新连接后，已接受连接继续排空。

### CFG-001 稳定候选读取

- watcher 事件只触发 reload 请求，不直接应用配置。
- FileSource 对 sharing violation、atomic rename 的短暂不存在和内容仍变化执行有界稳定采样。
- 只有连续稳定的文件身份与 digest 才进入解析；稳定非法内容返回原错误并保留旧代。
- 重试必须受 context、总 reload deadline、最大尝试和可识别瞬时错误约束，不得隐藏权限或永久 I/O 失败。
- Env 仍高于 File；运行中进程无法接收的外部 shell 环境变化不得被虚构为动态配置。

### TXN-001 完整候选事务

一个 candidate 必须从同一 Snapshot 显式准备：configured Logger、Database、Cache、I18n、Storage、Todo Repository/Service/Handler/Router、HTTP Server 和 listener route。全部 Build/Ready 成功前不得改变 current generation。

### GEN-001 不可变应用代际

- generation 发布后配置与对象图不可变。
- generation 拥有稳定 ID、Snapshot digest、资源引用、HTTP connection admission、active request/connection 和终结 journal。
- 相同配置资源可按 typed key 引用复用；不同配置不得共享可变实例。
- generation 不向业务代码暴露 Kernel、Registry、Resolver、Snapshot 或资源 Close 权。

### CAP-001 七节配置全部生效

- Logger：新业务日志和 process configured target 在 commit 后使用新配置；旧请求仍可完成旧 sink 写入，旧 sink 排空后关闭。
- Database：候选 pool 先 Ready；新 generation 只使用新 pool，旧请求固定旧 pool。
- Cache：Redis/disabled、连接和 tag namespace 随 generation 切换；typed Client 的 L1/tag index 不得跨 generation。
- I18n/Storage：新 generation 捕获新实例；旧请求继续使用旧实例。
- Todo：重建 immutable Policy、Service、Handler 和 Router，不使用共享可变配置。
- HTTP：所有当前 ServerConfig 字段作用于新 connection generation。

### HTTP-001 ListenerHub

- 物理 TCP listener 由进程级 Hub 独占，`http.Server` 只拥有 generation route 和已接受连接。
- 同地址候选不得第二次 `net.Listen`，也不得先物理关闭旧 listener。
- candidate Server 必须在 commit 前进入 Serve-ready 但不可接收生产连接。
- route handoff 必须处理已接受但尚未交付的 pending connection，不得因关闭旧 route 静默丢弃。
- Hub 的 Accept goroutine、route、Stop、错误传播和等待都有明确 owner 与有界关闭。

### HTTP-002 地址变化

- 新地址 bind 失败时旧地址与旧 generation 不变。
- 新地址成功并 commit 后，旧地址停止接受新连接；已接受旧连接 graceful drain。
- diagnostics 必须同时说明 configured address、实际 bound address 和 retiring address，不能把客户端迁移描述为框架保证。

### LIFE-001 排空与资源终结

- reload commit 后先关闭旧 generation 的 HTTP admission，再等待 active requests/connections。
- 模块运行 owner 先于底层 Capabilities 停止；资源按构造 journal 反向释放。
- reload cleanup 与进程 shutdown 使用同一 typed owner/generation/phase/policy/result 诊断语义。
- terminal finalizer 不因失败被盲目重试；cleanup debt 保留 owner/reference 并阻断后续 reload。

### DATA-001 外部数据边界

- Database candidate 必须执行连接 Ready 与只读 schema readiness；reload 不自动执行 schema migration。
- Storage/Database 地址变化的业务数据连续性由独立迁移计划保证。
- 文档和日志不得把资源连接成功描述为数据已经迁移或一致。

### FAIL-001 失败与恢复

- Load/Validate/Build/Ready/Serve-ready 任一失败：反向清理 candidate，current generation 和 listener route 不变。
- Commit 区只允许不会失败的内存状态、logger target 和 listener route 切换。
- Commit 后旧代清理失败：新代保持 active，状态 degraded/readiness false，后续 reload fail-closed。
- watcher 在被拒绝候选后继续受理后续稳定文件事件。

### OBS-001 诊断与日志

诊断至少包含 attempt、candidate/current/retiring generation、snapshot digest、changed sections、phase、bound routes、active connections/requests、resource reuse/build、restart policy（迁移完成后应为空）、cleanup debt 和脱敏 error type。日志必须显示失败 phase 与 owner，不能只输出 `*errors.errorString`。

### MIG-001 单轨替换

- 当前长期 Service 的 section-level `RestartRequired` 和 per-component reload 单轨删除，不保留双重 Coordinator。
- `application.http`、`module.todo` 仅校验但不构造的旧 reload binding 必须迁移。
- Cache 的跨 generation typed Client 入口必须替换为 generation-owned 构造和终结。
- 旧日志、测试、README 示例和 `RestartRequired` 行为说明同步删除或更新。

### PERF-001 性能与背压

- 请求热路径只获得一次 generation/connection ownership，不逐字段加锁。
- 未变化底层资源按 typed section digest 复用，避免只改 Todo 时重连 Database/Redis。
- reload 串行；事件按 latest-wins 合并，不允许无界 candidate 或 retired generation。
- pending listener dispatch 有明确背压，不为每个连接创建脱离 owner 的转发 goroutine。

## 5. 验收标准

1. 七个 section 分别修改都在同一 PID 内生效，不出现 `RestartRequired`。
2. 七节组合修改只产生一个新 generation；不存在半提交或混合 generation 请求。
3. 同地址 HTTP timeout/header-limit 变化期间持续请求无 connection-refused、无 reload 导致的 5xx、无 data race。
4. 地址变化时新地址先 Ready，旧地址已接受请求完成；测试不宣称旧客户端自动迁移。
5. Database DSN 切换测试证明旧在途请求使用旧 DB、新请求使用新 DB；缺 schema 的新 DB 被 commit 前拒绝。
6. Cache backend/tag namespace 切换后新 generation 看不到旧 L1；旧 generation 在排空前仍可完成。
7. Todo limits 修改后新请求使用新 Policy，旧在途请求保持旧 Policy。
8. 稳定非法 YAML、owner validation、candidate Ready、listener bind 失败均保留旧代。
9. Windows 原地写、atomic rename、sharing violation 和连续保存测试稳定通过；不会读取半文件。
10. cleanup failure、长请求超时和进程 shutdown 的 owner/diagnostics/物理释放结果真实。
11. Windows 与 Linux 真实 runtime、全量 test/race/vet/build、架构残留与文档链接门禁通过。

## 6. 非功能约束

- 中文注释、文档和诊断说明；Go 标识符符合社区惯例。
- 不新增大型 Server/runtime 框架依赖；ListenerHub 使用标准库，若原型否定可行性必须退回研究重新确认。
- secrets 不进入 digest 明文、日志、diagnostics 或测试输出。
- reload 默认有界，不无限等待、不强杀请求、不静默降级旧实现。
