# GitHub 主流样本

## go-clean-template：手工 composition root

官方 README 明确说明 `internal/app/app.go` 只有一个 `Run`，所有主要对象在这里通过 `New...` 构造函数创建。当前代码依次创建 logger、tracing、Postgres、JWT、repository、use case 和多种 server，使用 `defer` 或显式 shutdown 释放资源。

特征：

- 依赖图就是可读的普通 Go 调用链；编译器校验类型，但构造顺序和完整性由人维护。
- 业务 use case 依赖调用方定义的窄接口，符合依赖倒置。
- 资源实例被下游对象直接持有，生命周期与进程一致。
- 不需要反射或生成步骤；对象图增长后会增加 composition root 样板。
- 官方说明把 Wire 作为大量注入时的可选项，而不是默认前提。

它与当前仓库都强调显式构造和调用方接口，但当前仓库多了一层稳定 Access 和运行期代际事务。

## go-zero：生成入口 + ServiceContext

go-zero 的官方 README 把 `internal/svc/servicecontext.go` 定义为 MySQL、Redis 等依赖的传递位置。当前 API/RPC 入口模板执行以下固定流程：加载 typed config、创建 server、创建 `ServiceContext`、注册 handler/server、`defer Stop`、启动服务。

特征：

- `ServiceContext` 是服务级依赖聚合对象，logic/handler 通常持有它。
- 新依赖通常增加 Config 字段、`ServiceContext` 字段和构造逻辑。
- 入口非常直接，生成工具降低样板，排障路径短。
- 聚合对象容易随服务增长而扩大；调用方可能获得超出自身需要的依赖。
- 底层库提供大量内建韧性能力，但默认装配路径仍以启动期创建、进程停止时释放为主。

它比当前 `Capabilities` 更偏业务服务上下文。当前仓库明确禁止业务拿 Kernel/Resolver，但如果把所有未来能力都堆进 `Capabilities` 并整体下传，也会逐渐接近大 `ServiceContext` 的耦合风险。

## Kratos：ProviderSet + Wire 生成静态图

Kratos 官方 `kratos-layout` 把 `server`、`data`、`biz` 和 `service` 的 ProviderSet 交给 `wire.Build`。生成的 `wire_gen.go` 是普通 Go：按依赖顺序调用 `NewData`、repository、use case、service、gRPC/HTTP server 和 `newApp`，并汇总 cleanup。

特征：

- 构造函数参数表达依赖，Wire 在生成期解析静态图；最终二进制不需要运行时容器或反射。
- ProviderSet 便于按层组织，但真实调用顺序需要查看生成代码。
- 构造函数可返回 cleanup 和 error，生成代码负责连接失败清理路径。
- Kratos App 统一管理 server 的进程生命周期。
- 标准图在启动时生成一组固定对象；默认模板没有当前 Kernel 的租约排空和多资源原子换代语义。

需要单独考虑维护风险：Google Wire 官方仓库已于 2025-08-25 归档并声明不再维护，而当前 Kratos 模板仍在使用它。这说明 Wire 模式仍有现实存量和生态惯性，但不等于它适合作为新项目的无条件默认选型。

## Uber Fx：运行时类型图 + Lifecycle

Fx 使用 `fx.Provide` 注册构造函数、`fx.Decorate` 注册装饰器、`fx.Invoke` 触发所需对象构造。官方生命周期分为初始化和执行两阶段：初始化注册并解析对象，执行阶段按追加顺序运行 OnStart、等待信号，再按反序运行 OnStop。

特征：

- 运行时容器根据类型和参数对象解析依赖，减少大规模手工 wiring。
- Module、group、name 等机制适合大型组织复用装配模块。
- Lifecycle 统一收集资源启动和停止钩子，并施加硬超时。
- 图错误主要在 `fx.New`/启动时暴露，不是 Go 编译器原生错误；排障需要理解容器事件和类型解析。
- 官方生命周期关注一次初始化、启动、等待和停止；默认模型不提供当前 Kernel 的配置候选、租约排空和跨能力原子发布。

Fx 与当前 Kernel 都管理生命周期，但管理对象和目的不同：Fx 负责解析应用对象图，Kernel 负责少量资源的运行期代际切换。仅因为两者都有 hooks，就把 Kernel 改成 Fx Module，会丢失最关键的重载事务语义。

## 四类范式归纳

| 范式 | 图的表达 | 校验时机 | 直接持有实例 | 标准生命周期 | 标准运行期换代 |
| --- | --- | --- | --- | --- | --- |
| 手工构造 | 普通 Go 调用链 | 编译期 + 测试 | 是 | `defer`/自有 supervisor | 否 |
| `ServiceContext` | 聚合结构体 + 构造函数 | 编译期 + 启动期 | 是 | server Start/Stop | 否 |
| Wire | ProviderSet + 构造函数参数 | 生成期 + 编译期 | 是 | cleanup + App | 否 |
| Fx | 运行时类型图 | `fx.New`/启动期 | 是 | OnStart/OnStop | 否 |
| 当前 Kernel | 固定清单 + Definition | 注册期 + 启动/重载期 | 否，持有稳定 Access | Kernel + Supervisor | 是 |

表中的“否”只表示本报告检查的官方默认装配路径没有提供同等级的跨能力事务替换，不否认应用可以自行扩展其他机制。
