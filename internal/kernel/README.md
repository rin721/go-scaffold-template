# kernel

`internal/kernel` 是脚手架内部的基础能力运行时。它拥有 Builder、InstanceHooks 和 Access 托管契约，加载不可变配置快照，按显式注册顺序启动独立能力，并在配置变化时执行排空、候选构造、统一发布和旧实例清理。

## 运行方式

调用方负责选择配置来源，主动调用固定组合清单，把稳定 Access 显式传给业务构造函数，最后创建 Host 监督整个进程：

```go
loader := config.New(
	config.FileSource("config.yaml"),
	config.EnvSource("APP_"),
)
runtime, err := kernel.New(loader, kernel.Options{})
if err != nil {
	return err
}
capabilities, err := composition.Compose(runtime)
if err != nil {
	return err
}
service := NewService(capabilities.Database)
server := NewServer(service)
host, err := kernel.NewHost(runtime, kernel.HostOptions{
	ShutdownTimeout: 10 * time.Second,
	Watch: &kernel.WatchOptions{
		OnReloadError: reportReloadError,
	},
}, server)
if err != nil {
	return err
}
ctx, cancel := supervisor.SignalContext(context.Background())
defer cancel()
return host.Run(ctx)
```

`kernel.New` 只创建空运行时，不扫描、不反射发现，也不默认组合任何能力。`composition.Compose` 当前在源码中逐项登记 Database Definition；必须在 `Host.Run` 前显式调用，重复调用会触发重复 ID 错误。创建 Host 不会登记或查找 Capability，因此引入进程监督不会改变显式注入方式。

`NewHost` 固定把 Kernel 作为第一个 `pkg/supervisor.Participant`，随后才启动上层 Participant；停止时顺序相反，因此业务服务会在 Kernel 管理的资源之前退出。`Watch` 为 `nil` 时不监听配置；显式启用监听时必须提供错误回调，并且 Loader 必须包含文件配置源：

```go
host, err := kernel.NewHost(runtime, kernel.HostOptions{
	Watch: &kernel.WatchOptions{OnReloadError: reportReloadError},
}, server)
if err != nil {
	return err
}
return host.Run(ctx)
```

## 配置事务

- `Start` 加载初始快照，按注册顺序执行 Decode、Build、Start；全部成功后才发布 Access。
- `Reload` 先解码和校验全部变化配置，再关闭受影响 Access 的服务闸门。
- 候选 Build/Start 与旧租约排空并行；任一步失败或超时都会停止候选、恢复旧实例并保持旧配置版本。
- 全部候选成功且旧租约归零后，kernel 在闸门关闭期间替换所有实例，再统一恢复调用。
- 发布后按反向顺序 Stop 旧实例。此阶段失败返回 `CommittedCleanupError`，表示新配置已提交，不得回滚。
- 相同配置段摘要不会重建实例；文件事件默认防抖 `250ms`，单次事务默认超时 `30s`。

## 配置来源

`internal/kernel/config` 支持 Map、JSON/YAML 文件和环境变量来源。后加载来源覆盖先加载来源，环境变量使用双下划线表达嵌套路径，例如 `APP_DATABASE__PINGTIMEOUT=5s`。

```yaml
database:
  engine: sql
  driver: postgres
  # dsn 通过 APP_DATABASE__DSN 提供，不写入文件。
  pool:
    maxOpenConns: 25
    maxIdleConns: 5
    connMaxLifetime: 30m
    connMaxIdleTime: 5m
  pingTimeout: 5s
```

生产环境通过 `EnvSource` 提供 `database.dsn`。当前 loader 不执行字符串插值；快照的脱敏视图会隐藏 DSN、密码、Token、Key 和 Credential。

## 边界

- v1 能力彼此独立，不解析依赖 DAG，也不构造业务 service、handler 或 server。
- 业务代码不得持有 Kernel Handle、Resolver 或 Container；依赖必须通过构造函数接收 Capability Access。
- Capability Definition 不得自行登记 Kernel；启用清单和注册顺序只由 `internal/kernel/composition` 决定。
- `composition.go` 只维护总入口、组合顺序和结果汇总；每项能力的 Definition 选择与登记放在同名文件，例如 `database.go`。
- Host 只把 Kernel、上层 Participant 和可选 Watch Task 交给 `pkg/supervisor`；它不复制进程启动、取消和停止算法。
- `Watch` 的单次重载错误通过必填回调上报并继续监听；fsnotify 后端错误才终止 Task。
