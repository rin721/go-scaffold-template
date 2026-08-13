# 产品需求：配置重载与生命周期修复

## 1. 背景与代码事实

### 1.1 已复现症状的代码解释

- `cmd/app` 的 Loader 顺序是 `FileSource(config.yaml) -> EnvSource(APP_)`，后者覆盖前者。
- `Kernel.Start` 会加载一次初始不可变 Snapshot，按 Plan 顺序构造并发布配置化 Logger 与 Database。
- `cmd/app` 服务模式使用 `kernel.NewHost(runtime, kernel.HostOptions{}, ...)`，没有传入 `WatchOptions`。
- `NewHost` 仅在 `HostOptions.Watch != nil` 时把 `runtime.Watch` 注册为 Supervisor 长期任务。
- 因此当前默认应用启动后修改文件不会触发 `Reload`。用户日志中的 `kernel started`、`application started`、`application stopping`、`kernel stopped` 只证明初始启动和 Ctrl+C 反向停止成功，不证明 watcher 或 reload 运行过。

### 1.2 已实现但尚未贯通的机制

- `config.WatchFiles` 监听配置文件父目录，处理 Write/Create/Rename/Remove 并防抖。
- `Kernel.Reload` 重新加载全部 Source，按组件配置段摘要判断影响；`RestartRequired` 在任何构造副作用前拒绝整轮。
- `KernelInstanceSwap` 先准备全部候选，再反向排空旧租约、提交新实例、恢复入口，最后反向关闭旧实例。
- Kernel 单元测试已覆盖候选构造时旧实例可用、候选失败保旧、排空超时回滚、整轮 RestartRequired、提交后旧代清理失败和 watcher 失败后继续监听。
- Host/Supervisor 单元测试已覆盖 Kernel 先启动、上层后启动、上层先停止、Kernel 后停止、长期任务失败联动停止和取消退出。

这些是通用机制证据，不等同于默认应用和真实 Database 已经完成端到端验收。

### 1.3 当前能力分类

| 能力 | 当前装配形态 | 配置变化语义 | 生命周期所有者 |
| --- | --- | --- | --- |
| 配置化 Logger | managed replacement | `KernelInstanceSwap`，稳定 facade 在 Commit 后切换 | Kernel 关闭配置化 Resource；入口关闭 baseline |
| Database | managed leased component | `KernelInstanceSwap`，稳定 Access 在 drain/Commit 后切换 | Kernel 关闭当前与旧代 Resource |
| Clock | fixed direct value | 无运行期配置，不参与 Reload | 普通值，无关闭动作 |
| ID Generator | fixed direct value | 无运行期配置，不参与 Reload | 普通值，无关闭动作 |
| Validator | fixed direct value | 无运行期配置，不参与 Reload | 普通值，无关闭动作 |
| CLI / DefaultManager | 启动前普通能力 | CLI 路径不启动 Host 和 watcher | `process.run` 调用栈 |
| application Participant | Host 上层 Participant | 不随配置重建 | Supervisor |

008 方案中的 Web、Plugin、Runner 尚未完成，不属于本任务的当前实现验收对象。以后这些组件进入默认 Plan 时，必须遵守其自己的 `RestartRequired`、Handoff 或 Runner 语义，不能机械套用 Database Swap。

## 2. 功能需求

### 2.1 默认入口启用监听

- 无参数长期服务模式必须显式传入非 nil `WatchOptions`，并使用稳定 Kernel Logger 上报单次 reload 失败。
- 有参数 CLI 模式仍只执行启动前命令，不启动 Kernel、不建立数据库连接、不注册 watcher。
- reload 错误回调不得阻塞、panic、泄漏 DSN/Token/完整配置，也不得把失败记录成成功。
- 成功变化继续由 Kernel 记录 changed component IDs；无有效变化不得伪装成实例替换。
- `CommittedCleanupError` 必须保留“新代已提交、旧代清理失败”的语义；普通候选失败则明确表示旧代仍有效。

### 2.2 监听启动不能丢更新

- 修复必须消除 `Kernel.Start` 初始 Load 完成后、fsnotify 完成目录注册前的变化窗口。
- watcher 完成目录注册后必须执行一次受控 reconciliation，或使用等价握手确认当前文件快照与 Kernel 有效快照一致。
- reconciliation 复用 `Kernel.Reload` 的串行事务和摘要判断，不建立第二套配置应用路径。
- 父目录监听继续支持常见编辑器的临时文件替换与 rename-save；防抖只合并通知，不得跳过最终有效状态。
- watcher 基础设施错误按当前 fail-fast 语义结束长期任务并触发 Host 反向停止；单次候选错误上报后继续监听。

### 2.3 Database 安全换代

