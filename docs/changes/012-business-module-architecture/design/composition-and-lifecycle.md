# 装配、配置与生命周期

## 1. 当前约束

当前 `Kernel.Start` 与 `Kernel.Reload` 内部调用配置 Loader，而业务对象图和 HTTP 监听器尚不存在。如果 composition root 为业务配置再次调用 Loader，同一次启动就可能读到两份不同快照；如果先构造业务 Adapter 再启动 Kernel，又可能在资源就绪前执行 I/O。这两点必须在接入首个业务模块前解决。

另外，当前 `pkg/httpx.Server.Start` 在内部调用阻塞式 `ListenAndServe`。监听地址占用等错误无法按目标 Host 语义在 `Start` 阶段同步完成确认，因此不能直接把它作为普通 Participant 使用。

## 2. 唯一配置协调者

目标由 application config coordinator 成为唯一 Loader 调用者：

1. 启动时加载一份不可变候选快照。
2. Kernel 使用“外部提供的快照”完成 stage/build/start，而不是再次读取。
3. application composition 从同一快照解码进程模式、HTTP 与各模块的不可变配置。
4. 解码和校验全部成功后才构造业务对象图和 contribution。

配置解码仍应由各语义所有者定义 schema 和默认值，coordinator 只编排，不变成包含所有配置细节的巨型结构。错误信息指出配置节与字段，但不得回显 Secret、完整 DSN 或 Token。

## 3. 初始启动事务

目标顺序如下：

1. 创建必需的 baseline Logger；即使配置尚未加载也能记录启动失败。
2. Loader 读取一次候选快照；失败则退出。
3. 解码并校验 Kernel 与 application/module 配置。
4. 构建并冻结 Kernel Plan；安装稳定 Capability facade。
5. 以纯构造方式建立模块 Service、Adapter 与 contribution；不得在此时 I/O。
6. 集中验证 Module、Route、Command、Participant 冲突与运行模式约束。
7. 创建 Host，固定 Participant 顺序。
8. Host 启动 Kernel；Kernel 完成资源 start/ready/publish。
9. 启动确需资源探测或后台任务的 Module Participant。
10. HTTP Participant 预绑定 listener，确认监听成功后启动 serve goroutine。
11. 所有 Participant 成功后进程进入运行态。

任一步失败都按已启动项的严格反序停止，并使用 `errors.Join` 或等价项目能力保留主要错误与清理错误。只有新 Logger 已成功构造并接管后，才能替换 baseline Logger；关闭顺序保证 baseline 最后可用。

## 4. Participant 与资源所有权

每个 Participant 必须明确：

- `Start(ctx)` 何时可认为成功；
- 自己创建哪些 goroutine、listener、client 或 timer；
- 取消信号与等待退出方式；
- `Stop(ctx)` 是否幂等；
- 运行期意外退出如何通知 Supervisor；
- 主错误与停止错误如何合并。

构造函数不承担资源探测。普通无后台资源的 Service 不需要被包装成 Participant；不要为了统一形式扩大生命周期表面。

目标启动与停止顺序：

```text
Start: Kernel -> module participants -> HTTP / application command
Stop:  HTTP / application command -> module participants -> Kernel -> baseline logger
```

模块拥有的 typed cache client 或清理器必须在 Kernel Cache facade 失效前关闭，因此由模块 Participant/Cleanup 或 composition owner 管理，不归 Handler 临时释放。

## 5. HTTP Participant 的失败语义

目标 HTTP Participant 不能直接复用当前阻塞式 `Server.Start` 语义。它应：

1. 在 `Start` 中同步创建并绑定 listener；地址非法或端口冲突立即返回。
2. 绑定成功后启动受 owner 管理的 serve goroutine，并记录 done channel。
3. 将非正常 serve 退出报告给 Supervisor，而不是只写日志。
4. `Stop` 调用带超时的 graceful shutdown，然后等待 goroutine 退出。
5. graceful shutdown 与强制关闭都失败时保留全部错误。

这要求实施阶段调整 `pkg/httpx` 或增加项目自有生命周期 Adapter；不得在业务模块中各自复制 Server 管理逻辑。

## 6. 重载边界

首版业务对象图、路由表、命令表和监听器在进程内不可变。重载流程使用同一候选快照：

1. coordinator 加载一个候选快照。
2. 先计算 application-owned 配置节的稳定摘要并比较。
3. 若任何影响业务对象图、路由、命令、HTTP listener 或模块不可变配置的字段变化，整个候选返回 `RestartRequired`，Kernel 不得提前应用候选。
4. 若 application-owned 配置未变化，把同一候选交给 Kernel 的 prepare/drain/commit/resume 事务。
5. Kernel 失败时保留旧 Generation；application 对外仍保持旧对象图。

初版不做业务对象热重建、动态路由替换或部分应用配置。这一保守边界避免“Kernel 已提交而业务配置仍旧”或相反的撕裂状态。

## 7. 运行模式

运行模式使用明确枚举而不是布尔参数：

- **Service mode**：启动 Kernel、模块 Participant 与 HTTP，等待信号。
- **Bootstrap CLI mode**：例如当前 `config init`，在 Kernel 前执行，不打开数据库或缓存。
- **Application CLI mode**：需要业务 Capability，启动 Kernel 和必要模块后执行一次命令，再按反序停止。

命令分类由静态命令元数据或显式入口决定，不能先构造整张业务图再猜测命令类型。未知命令、帮助和参数错误也不得无必要地打开资源。

## 8. 验证要求

- Loader spy 证明一次启动只读取一次快照。
- 故意改变 application-owned 配置，证明在 Kernel 事务前返回 `RestartRequired`。
- 端口占用测试证明 Host `Start` 同步失败且已启动资源反向关闭。
- 注入 Module Participant 启动失败、serve 异常退出和 shutdown 失败，验证顺序与错误合并。
- Bootstrap CLI 测试证明不创建/启动数据库、缓存和 HTTP。
- Application CLI 测试证明使用同一 Service，并在命令结束后无 goroutine 或资源泄漏。
