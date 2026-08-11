# 开发设计：Logger Capability 注入

## 1. 总体方案

应用入口先构造 `pkg/logger.Resource` 作为基线，再创建 `internal/kernel/logging.Manager`。同一个 manager 被显式传入 Kernel；Logger Capability 的发布钩子只在 Kernel 配置事务提交点调用 manager，从而避免候选提前可见。

```text
cmd/app
  -> baseline logger.Resource
  -> kernel/logging.Manager
  -> kernel.New(..., Options{Logging: manager})
  -> composition.Compose
       -> Logger Definition(manager) -> Logger Access
       -> Database Definition         -> Database Access
  -> application lifecycle Participant(Logger Access)
  -> Host.Run
```

基线 Resource 由应用入口拥有；每一代配置化 Resource 由 Logger Capability 拥有。manager 只切换委托目标，不承担这些 Resource 的关闭责任。

## 2. `pkg/logger` 契约与 sink 所有权

公共能力单轨调整为：

```go
type Logger interface {
	Debug(string, ...Field)
	Info(string, ...Field)
	Warn(string, ...Field)
	Error(string, ...Field)
	With(...Field) Logger
}

type Resource interface {
	Logger
	Sync() error
	Close() error
}

func New(*Config) (Resource, error)
func ValidateConfig(*Config) error
```

`ValidateConfig` 复用现有配置补全和枚举校验，但不打开 sink。`New` 使用解析后的配置构造 encoder、普通输出和内部错误输出，记录所有由本次构造打开的文件。stdout/stderr 使用非关闭包装；文件路径使用项目持有的文件对象，并继续通过 zapcore 写入。

`Close` 使用 `sync.Once` 保证幂等，固定执行 Sync、关闭普通输出文件、关闭错误输出文件，并用 `errors.Join` 返回全部错误。`With` 产生的 logger 共享同一个底层 Resource 状态，不能重复拥有或关闭 sink。现有 Noop/TestLogger 只需满足业务 `Logger`；需要模拟资源生命周期的测试使用独立 fake Resource。

## 3. Kernel logging manager

新增 `internal/kernel/logging`，提供并发安全的 `Manager`：

- 构造必须接收非 nil baseline `pkg/logger.Logger`。
- Manager 本身实现 `pkg/logger.Logger`；每次日志调用在读锁内取得并调用当前实例，替换必须等待在途调用结束。
- `With(fields...)` 返回持有 manager 和累计字段的动态 logger，而不是冻结当前底层 logger。
- `Replace(next)` 只接受经过 Capability Build 验证的非 nil Logger，无 I/O、无失败。
- `Restore()` 把当前目标恢复为 baseline，无 I/O、无失败。

`kernel.Options` 新增必填 `Logging *logging.Manager`。Kernel 保存同一实例并在以下成功边界记录结构化状态：初始启动完成、Reload 无变化、Reload 提交完成、停止完成。错误仍完整返回给 Host/入口，避免同一错误在 Kernel 和 stderr 重复打印。

## 4. 发布激活契约

`kernel.Definition` 增加可选的无失败发布钩子：

```go
type ActivationHooks[T any] interface {
	Activate(T)
	Deactivate(T)
}
```

- 普通能力不提供 Activation，现有 Build/Start/Stop 语义不变。
- 初始 Start 先完成全部候选 Build/Start；确认全部成功后发布 Handle，并按登记顺序 Activate。
- Reload 先完成全部候选 Build/Start 和旧租约排空，再进入不可失败的提交区；替换 Handle 后 Activate 新实例，随后统一恢复 Access。
- Reload 提交后的旧实例只执行 Stop，不执行 Deactivate，因为 manager 已指向新代。
- 候选丢弃和事务回滚不执行 Activate/Deactivate。
- Kernel Stop 按反向登记顺序处理当前实例；在 Logger Resource Stop 前先 Deactivate，使 manager 恢复 baseline。

Activation 不允许执行 I/O、阻塞或返回错误；所有可能失败的准备工作必须在 Build/Start 完成，确保提交点仍是不可失败区。

## 5. Logger Capability 与业务 Access

新增 `internal/kernel/capability/logger`：

- `ID = "logger"`，`ConfigPath = "logger"`。
- Config 使用 `mapstructure` tag 映射 `pkg/logger.Config` 字段；默认构造从 `pkg/logger.DefaultConfig()` 读取。
- Defaults 输出 environment、level、outputPaths 和 errorOutputPaths；encoding、addCaller、addStacktrace 只有显式配置时才覆盖 Environment 推导值，因此默认骨架不固化这些派生字段。
- Decode 转换为 package Config 并调用 `pkg/logger.ValidateConfig`。
- Build 调用 `pkg/logger.New`，形成 Capability 私有 Instance；Start 无外部就绪动作；Stop 调用 Resource.Close。
- Activation 使用 Instance 内的业务 Logger 调用 manager Replace/Restore。

Capability 自己实现一个 Access 适配器：底层租约跟踪私有 Instance，公开回调签名固定为 `func(pkglogger.Logger) error`，不让 `Resource` 或 Close 方法逃逸。Definition 不登记 Kernel；composition 负责调用 Register 并构造 Access。

Composition 先登记 Logger，再登记 Database，收集两者 Defaults 后构造 DefaultManager 和可选 CLI。任一步失败仍返回零值 Capabilities，不暴露部分组合结果。

## 6. 应用入口和错误边界

入口增加可测试的进程函数来拥有 baseline Resource，确保 `os.Exit` 之前可以执行 Close。构造 baseline 或最终 Close 失败时使用 stderr 兜底，因为此时没有可安全依赖的配置化 logger。

服务模式增加 `applicationLifecycle` Participant：

- `Start(ctx)` 通过 Logger Access 输出 `application started` 和稳定 application 字段。
- `Stop(ctx)` 通过同一 Access 输出 `application stopping`。
- Host 顺序为 Kernel、applicationLifecycle；停止顺序相反，保证停止日志发生在 Logger Deactivate/Close 前。

`execute` 继续只在最外层输出一次返回错误并调用 `cli.GetExitCode`。本任务不把返回错误改成 Kernel 内日志，也不启用 Host Watch。

## 7. 文档、兼容与验证

这是仓库内单轨替换：迁移全部 `logger.New`、`Logger.Sync` 和 `kernel.New(..., Options{})` 调用，不保留旧接口别名或默认 manager 兼容分支。同步根 README、Kernel/Capability 权威说明和 `pkg/logger` 用法；任务目录只保留实施证据。

验证覆盖资源释放、manager 并发与动态 With、Kernel 激活时序、Logger Capability、Composition 默认配置和应用 Participant。最终执行格式化、tidy 差异检查、build、unit、race、vet、实际 `config init` 和 Diff 检查；不需要外部 Database 即可完成本任务的自动化验收，真实服务启动不作为已验证结果声明。
