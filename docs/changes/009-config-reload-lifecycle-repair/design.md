# 开发设计：配置重载与生命周期修复

## 1. 设计结论

保留现有 Kernel 配置事务，只修复默认应用未启用 Watch 的 composition-root 缺口，并给 watcher 建立“注册完成后 reconciliation”保证。Database、Logger 和 Lease 状态机不另建旁路。

```text
service process
  -> Kernel.Start: Load v1 -> Build/Ready -> Publish
  -> Host starts application Participant
  -> WatchFiles registers parent directories
  -> reconciliation: Kernel.Reload(current sources)
  -> event/debounce -> Kernel.Reload(candidate)
       -> Stage all affected components
       -> reject RestartRequired before side effects
       -> Build/Start/Ready all candidates
       -> drain old leases in reverse order
       -> Commit all candidates
       -> Resume stable entries
       -> stop old generations in reverse order
  -> Ctrl+C: cancel tasks -> stop application -> stop Kernel
```

初始 reconciliation 即使配置没有变化也只更新/确认 Snapshot，不构造新实例；如果文件恰好在启动与 watcher 注册之间改变，它会应用最新候选。

## 2. 入口修复

`cmd/app` 的长期服务路径显式启用 Watch：

```go
host, err := kernel.NewHost(runtime, kernel.HostOptions{
	Watch: &kernel.WatchOptions{
		OnReloadError: reportReloadError(capabilities.Logger),
	},
}, applicationLifecycle{logging: capabilities.Logger})
```

实际 helper 命名可按现有代码风格调整，但必须保持：

- 回调依赖 `pkg/logger.Logger` 稳定 facade，不取得 Manager 替换权或 Resource 关闭权；
- 普通候选失败记录“未应用，旧代保留”；
- `CommittedCleanupError` 记录“已应用，旧代清理失败”；
- `app.ErrRestartRequired` 记录需要重启且本轮未应用；
- 错误链继续由 Watch/Host 保留，日志字段不包含 Snapshot 或 DSN；
- CLI 分支在创建 Host 之前返回，行为不变。

不把 `Watch` 默认隐式塞入 `NewHost`：Host 仍是可复用底层协调器，是否监听由 composition root 明确选择；本缺陷通过入口测试锁定默认应用的选择。

## 3. Watch 注册握手与 reconciliation

### 3.1 当前窗口

当前 `Host.Run` 先启动 Kernel 和上层 Participant，再启动 `runtime.Watch` task。`runtime.Watch` 又异步调用 `config.WatchFiles`；目录注册完成前没有 ready 信号。若文件在初始 `Loader.Load` 后、`watcher.Add` 前改变，可能没有 fsnotify 事件，Kernel 会长期保留旧 Snapshot。

### 3.2 目标契约

为 `config.WatchFiles` 增加 Kernel 私有的注册完成通知，或提供语义等价的内部 watcher 对象。目录全部注册成功后按以下顺序执行：

1. 标记 watcher ready；
2. 触发一次 reconciliation 通知；
3. 进入 fsnotify 事件循环；
4. 后续事件继续防抖并通知。

`Kernel.Watch` 仍用容量为 1 的 changes channel 串行调用 `Kernel.Reload`；`operationMu` 继续禁止 Start/Reload/Stop 并发修改状态。不得让 fsnotify goroutine直接调用组件。

如果目录注册失败，Watch task 返回错误，Supervisor 取消其他长期任务并反向停止 Participant。单次 reconciliation/候选错误则调用 `OnReloadError` 并继续等待下一次变化。

### 3.3 文件编辑模式

继续监听父目录而非文件句柄，以覆盖：

- 原地 Write；
- 临时文件 rename 到目标；
- Remove 后 Create；
- 一次保存产生的多事件防抖。

测试只等待显式 ready/可观察状态，不依赖固定 `time.Sleep` 猜 watcher 是否注册完成；若需要引入测试握手，只放在 Kernel/config 内部契约，不成为业务 API。

## 4. Reload 事务与错误分层

### 4.1 成功路径

```text
Load merged candidate (file then env)
  -> Stage Logger / Database by effective section digest
  -> prepare every changed candidate
  -> drain Database leases and replacement barriers in reverse Plan order
  -> Commit in Plan order
  -> publish candidate Snapshot
  -> Resume entries
  -> close previous generations in reverse order
```

成功日志只包含安全 component IDs。若文件字段被环境变量覆盖、effective section digest 不变，则 `Applied=false` 且不构造实例，这是正确语义。

### 4.2 失败路径

| 失败阶段 | 有效状态 | 处理 |
| --- | --- | --- |
| Load/Decode/Validate | 旧 Snapshot、旧实例 | 不 Build；上报并继续监听 |
| Build/Start/Ready | 旧实例继续服务 | 反向关闭已准备候选 |
| drain timeout/error | 旧实例恢复服务 | Rollback 已 drain 入口，丢弃全部候选 |
| RestartRequired 预检 | 整轮旧状态 | 任何候选 Build 前拒绝 |
| Commit 后旧代 Close | 新 Snapshot、新实例 | 返回 `CommittedCleanupError`，禁止伪装回滚 |
| watcher 基础设施错误 | Host 退出 | 取消 tasks，反向停止上层与 Kernel |

