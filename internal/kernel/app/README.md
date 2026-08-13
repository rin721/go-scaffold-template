# Kernel App 组件开发

`internal/kernel/app` 提供 typed Definition、Binding/Input、Builtin Role 和生命周期事务。组件作者声明构造、依赖、配置与所有权；是否 Add 独立实例或 Replace 内置主槽位由 composition 明确选择。

## 选择声明形态

| 目标 | 构造入口 | 输出 |
| --- | --- | --- |
| 固定值能力 | `app.Value` | 普通项目接口 |
| 固定但需生命周期 | `app.ManagedFixed` | 独立 Lease facade |
| 配置化独立实例 | `app.ManagedConfigured` | 独立 Lease facade |
| 启动期替换 | `app.StartupReplacement` | 不发布 Binding |
| 运行期替换 | `app.ManagedReplacement` | 不发布 Binding |

`Add` 与 `Replace` 不能互换。`Decorate` 尚未实现，不能以 marker、相同接口、反射或字符串 qualifier 推断。

## 具名配置实例

配置化 App 接收显式规格：

```go
spec := app.Spec{
    ID:         "logging.db2",
    ConfigPath: "loggers.db2",
}
definition, err := loggerapp.Instance(spec)
added, err := app.Add(plan, definition)
```

ID 在 Plan 内全局唯一。ConfigPath 必须是合法点分段；`logger` 与 `logger.output`、`databases` 与 `databases.db1` 冲突，`logger` 与 `loggers.db2` 不冲突。

## 显式替换内置 Role

只有 Assembly 返回的 typed handle 可以选择生产 catalog 中的 Role：

```go
replacement, err := loggerapp.Replacement(app.Spec{
    ID:         "logging.main",
    ConfigPath: "logger",
})
if err != nil {
    return err
}
if err := app.Replace(plan, builtins.Logging.Role, replacement); err != nil {
    return err
}
```

一个 Role 最多一个 replacer；Replace 必须早于首个 root Binding 消费者。`Fixed`、`StartupReplace`、`RuntimeTransaction` 与 replacement 模式不匹配时原子失败。

## 声明 typed 依赖

```go
loggingInput := app.InputOf(builtins.Logging.Output.Binding())
dependencies, err := app.DependencySet(func(values app.Values) (Dependencies, error) {
    logging, err := app.Resolve(values, loggingInput)
    if err != nil {
        return Dependencies{}, err
    }
    return Dependencies{Logging: logging}, nil
}, loggingInput)
```

Input 必须来自同一 Plan 的更早 Binding，且 producer 阶段不能晚于 consumer。所有 Input 都要传给 `DependencySet`；`Values` 离开 decoder 后失效，运行期没有 Resolver。

## 独立 Leased 组件

```go
source, err := app.Configured(spec.ConfigPath, decodeAndValidate, defaults{})
definition, err := app.ManagedConfigured(
    spec.ID,
    source,
    dependencies,
    build,
    app.Leased(newAccess),
    app.KernelInstanceSwap,
    app.WithReady(ready),
    app.WithStop(stop),
)
```

- Decode 只解码和校验，不打开资源。
- Builder 接收 Context、typed 配置与依赖。
- Access callback 期间持有 Lease；资源及其子对象不得逃逸 callback。
- 创建 Resource 的组件负责 Stop；消费者不能获得 Close 权。
- `WithStart`、`WithReady`、`WithStop`、`WithCLI` 只声明真实行为。

## Builtin 定义边界

生产 builtin Definition 只存在于 `internal/kernel/builtin/config`、`builtin/logger`、`builtin/cli`，由 `builtin.NewCatalog` 收敛。普通 App 不导入 builtin、Kernel、composition 或旧日志控制面。

`RuntimeBuiltin`、`StartupBuiltin` 与 `RegisterBuiltin` 是低层框架入口，由 builtin catalog 与同包测试 fixture 使用；生产 Composition 不自行创建 Role。外部 baseline 需要保留关闭权时使用显式 `BorrowedRuntimeBuiltin`，不能伪装成 Assembly-owned。

## 当前组件

- `app/clock`：System Clock，Fixed Direct。
- `app/idgen`：UUID Generator，Fixed Direct。
- `app/validation`：Default Validator，Fixed Direct。
- `app/logger`：`Replacement(spec)` 替换主槽位；`Instance(spec)` 创建独立 Logger Binding。
- `app/database`：`Definition(spec, loggerInput)` 创建具名数据库实例，Ready 执行 Ping，Stop 关闭私有 Client。

当前没有 HTTP、middleware、handler、service、repository、model；App Plan 不承担业务对象容器职责。
