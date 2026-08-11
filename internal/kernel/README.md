# kernel

`internal/kernel` 是脚手架内部的基础能力运行时。它拥有 Builder、Lifecycle 和 Access 托管契约，加载不可变配置快照，按显式注册顺序启动独立能力，并在配置变化时执行排空、候选构造、统一发布和旧实例清理。

## 运行方式

调用方负责选择配置来源，主动调用固定组合清单，再把稳定 Access 显式传给业务构造函数：

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
if err := runtime.Start(ctx); err != nil {
	return err
}
_ = service
```

`kernel.New` 只创建空运行时，不扫描、不反射发现，也不默认组合任何能力。`composition.Compose` 当前在源码中逐项登记 Database Definition；必须在 `Start` 前显式调用，重复调用会触发重复 ID 错误。

kernel 实现 `pkg/lifecycle.Participant`。应用应把它排在依赖其能力的 Participant 之前，并把文件监听作为长期 Task：

```go
runner := lifecycle.New(lifecycle.Config{}, runtime, server)
if err := runner.AddTask("kernel-config-watch", func(ctx context.Context) error {
	return runtime.Watch(ctx, reportReloadError)
}); err != nil {
	return err
}
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
- `Watch` 的单次重载错误通过必填回调上报并继续监听；fsnotify 后端错误才终止 Task。
