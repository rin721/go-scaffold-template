# 数据库、事务、缓存与跨模块协作

## 1. Database Capability 使用边界

当前 Kernel Database `Access` 通过 `Use`/`WithinTx` 回调限制 Lease 生命周期，`pkg/database.Borrow` 保证借用 Client 不能在回调后继续使用。目标 Repository Adapter 必须保持这一不变量：

- Adapter 可以长期保存稳定 `Access` facade，但不能保存一次借用得到的 Client、Tx 或 GORM 对象。
- 每个 Repository 方法在 `Access.Use` 回调中建立持久化操作对象，并在回调返回前完成全部访问。
- 返回值必须是复制后的业务/应用类型，不能携带懒加载对象或底层句柄。
- 错误补充“哪个业务操作失败”的上下文，同时保留原始原因链。

示意流程：

```text
Service -> Repository port
        -> database Adapter
        -> Access.Use(ctx, func(Client) error {
             persistence record <-> domain/result
           })
```

## 2. Repository port

Repository 接口由使用它的 application 包定义，方法使用业务语言和项目自有类型。禁止：

- 暴露 GORM、SQL row、Redis client 或通用 `Query(any)`；
- 按数据库表自动生成没有用例语义的巨型 CRUD 接口；
- 在不同模块间共享具体 BaseRepository 实例；
- 把“未找到”、连接失败和取消统一变成空值。

`pkg/database.BaseRepository[T]` 可以作为 Adapter 内的实现工具，`T` 应是持久化 Record，而非强迫领域实体适配 ORM。

## 3. 事务边界

事务由 application 用例决定，绝不由 HTTP Middleware 或 context 隐式传递。

### 3.1 单 Repository/聚合

若一个 Repository 方法完整表达一个原子操作，事务可以封装在该 Adapter 方法内部，Service 不感知技术事务。

### 3.2 多 Repository 用例

真实用例需要同一数据库事务协调多个 Repository 时，由 application 定义模块专用 `UnitOfWork` port。其回调参数提供该用例需要的 typed repository view，Adapter 使用 `Access.WithinTx` 实现，并保证所有借用对象不逃逸。

```text
UnitOfWork.Within(ctx, func(Repositories) error { ... })
```

这只是语义形状，不是已定公共 API。禁止全局万能 UnitOfWork、`TxFromContext` 或让 Service 导入数据库类型。事务回调同时失败和回滚失败时必须保留两者。

跨数据库、跨模块分布式事务不在当前范围；出现真实需求时单独评估一致性模型、补偿和消息交付。

## 4. Cache

当前 `cacheapp.NewClient[T]` 从稳定 Access 创建模块所需的 typed Client，Client 自己拥有部分清理责任。目标用法：

- 只有真实的延迟、负载或可用性目标证明需要时才接入 Cache。
- Cache 是 Repository 的显式 Decorator 或 Service 的明确协作者，不能隐藏在全局工具中。
- key 命名、版本、TTL、容量和序列化格式由模块拥有并通过受控配置提供。
- hit、miss、stale、decode failure、backend unavailable 和 cancellation 语义均明确测试。
- Cache 不可用是否允许降级必须由当前用例设计明确、可观测且不改变正确性；不得捕获错误后静默返回成功。
- typed Client owner 在 Kernel Cache 停止前关闭，并等待清理任务退出。

影响 Client 构造或业务对象图的 Cache 配置初版均为 `RestartRequired`，不做局部热替换。

## 5. 其他 Capability

- Logger、Clock、ID、Validator、I18n、Storage 通过 Kernel 稳定 facade 或调用方最小接口注入。
- Service 只接收实际需要的方法；不得把完整 `Capabilities` 传入业务模块深处。
- Storage Adapter 与 Database Adapter 遵守相同所有权原则，外部资源句柄不跨回调/租约泄漏。
- Validator 负责输入结构规则，不代替领域不变量。
- Clock/ID 缺失时构造失败，不静默改用系统时间或随机全局函数。

## 6. 跨模块同步调用

调用方定义窄 port，并由 composition root 连接提供方：

```text
consumer application -> consumer-owned port
composition root      -> provider concrete service satisfies port
```

接口使用调用方真正需要的类型。如果需要模型转换，建立调用方拥有的 Adapter；提供方不为了所有潜在消费者暴露内部实体。必须防止 A 调 B、B 又同步调 A 的循环；发现循环时先重新审视业务所有权和流程编排，而不是引入 Registry 或无语义事件中转。

## 7. 异步、远程与插件边界

当前不建立事件总线、Dapr sidecar、远程 Service 或 go-plugin 进程。引入这些机制需要独立真实需求和 ADR，至少回答：

- 为什么同步进程内 port 不满足需求；
- 交付、幂等、排序、重试、死信和一致性语义；
- 身份、授权、版本兼容和可观测性；
- 部署、升级、故障隔离与运维责任；
- 删除或迁移计划。

“未来可能拆微服务/做插件”不是当前增加复杂度的充分理由。

## 8. 验证要求

- 借用 Client/Tx 逃逸测试在回调后可靠失败。
- Repository 合约覆盖不存在、冲突、超时、取消、连接错误与转换错误。
- UnitOfWork 覆盖提交、业务失败回滚、回滚再失败和取消。
- Cache 合约覆盖命中、未命中、污染数据、失效、后端故障和 Close。
- 架构测试拒绝业务包导入数据库/缓存第三方类型和其他模块 Adapter。
