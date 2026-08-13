# Kernel 内置能力与 App 装配

`internal/kernel` 负责 Config、Logger、CLI 三项内置 baseline、显式 App Plan、配置事务和反向资源关闭。它不扫描包、不构造业务对象，也不提供运行期 Service Locator。

## 生产装配路径

```text
pkg/<name>
    -> internal/kernel/builtin/<name>  # Kernel baseline
    -> internal/kernel/app/<name>      # replacement 或 independent instance
    -> internal/kernel/composition
    -> Assembly / Plan / Kernel / Host
```

生产代码通过 `builtin.NewCatalog` 得到封闭清单，`Assembly.Plan` 构造 Required Bootstrap baseline 并返回 typed Role handle。普通 App 只能使用这些 handle、Binding 和 Input；不能从配置、接口实现、字符串名称、反射或 `init` 动态产生 Role。

当前 catalog：

| Role | 阶段 | 激活 | 可见性 | 策略 | baseline 所有权 |
| --- | --- | --- | --- | --- | --- |
| Config | `Bootstrap` | `RequiredActivation` | `KernelOnly` | `StartupReplace` | `AssemblyOwnedBaseline` |
| Logger | `Bootstrap` | `RequiredActivation` | `AppVisible` | `RuntimeTransaction` | `AssemblyOwnedBaseline` |
| CLI | `PreStart` | `SelectedActivation` | `KernelOnly` | `StartupReplace` | `AssemblyOwnedBaseline` |

Config 与 CLI 本轮完成 baseline 组件化和策略登记，没有生产 replacement。CLI Definition 始终存在，但只在 `AssemblyOptions.CLI` 非 nil 时于 Plan Freeze 后构造 Factory。

## Add 与 Replace

- `app.Add(plan, definition)` 创建新实例身份和独立 Binding。
- `app.Replace(plan, role, replacement)` 改变既有 Role 的主槽位，不返回独立 Binding。
- 相同 Go 接口只表示类型兼容，不产生替换意图。
- `Decorate` 尚未实现；不能用 Add 或 Replace 模拟调用链装饰。

Logger 的三个确定场景：

```text
baseline-only: builtin logger -> root Access -> Kernel / db1
main replace:  logging.main -> root slot -> Kernel / db1
independent:   logging.db2 -> independent Access -> db2 only
```

独立实例失败不会查询或回退 root Access。主槽位 replacement 失败也不会静默当作 baseline-only 成功。

## Plan 与 typed Input

`app.NewPlan -> app.Replace/app.Add -> app.InputOf -> app.Freeze -> Assembly.Install` 是显式流程：

- Component ID 全局唯一；配置化组件使用 `app.Spec{ID, ConfigPath}`；
- ConfigPath 按完整分段检查，相等或父子路径冲突；
- Input 必须来自同一 Plan 的更早 Binding，且不能从较晚阶段反向依赖；
- root Binding 在普通组件之前产生，Logger 是当前唯一 `AppVisible` root 输出；
- 一个 Role 最多一个 replacer，且 Replace 必须早于首个消费者；
- Binding 没有运行期 `Get/Resolve`；`Values` 只在依赖 decoder 调用期间有效；
- Freeze 后不能 Add/Replace；零值 FrozenPlan 不能安装。

低层 `app` 构造器供 builtin 实现与框架 fixture 使用；生产 Plan 的 Role 仍只由 Assembly 从封闭 catalog 登记。

## 生命周期与所有权

普通 `ManagedConfigured` 独立实例使用自己的 Lease。`RuntimeTransaction` replacement 使用 Role 主 slot 的 Lease，因此排空覆盖 Kernel、db1 等全部 root 消费者：

```text
Stage/Build/Ready candidate while current remains visible
-> drain root slot and wait in-flight Use
-> Commit target
-> Resume
-> close previous replacement Resource
```

初始 replacement 的 Build/Ready 期间 baseline 保持可用；发布成功后后续消费者看到 replacement。停止 replacement 时先排空主 slot、恢复 baseline、恢复调用，再关闭 replacement Resource。Runtime 最终停止后，Assembly 按反序关闭 CLI、Logger、Config 的 owned baseline。`BorrowedRuntimeBuiltin` 仅用于明确保留外部关闭权的测试或嵌入场景。

普通组件仍支持 `NoReload`、`KernelInstanceSwap`、`RestartRequired`。候选准备失败会清理候选并保留 current；提交后旧资源关闭失败返回 `CommittedCleanupError`。

## 运行示例

```go
assembly, err := kernel.NewAssembly(kernel.AssemblyOptions{
    Config: builtinconfig.Options{Sources: []config.Source{
        config.FileSource("config.yaml"),
        config.EnvSource("APP_"),
    }},
})
if err != nil {
    return err
}
capabilities, err := composition.Compose(assembly)
if err != nil {
    return err
}
host, err := kernel.NewHost(capabilities.Runtime, kernel.HostOptions{})
if err != nil {
    return err
}
return host.Run(ctx)
```

CLI 模式在 `AssemblyOptions.CLI` 中显式选择；命令执行后必须停止 Runtime，以释放尚未启动的运行节点和 Assembly-owned baseline。`cmd/app` 已执行该清理。

## 当前边界

- Clock、ID Generator、Validator 仍是普通 Direct App，不登记 builtin。
- Database 使用 `Spec + Input[pkglogger.Access]`；生产 `database.db1` 跟随 root Logger。
- 当前没有 HTTP Server、middleware、handler、service、repository、model 等业务层装配。
- 排他资源不能直接套用双实例 Swap；在专用 Handoff 落地前应使用 `RestartRequired`。
