# ADR-002：采用不可变 Application Generation 与进程级 ListenerHub

- 状态：已确认并已实施；跨平台与真实 Redis 验收未闭环
- 日期：2026-08-15
- 依据：[R001](research/R001-current-reload-boundaries/report.md)、[R002](research/R002-generation-listener-patterns/report.md)
- 取代范围：长期 Service 的逐组件配置 reload 与 `HTTP/Todo/Cache RestartRequired` 终态

## 决策

长期 Service 把当前完整配置 Snapshot 作为不可变 Application Generation 的唯一输入。每个 generation 显式拥有或引用 configured Capabilities、应用模块对象图、Router、`http.Server` connection generation 与终结 journal。有效候选全部准备成功后，通过单一提交点把新连接 admission 切换到新 generation；旧 generation 不再接收新连接，完成在途请求后释放。

物理 TCP listener 不再由某个 `http.Server` 独占，而由进程级 `ListenerHub` 持有。每个 `http.Server` 使用自己的虚拟 listener route；同地址 reload 切 route 而不重新 bind，地址变化先 bind 新地址再提交。

## 不选择的方案

- 不原地修改共享 Config、Todo Policy 或 `http.Server` 字段。
- 不把 application binding 简单改成 `KernelInstanceSwap`。
- 不保留 `RestartRequired` 作为当前七个 section 的长期策略。
- 不用外部进程重启冒充本次进程内 reload。
- 不引入 Caddy、运行时 DI 容器、service locator 或通用可变 Registry。
- 不以 `SO_REUSEPORT` 作为跨平台正确性的唯一基础。

## 不变量

1. 一个请求只使用一个 generation。
2. candidate 在 commit 前不得获得生产连接 admission。
3. commit 区无 I/O、无 goroutine 启动、无可失败动作。
4. 旧 generation 只有在 admission 关闭且 active work 为零后才能终结底层资源。
5. 相同 digest 的资源引用复用必须 typed、显式且有引用计数；不得按运行时类型查找。
6. 外部数据迁移不属于内存 generation rollback。
7. ListenerHub 与 watcher 是 process baseline owner，不被 watched config 自己替换。

## 后果

正面结果：七节配置可以同进程生效；HTTP transport 与对象图拥有一致代际；候选失败保持旧服务；热路径不需要逐字段同步；同地址 listener 不发生 close-then-bind。

代价：需要重写长期 Service 的协调层，动态 generation owner 比当前固定 Supervisor participant 更复杂；ListenerHub 需要严格的并发、背压与跨平台运行测试；配置改变外部数据目标时仍需独立迁移纪律。

## 重新决策触发器

- 标准 `net.Listener` 无法通过原型证明 pending dispatch 无丢失；
- 引入 TLS/HTTP3、WebSocket/hijacked、后台 consumer 或多个 listener；
- 需要跨进程二进制升级；
- 需要任意状态型外部资源的自动回切；
- Windows 或 Linux 运行验收不能证明相同地址无连接中断。
