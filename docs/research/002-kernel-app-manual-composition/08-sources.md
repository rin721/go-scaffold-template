# 来源与复核入口

## 1. 本地代码证据

本报告研究事实以提交 `a1a3a65b8f180ca9b571b8e3d7424c74403746e0` 为历史基线。006 实施后的当前复核入口如下：

| 主题 | 本地入口 | 支持的事实 |
| --- | --- | --- |
| 应用启动链 | `cmd/app/main.go` | baseline Logger、Loader、Kernel、Compose、Host 的真实调用顺序 |
| App 定义与 Plan | `internal/kernel/app/*.go` | Definition、Binding/Input、Direct/Leased、Freeze 和运行节点 |
| 稳定租约 | `internal/kernel/app/lease.go` | Use、activeUses、draining、replace 和 resume |
| Reload 事务 | `internal/kernel/kernel.go` | Stage、候选准备、反向 drain、commit、rollback、RestartRequired 和 cleanup |
| 配置监听 | `internal/kernel/watch.go`、`internal/kernel/config/watch.go` | fsnotify、防抖、单次 Reload 错误回调 |
| Host | `internal/kernel/host.go` | Kernel 与业务 Participant 的监督顺序 |
| Supervisor | `pkg/supervisor/supervisor.go` | 顺序 Start、Task、失败取消和反向 Stop |
| Logger 组件 | `internal/kernel/app/logger/logger.go` | 私有 Resource、稳定 facade 和 typed target replacement |
| Database 组件 | `internal/kernel/app/database/database.go` | typed Config、New、Ready/Ping、私有 Close 和 Access |
| Cache 组件 | `internal/kernel/app/cache/cache.go` | disabled/Redis、稳定 Access、Ready/Ping、RestartRequired 和连接池所有权 |
| I18n 组件 | `internal/kernel/app/i18n/i18n.go` | typed Config、稳定 Translator facade 和 KernelInstanceSwap |
| Storage 组件 | `internal/kernel/app/storage/storage.go` | 对象存储 route、借用 Client、Ready、私有 Close 和 KernelInstanceSwap |
| 显式组合 | `internal/kernel/composition/*.go` | 固定清单、每能力登记、Defaults/CLI 聚合 |
| Clock | `pkg/clock/clock.go`、`internal/kernel/app/clock/clock.go` | 项目契约与 Fixed Direct Definition |
| ID Generator | `pkg/idgen/idgen.go`、`internal/kernel/app/idgen/idgen.go` | 项目契约与 Fixed Direct Definition |
| Validator | `pkg/validation/validation.go`、`internal/kernel/app/validation/validation.go` | 项目契约与 Fixed Direct Definition |
| 当前简单能力使用 | `pkg/httpx/production_middleware.go`、`pkg/storage/watch.go` | UUID nil 回退及直接 `time.Now` 等尚未统一注入的现状 |
| 健康能力库 | `pkg/health/*.go` | Registry、Kind、Snapshot；当前未接入观察期 |
| HTTP 排他资源示例 | `pkg/httpx/server.go` | `ListenAndServe` 与 Shutdown 的当前封装 |

006 已实现的能力可由上述当前源码确认；Native Reload、Handoff、观察期和自动回切仍不能从这些文件推导为已实现。

## 2. 本仓库既有研究

- [Go 脚手架底层能力装配架构对比](../001-go-capability-composition/README.md)：对照 go-clean-template、go-zero、Kratos/Wire 和 Fx，解释当前 Kernel 与普通启动期对象图的差异。
- [当前架构事实](../001-go-capability-composition/02-current-architecture.md)：记录 `Definition`、Access 租约、初始启动和 Reload 事务。
- [演进建议](../001-go-capability-composition/05-recommendations.md)：记录 001 报告当时的分轨建议；其中涉及未来业务对象装配的内容不作为本报告当前阶段结论。

本报告不复制 001 的样本全文，而是回答更具体的问题：新增一个底层能力如何开始、为什么当前入口复杂、怎样按组件特性选择安全重载策略。

## 3. 外部官方资料

### Uber Fx

- [Fx 首页](https://uber-go.github.io/fx/)：说明 Fx 的目标是依赖图自动构造、模块复用和生命周期管理。
- [Fx Lifecycle](https://uber-go.github.io/fx/lifecycle.html)：说明初始化、OnStart、等待和反向 OnStop。
- [Fx Modules](https://uber-go.github.io/fx/modules.html)：说明薄 Module、Provide/Invoke、所有权边界，以及组件应能脱离 Fx 直接构造。
- [Fx Container](https://uber-go.github.io/fx/container.html)：说明构造函数和值怎样进入运行时依赖容器。

这些官方默认机制不描述本报告目标中的业务使用租约、候选双实例、切换后观察期和健康失败自动回切。因此 Fx 的入口更短，不能证明强换代状态机也可以被等价删除。

### Kratos / Wire

- [Kratos Layout](https://github.com/go-kratos/kratos-layout)：官方模板通过 ProviderSet 和生成代码连接 data、biz、service、server 与 App。
- [Google Wire](https://github.com/google/wire)：编译期依赖注入工具；仓库已归档，适合用于理解静态生成范式，不应在未评估维护风险时作为新项目默认依赖。

### go-zero

- [go-zero](https://github.com/zeromicro/go-zero)：官方项目和模板通过 Config、ServiceContext、server 构造及 Stop 形成直接启动路径。

### 手工装配样本

- [go-clean-template](https://github.com/evrone/go-clean-template)：以普通 Go 构造函数和 composition root 显式连接 repository、use case 和 server。

外部资料只用于比较装配职责和默认生命周期，不用于宣称这些项目绝对不可能通过扩展实现热重载。

## 4. 事实与推断标记

- **当前已实现**：可以由上述本地源码直接确认。
- **报告判断**：基于当前代码和官方资料推导出的复杂度、适用性或风险判断。
- **目标设计**：本报告为满足已确认意图提出的目录、契约、策略和状态机。
- **尚未实现**：当前代码不存在，必须通过后续变更任务实现和验证。

涉及配置原子性、第三方库热更新或资源交接时，后续组件设计必须引用具体上游版本的官方保证和契约测试。不能把本报告的一般分类当成某个第三方库已经满足准入条件的证据。
