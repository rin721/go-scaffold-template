# 底层闭环设计

## 1. 设计边界

本设计只解决当前已确认的进程级缺口：配置候选一致性、长期运行监督、HTTP lifecycle、状态诊断、reload/终止语义与可执行治理。它不定义业务实体、Service/Repository API、动态模块或通用编排平台。

## 2. 唯一候选与责任分离

application coordinator 是 Loader 的唯一调用者。一次 Start/Reload 生成一个不可变 candidate，包含稳定 digest/identity；各 config owner 只负责自己的 decode、default、validate、sensitive redaction 和 change classification。

目标阶段：

```text
load candidate once
  -> decode/validate all owners
  -> classify changes and preflight RestartRequired
  -> prepare Kernel resources
  -> commit one application decision
```

coordinator 不拥有各节 schema，也不变成巨型 Config。Kernel 仍是底层资源事务 owner，但必须接受同一显式 candidate 或其不可变 view，不能私自再次读取。任何 application-owned immutable field 变化都在 Kernel 副作用前拒绝；即使 Kernel component section 未变化，也不能悄悄只更新 Kernel digest。

## 3. 运行单元语义

目标把三个概念分开：

- **Startup owner**：同步准备资源并确认可运行；失败立即返回并反向清理。
- **Runner**：启动后阻塞运行，任何非预期完成向 Supervisor 报告。
- **One-shot operation**：显式声明完成即成功，只用于 Bootstrap/Application CLI 等模式。

所有 owner/runner 名称非空且唯一，注册顺序确定。Service mode 中关键 runner 返回 nil 也视为运行终止事件；Supervisor 记录 primary cause，撤销 ready、取消所有 runner，并在统一 shutdown deadline 内按依赖反序 StopAndWait。错误取消不能替代 Stop，Stop 也不能在无限 Wait 之后才发生。

Go 无法安全强杀不合作 goroutine。超时后必须报告具体 owner、保持 not-ready 并以失败退出；文档和测试不得声称资源已清理。

## 4. HTTP lifecycle

HTTP 是一个完整受监督单元，而不是业务模块自行启动的 goroutine：

1. Start 阶段创建并预绑定 listener，地址/端口错误同步返回。
2. Serve 使用该 listener 阻塞运行，非 `ErrServerClosed` 的退出作为 runtime failure 上报。
3. ready 只有在 listener 成功且 runner 已进入监督后才能发布。
4. drain 先撤销 ready/停止接受新请求；Stop 调用有界 Shutdown，必要时按已确认策略 Close，并等待 Serve 退出。
5. Shutdown error、Close error 和 Wait timeout 与 primary cause 合并；不会只写日志后返回成功。

当前没有 WebSocket/hijacked connection 需求，目标不预建连接 Registry；一旦真实入口需要，必须补 owner、等待和强制结束策略后重新确认。

## 5. 状态、readiness 与诊断

最小进程状态：

```text
starting -> ready/running -> draining -> stopped
     |           |              |
     +----------> failed <------+
                     ^
ready/running -> degraded -------+
```

`Kernel RuntimeComponent.Ready` 只是候选发布门禁，不能直接成为进程 readiness。进程 ready 需同时满足：必需 Kernel generation 已发布、必需 startup owners 成功、关键 runners 仍受监督、HTTP 已绑定且未 drain、没有按策略阻断服务的 degraded。

诊断最少暴露：当前 state、ready 原因、Kernel generation、candidate digest、last reload phase/result、last committed cleanup failure 和未退出 owner；只保留安全标识，不返回配置值、DSN、Token 或错误中的敏感细节。`pkg/health` 可作为 checker 聚合原语，但 lifecycle state 是单一事实源，Health 不拥有启动/停止。

## 6. Reload 与终止排空

### Reload

- 全 owner 预检成功后才 prepare。
- 候选失败或 reload drain 超时：恢复旧代 serving，snapshot/state 不提交。
- 新代提交后旧代 cleanup 失败：新代保持 active；标记 degraded/restart-required，记录 component/generation/cause，在重启或明确处置前拒绝后续 reload。

不实现自动观测回滚：新代可能已产生对外副作用，且当前没有健康窗口、流量切换或补偿需求证明其正确性。

### Process termination

- 首先撤销 ready 并永久阻止新工作。
- cancel runners，按反序 StopAndWait。
- drain/Stop 超时不 Resume 成 running；记录不完整清理并失败退出，由 OS 作为最终资源边界。
- 尽可能继续调用后续 owner 的有界清理并合并错误，但不在仍有活跃借用时强行关闭共享资源。

这要求把当前可回滚 `Kernel.Stop` 语义与进程终止策略明确分开；具体 API 名在实施设计中确定。

## 7. 可执行治理

- package graph 门禁：验证 Kernel/composition/未来 application-adapter 方向，规则集中且有 owner。
- registration 门禁：非空唯一 ID、确定顺序、重复 path/command/runner 在监听前失败。
- lifecycle 门禁：事件 channel 证明 Start/Run/Fail/Drain/Stop/Wait 顺序和错误合并，禁止 sleep 碰运气。
- snapshot 门禁：spy 证明一次加载；未知/application section 变化不被 Kernel 单独吞掉。
- diagnostics 门禁：状态转换、readiness、degraded、redaction 和并发读取可测试。

## 8. 复杂度控制

当前只增加已发生问题所需的最小语义。不要创建通用 Module SDK、DAG、动态注册、每组件状态机、自动 cleanup 重试、listener handoff 或插件协议。若实施发现只需现有类型的小幅扩展即可满足验收，优先局部调整而非新包；任何公共 API、依赖或边界实质变化都需回到待确认。