回调仅负责一次边界日志，Kernel/config/Database 各层不重复打印同一错误。

## 5. Database 真实换代验证

在 `internal/kernel` 或 composition 集成测试中使用完整 Plan 和两个临时 SQLite 文件：

1. 写入 v1 配置并启动 Host，借助 `Capabilities.Database` 建表并写入 `generation=v1`；
2. 预先在 v2 文件建立 `generation=v2`；
3. 等 watcher ready 后原子替换配置中的 DSN；
4. 通过同一个稳定 Database Access 读取到 v2 标记，证明调用方未重新注入但底层实例已切换；
5. 直接检查 v1 文件仍只有 v1 数据，后续 Access 写入只进入 v2；
6. 使用所有权可观察测试钩子或底层连接行为证明 v1 Resource 已关闭一次；钩子必须留在测试边界，不把 Close/实例指针暴露给业务 Access；
7. 取消 Host，证明 v2 关闭且 watcher 退出。

失败恢复场景把候选改为非法 Driver/不可用 DSN，确认 Access 仍读 v1；再写有效 v2，确认 watcher 没有因单次错误退出。

若当前 Database API 不足以无泄漏地观察关闭次数，优先在 `internal/kernel/app/database` 增加包内构造 seam，仅供测试注入 Resource factory；不得为测试向公开 `Access` 添加 `Close`、`Stats`、底层 GORM 或实例身份。

## 6. Logger 与跨组件事务验证

- 使用稳定 facade 在启动前、初始发布后、成功 reload 后、失败 reload 后和 Stop 后写入，核对目标 sink 与关闭次数。
- 同轮修改 Logger 与 Database，并让 Database 候选失败；Logger candidate 必须被清理，当前 facade 仍指向旧 Logger。
- 同轮全部成功时，两个组件都在候选准备完成后提交；`kernel reload completed` 由新 Logger 输出。
- Stop 顺序继续是 application Participant -> Database -> configured Logger restore/close -> Kernel baseline 日志；具体反序以冻结 Plan 为准，测试断言所有权而非偶然时间。

## 7. 其他能力生命周期矩阵

| 场景 | Clock / ID / Validator | CLI / DefaultManager | application Participant |
| --- | --- | --- | --- |
| CLI 命令 | 可在 composition 构造，但 Kernel 不 Start | 执行一次并返回 | 不构造 Host |
| 服务启动 | direct output 身份固定 | 不参与运行 task | Kernel 后启动一次 |
| 配置 reload | 不 Stage、不重建、不进入 changed | 不重建 | 不重复 Start/Stop |
| Ctrl+C | 无虚构关闭动作 | 无 watcher 所有权 | watcher task 结束后、Kernel 前 Stop 一次 |
| 启动失败 | 普通值无需补偿 | 返回原始错误 | 只有成功 Start 后才进入 Stop 集合 |

这组测试用于证明“无生命周期”也是明确契约，不为普通值增加空接口或通用 Component 包装。

## 8. 文件影响

预计修改范围：

- `cmd/app/main.go`、`cmd/app/reload_test.go`：默认服务启用 Watch、错误分类日志和入口门禁测试；
- `internal/kernel/watch.go`、`host_test.go`、`kernel_test.go`：注册 reconciliation、Host 集成和事务场景；
- `internal/kernel/config/watch.go`、`watch_test.go`、`config_test.go`：ready 握手、启动窗口、rename-save、防抖、取消和有效配置优先级；
- `internal/kernel/composition/reload_integration_test.go`：完整当前能力矩阵、真实 SQLite 换代、关闭证据和跨组件原子性；
- 根 `README.md`、`internal/kernel/README.md`：默认服务已监听、来源优先级、失败与诊断语义；
- `docs/changes/009-config-reload-lifecycle-repair/*`：确认、逐轮证据和最终结果。

不计划修改 Database 配置格式、业务 API、`go.mod/go.sum` 或 008 未完成能力。实现前必须重新检查工作树；任何与 008 同文件的增量都按 hunk 归属审阅和暂存。

## 9. 验证方案

实施后的最小门禁：

```powershell
gofmt -w <009 修改的 Go 文件>
go build ./cmd/app
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

真实 smoke 使用临时目录和 SQLite v1/v2：启动应用、等待明确 ready、修改配置、观察 Database/Logger reload、写入无效候选并确认保旧、恢复有效候选，最后发送 Ctrl+C 并核对退出码和资源关闭日志。运行前显式检查并清除相关 `APP_DATABASE__*`/`APP_LOGGER__*` 覆盖，不修改用户现有 `config.yaml` 或 `.data`。

PostgreSQL/MySQL 只在现有安全测试环境和凭据可用时运行；未执行必须记录。所有测试不得输出 DSN。

## 10. 取舍

- 选择“入口显式启用 + watcher ready reconciliation”，因为根因位于 composition root，同时需要封闭真实事件丢失窗口。
- 不让 `NewHost` 自动猜测 FileSource 并默认监听，避免底层 Host 隐式创建长期任务。
- 不用轮询替代 fsnotify，不创建第二套 Reload 协调器。
- 不扩大为通用 Handoff/回切框架；Database 与 Logger 已有稳定 facade/lease，当前修复只验证并接通现有 Swap。
