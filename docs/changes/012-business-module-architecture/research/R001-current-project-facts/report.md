# R001：当前仓库事实

## 研究问题

在不把蓝图当实现的前提下，确认 `2daf47a` 当前装配链、生命周期、Capability、HTTP/CLI/Database/Cache 边界和业务能力缺口。

## 方法与范围

- 从根 `README.md` 进入当前文档。
- 读取 `cmd/app`、`internal/kernel`、`internal/kernel/composition`、`internal/kernel/app`、`pkg/httpx`、`pkg/cli`、`pkg/database`、`pkg/cache`、`pkg/fault` 等源码与相关测试。
- 使用包列表和 import/符号搜索确认是否存在真实业务模块、Handler、Repository、Route 或 HTTP listener 装配。
- 本报告是静态快照，不声称外部服务或产品验收已运行。

## 已验证事实

### 入口与装配

`cmd/app/main.go` 先建立 baseline Logger，再构造配置 Loader、Kernel 与 `internal/kernel/composition.Compose`。有参数时当前路径运行配置 CLI；无参数时创建 Host，并把 Kernel 生命周期放在最前。

`composition.Capabilities` 已包含 Logger、Clock、IDGenerator、Validator、Database Access、Cache Access、I18n Translator、Storage Access、默认配置管理器和 CLI App。组装顺序显式，Plan 在安装前 Freeze，不存在反射扫描或业务 Resolver。

### Kernel 与重载

Kernel 自己拥有 Loader。`Start` 内部读取配置并依次 stage/build/start/ready/publish；失败进行反向回滚。`Reload` 再次调用 Loader，执行 preflight/prepare、反向 drain、commit/resume/cleanup，失败保留旧 Generation。

Host 保证 Kernel Participant 最先启动，其他 Participant 随后，停止顺序反向。Supervisor 对已启动项回滚、受管任务和停止超时已有基础语义。

### 数据库与缓存

Database Access 用回调和 Lease 控制资源。`pkg/database.Borrow` 使借用 Client 在回调后失效；`BaseRepository[T]` 可以在 Adapter 内复用，但不要求 `T` 是领域实体。

Cache Access 提供创建 typed Client 的路径。typed Client 有自己的 Close/清理责任，底层 backend 由 Kernel 管理；模块接入后必须额外定义 Client owner 和停止顺序。

### HTTP、CLI、错误与 I18n

`pkg/httpx` 已有项目自有 Router/Handler/Middleware 抽象和 Server，但 composition 没有创建监听器。当前 Server `Start` 内部阻塞监听；RequestID 在生成器为空时存在隐藏 fallback，AccessLog 使用系统时间；默认错误处理尚未统一映射 `pkg/fault` 与 I18n。

`pkg/cli` 已封装 Cobra/Bubble Tea 并使用项目自有 Command/Flag/Context。当前有 Bootstrap 风格配置命令，没有需要 Kernel 业务资源的 Application 命令路径。

I18n Translator 使用显式配置文件路径；Logger、Clock、ID、Validator、Storage 均已有 Kernel stable facade 或项目契约。`pkg/fault` 已有基础错误分类、cause 与 retryable 信息。

## 明确缺口

- 没有真实业务 Domain、Application Service、Repository port/Adapter、HTTP Handler、Route 或业务 CLI Command。
- 没有业务对象图的唯一 composition root、模块 contribution 或冲突校验。
- 没有 HTTP Server Participant 和监听失败/serve 异常退出的 Host 语义。
- Kernel 内部读配置，尚不能保证 Kernel 与业务图来自同一启动快照。
- 没有业务 I18n reason/message ownership、Error Presenter、跨模块 port 或用例事务模式。
- 没有 tracing/metrics 的业务集成证据。

## 对 012 的影响

现有 Kernel 适合继续治理底层、可重载且有生命周期的 App 资源，不应被扩张成普通业务对象容器。业务对象图应在其上方静态构造。首个实现批次必须先解决配置快照、HTTP Participant 和 contribution/运行模式，不能直接把 Handler 塞进现有 Plan。

## 局限与刷新

结论绑定 `2daf47ad111141b27a1d8e100bb3d6e4cc1ea743`。任何入口、Kernel API、HTTP listener 或首个业务模块变更后都必须重新核对；构建通过也不能替代运行和产品验收。
