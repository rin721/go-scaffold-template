# R012：Go HTTP 与 errgroup 生命周期语义

## 1. 当前问题

当前 `pkg/httpx.Server.Start` 调用 `ListenAndServe` 并阻塞；当前 Supervisor 又先等待全部 Task，再调用 Participant.Stop。需要用项目实际依赖版本确认这两种语义能否直接组合。

## 2. 外部事实

- Go 1.25.7 `net/http` 文档和源码规定 `ListenAndServe`/`Serve` 在 Server 关闭后仍返回非 nil error，正常关闭需识别 `ErrServerClosed`。`Serve(listener)` 接收已绑定 listener，适合把 bind error 放在启动阶段同步暴露。
- `Shutdown(ctx)` 关闭 listener、空闲连接并等待活跃连接；调用方必须等待 Shutdown 返回后进程才可退出。Hijacked connection 不由它自动等待，若未来支持 WebSocket 等协议必须另定 owner。
- `x/sync/errgroup` v0.21.0 只在某个函数返回第一个非 nil error或 `Wait` 返回时取消派生 context。某个函数返回 nil 不会立即取消兄弟；`Wait` 本身等待所有函数结束，没有超时或强制终止不合作 goroutine 的能力。

来源：[Go net/http 源码](https://github.com/golang/go/blob/go1.25.7/src/net/http/server.go)、[errgroup 源码](https://github.com/golang/sync/blob/v0.21.0/errgroup/errgroup.go)。

## 3. 方案比较

| 方案 | 收益 | 代价/风险 | 判定 |
|---|---|---|---|
| 把 `ListenAndServe` 直接作为 Participant.Start | 实现最少 | Start 永不返回；Host 无法进入 ready | 拒绝 |
| 在 Start 中 goroutine 调 `ListenAndServe` | Start 可返回 | bind/serve error 存在竞态，Participant 无运行期错误通道 | 拒绝 |
| 把 Serve 作为 Task，Shutdown 留在 Stop | 运行错误可返回 Task | 当前 Supervisor 先 Wait 后 Stop，确定性互锁 | 拒绝 |
| 预绑定 listener + 受监督的阻塞 Serve + 协调 Stop/Wait | bind 同步失败，运行错误可回传，Shutdown 可排空 | 需局部调整 httpx 与 Supervisor 契约 | 推荐 |

## 4. 对当前架构的建议

保留项目自有 `httpx` 边界，不直接暴露 `net/http.Server` 给业务模块。进程 owner 应把 HTTP 表达为“启动时取得 listener、运行期阻塞 Serve、停止时 Shutdown/Close 并等待 Serve 退出”的完整单元。Service mode 下关键 runner 无论返回 nil 还是 error 都是终止事件；one-shot CLI 则用独立模式声明 nil 完成是成功。

ShutdownTimeout 必须约束“发出停止、调用 owner Stop、等待 runner”整段，而不能只包住所有 Task 已经退出后的 Stop。Go 无法安全强杀 goroutine，因此每个 runner 仍必须遵守 owner 契约；超时后的明确结果是 not-ready、记录未退出 owner 并失败退出进程，而不是声称清理成功。

证据强度：高，来自与 `go.mod` 一致的标准库和依赖源码。验证应包含端口占用、Serve 异常、正常 Shutdown、超时、nil 提前返回和不响应 context 的 runner。未决项是管理端口与业务端口是否分离、是否允许 hijacked connection；没有真实需求前不预建额外连接管理器。
