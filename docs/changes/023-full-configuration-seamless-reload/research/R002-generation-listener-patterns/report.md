# R002：不可变应用代际与 Listener Handoff 模式

## 1. 研究范围

本报告比较四条路径：字段原地更新、当前逐组件换代、外部进程滚动重启、完整 Application Generation，并核对 Go `net/http` 与 Caddy 的官方契约。目标是当前纯 TCP、同步 HTTP/CLI profile 内的进程不退出和请求无中断，不把 TLS、HTTP/3、WebSocket 或二进制升级提前纳入。

## 2. 官方事实

### 2.1 Go `net/http`

- [`Server.Serve`](https://pkg.go.dev/net/http#Server.Serve) 在一个 `net.Listener` 上接受连接，并为连接创建服务 goroutine；返回后 listener 已关闭。
- [`Server.Shutdown`](https://pkg.go.dev/net/http#Server.Shutdown) 先关闭 listener 和 idle connections，再等待活跃连接进入 idle；它不会等待或关闭 hijacked connection。
- `ReadHeaderTimeout`、`ReadTimeout`、`WriteTimeout`、`IdleTimeout` 与 `MaxHeaderBytes` 是 `http.Server`/connection 读取路径使用的字段，不是可并发变更的运行时配置中心。

因此，标准库提供单个 Server 的优雅停止，却没有“候选 Server 与当前 Server 共享一个地址并原子交接”的上层所有权模型。

### 2.2 Caddy

[Caddy 架构文档](https://caddyserver.com/docs/architecture) 将每份配置视为不可变原子单元：先 provision/validate 新模块，成功后才清理旧配置，并允许两代短暂并存。其 listener 源码进一步按平台复用或虚拟关闭 socket，使新旧配置能在相同地址交接。

可复用的是两条原则：

1. 全配置作为不可变代际，不在请求热路径同步每个字段；
2. 物理 listener 生命周期高于单个 `http.Server` generation。

不直接引入 Caddy：它是完整 Server 平台，依赖、配置模型、模块系统和维护面远超当前项目所需的纯 TCP listener handoff；把它作为底层库会反向改变现有 `pkg/httpx`、composition 和 copy-owned scaffold 边界。

## 3. 方案比较

| 方案 | 优点 | 无法满足的条件 | 结论 |
| --- | --- | --- | --- |
| 每个字段使用 mutex/atomic 原地更新 | 改动小 | `http.Server` transport、Todo 对象图、Cache L1、跨资源原子性无法统一；热路径同步复杂 | 拒绝 |
| 扩展当前逐组件 `KernelInstanceSwap` | 可复用现有 Lease | application binding 和 HTTP 在事务外；一次请求可能混合多个 component generation | 拒绝作为终态 |
| 外部 Supervisor/双进程滚动重启 | 运维成熟，适合二进制升级 | PID 会变化，不是当前用户要求的进程内配置重载；本地脚手架还没有交付控制面 | 仅作为部署兜底，不是本计划方案 |
| 完整 Application Generation + listener handoff | 单一 Snapshot、单一提交点、在途请求固定旧代、所有 section 可生效 | 需要新的 generation owner、listener owner、资源引用与跨层验收 | 推荐 |

## 4. 推荐 ListenerHub

目标采用项目自有、TCP-only、进程级 `ListenerHub`，不依赖 `SO_REUSEPORT` 作为唯一保证：

- Hub 按 canonical network/address 拥有唯一物理 `net.Listener` 和唯一 Accept goroutine。
- 每个 HTTP generation 获得一个虚拟 `net.Listener` route；候选 `http.Server.Serve` 可以先启动并等待 route 激活。
- Hub 在一个锁保护的提交点把新接受连接的 route 从旧代切到新代。
- 已接受并已交给旧代的连接继续由旧 `http.Server` 跟踪；旧代 `Shutdown` 停止 keep-alive admission 并等待 active requests。
- route `Close` 只关闭该 generation 的虚拟 admission，不关闭仍被新代使用的物理 listener。
- 地址变化时先 bind 新物理 listener；成功后提交新 route，再停止旧地址接受新连接。已接受旧连接仍排空。
- Hub 的 Accept、route handoff、pending dispatch、Stop 和错误结果都有 owner、停止信号与等待方式；不得为每个连接启动无界转发 goroutine。

该方案只需要标准 `net.Listener`，避免 Unix/Windows 分叉的 socket option 成为正确性基础。若实现原型证明标准接口无法满足 pending dispatch barrier，必须返回研究阶段比较 Caddy 式平台 Adapter；不得退化成“先关旧 listener 再 bind”或静默允许连接失败。

## 5. Application Generation

每个 generation 从同一个稳定 Snapshot 显式构造：

```text
configured Logger + Database + Cache + I18n + Storage
  -> Todo Repository / Service / Handler / Router
  -> generation-specific http.Server
  -> prepared virtual listener route
```

请求或连接进入哪一代后，该代完整对象图保持不可变，直到请求完成。旧 generation 不再接受新连接后，依次排空 HTTP、停止模块运行 owner、释放 typed Cache Client，再反向释放底层资源。

相同 section digest 的资源允许通过 typed resource slot 引用复用；共享只说明配置和实例完全相同，不允许按类型或字符串从万能 Registry 查找。变化 section 必须构造新 slot 并通过 Ready。

## 6. 提交与回滚边界

- Commit 前允许失败：读取、解析、全 owner 校验、资源 Build、Ready、模块构造、只读 schema readiness、listener bind、Server Serve-ready。任一失败反向清理候选，旧代不变。
- Commit 区只做不会失败的内存/route/target 切换；不得在锁内 I/O、迁移或探测。
- Commit 后旧代 cleanup 失败时，新代已经生效；保留 cleanup debt、撤销 readiness 并阻止后续 reload，不谎称候选未应用。
- 本计划不承诺任意外部数据目标切换后的业务数据回滚。Database/Storage 改址若需要数据连续性，必须由独立迁移或复制协议先行完成。
- 当前同步 HTTP profile 没有 hijacked connection；未来出现 WebSocket/upgrade 时必须新增连接 owner 与 drain policy，不能沿用普通 `Shutdown` 声称完成。

## 7. 对 023 的决策影响

1. 采用不可变全应用代际，单轨替换 section 级 `RestartRequired` 和长期服务中的逐组件 reload。
2. 保留显式 Go composition，不引入 Caddy、运行时 DI 容器或服务定位器。
3. 新增 TCP ListenerHub 是必要自研边界；其维护责任由 `pkg/httpx` owner 承担，并以 Windows/Linux 运行验收作为启用门禁。
4. schema migration 从 reload candidate 的可回滚准备中移出；candidate 只做只读 readiness。变更 Database 到未迁移目标时拒绝，旧代继续服务。
5. 当前配置文件全部 section 可在不退出进程的情况下生效，但地址变更不能保证仍使用已移除旧地址的外部客户端自动迁移。