- `database` 有效配置段变化时必须构造新的 Resource 并完成 Ping/Ready，成功后才排空和切换稳定 Access。
- 候选配置非法、DSN 不可用或 Ready 失败时，旧 Database 继续接受新租约，候选资源被关闭，Snapshot 不提交。
- 切换时旧租约内事务/调用先完成；新租约在 drain 期间等待并在 Commit/Resume 后取得新实例。
- 成功切换后旧连接池只关闭一次；新实例立即可用，最终 Ctrl+C 只关闭当前实例。
- 使用两个独立 SQLite 文件建立可观察标记，证明 Access 从旧库切换到新库；不能仅依赖日志、指针或 mock。

### 2.4 Logger 与整轮原子性

- Logger 配置变化必须在候选 Resource 成功后才切换同一稳定 facade；失败时当前 Logger 保持可用。
- Logger 与 Database 同轮变化时，任一候选失败都不得发布另一候选；全部准备成功后才进入排空和提交。
- Stop 时 application Participant 先停止；配置化 Logger replacement 恢复 baseline 并关闭；Kernel 最终成功日志写入 baseline，随后入口关闭 baseline。
- reload 错误上报必须使用稳定 facade，因此 Logger 自身换代前后都能记录诊断。

### 2.5 配置优先级与可诊断性

- 保持 `file -> env` 优先级，不把 watcher 误解为“文件必然覆盖环境变量”。
- 验收必须清除 `APP_DATABASE__*`、`APP_LOGGER__*` 等相关覆盖变量，或明确验证被覆盖字段的文件变化不会改变有效配置。
- 文档说明环境变量属于进程启动环境，外部修改另一个 shell 的环境不会改变已运行进程；文件事件只会促使进程重新读取它自身可见的 Source。
- 日志和测试失败信息只报告组件 ID、阶段和安全错误上下文，不输出完整 Snapshot 或 DSN。

### 2.6 其他当前能力生命周期

- Clock、ID Generator、Validator 在 reload 前后保持同一 direct output，不进入 changed 列表，也没有虚构 Stop。
- application Participant 只启动一次、停止一次，不因底层资源换代重复启动。
- Ctrl+C/取消先结束 watcher 长期任务，再按 Supervisor 规则停止上层 Participant 与 Kernel；不得遗留自有 goroutine。
- 启动失败继续执行反向补偿；停止阶段多个错误保留完整错误链。
- 排空超时不得关闭仍在使用的共享资源；必须保留可诊断失败，不用强制 Close 制造 use-after-close。

## 3. 验收标准

- 入口测试能证明服务模式注册 watcher，CLI 模式不注册；不再允许 `HostOptions{}` 静默关闭默认服务的配置监听。
- watcher 注册后 reconciliation 覆盖启动窗口；连续写入和 rename-save 最终只应用稳定有效状态。
- 使用完整 `composition.Compose`、真实 SQLite v1/v2 文件和 Database Access 验证：初始 v1、有效变更切到 v2、旧库不再被 Access 使用、旧 Resource 已关闭、取消后当前 Resource 已关闭。
- 无效 Database 候选保留 v1，随后有效 v2 能恢复；Logger 与 Database 同轮失败保持整轮旧状态。
- 环境覆盖场景明确表现为 effective section 未变化，不错误声称发生 Database replacement。
- 当前 direct 能力身份稳定；application Participant、watch task、Kernel 与 baseline Logger 的启动/停止次数和顺序符合所有权。
- 执行 `go test ./...`、`go test -race ./...`、`go vet ./...`、`go build ./cmd/app`、`git diff --check`；真实运行 smoke 验证启动、文件修改、重载成功/失败恢复及 Ctrl+C。
- PostgreSQL/MySQL 不作为本缺陷的必需本地验收；若没有已授权凭据或 CI 环境，只记录未执行，不用 SQLite 结果冒充三库验证。

## 4. 范围与约束

- 允许修改 `cmd/app`、Kernel/config 的最小监听握手、相关测试和当前权威文档；不改变 Database 业务 API 或配置字段。
- 不新增第三方依赖；继续使用现有 fsnotify、Supervisor、Kernel Reload 和稳定 Access。
- 不新增全局变量、第二套 watcher、轮询循环、Service Locator、运行期 Resolver 或强制关闭在途资源。
- 不改变 File/Env 优先级，不增加远程配置源，不在本任务引入配置管理 CLI。
- 009 独立于 008；实施时若必须修改 008 正在改动的同一实现文件，先核对归属并只提交能够明确归属于 009 的增量。无法安全隔离时停止并报告，不混合提交。

## 5. 非目标

- 不完成 008 的 Web、Plugin、Runner、用户域、邮件、密码或 CI 建设。
- 不实现 `NativeAtomicReload`、`ComponentHandoff`、切换观察期、自动健康回切、多进程协调或远程配置监听。
- 不让 HTTP 监听器、文件锁、单消费者等排他资源使用双实例 Swap；相关能力仍应选择专用 Handoff 或 `RestartRequired`。
- 不改数据库 Schema、迁移策略、Repository API 或远端数据库凭据。
- 不在方案阶段启动服务、改配置、暂存、提交或推送。
