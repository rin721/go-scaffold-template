# capability

`internal/kernel/capability` 定义需要由 Kernel 托管的底层能力。每个能力负责提供 typed 配置契约、实例构造、就绪检查和资源释放钩子，但不登记 Kernel，也不决定自身是否启用。

## 定义契约

- 每个能力通过无副作用的 `Definition()` 返回完整 `kernel.Definition`。
- Builder 只根据已校验配置创建全新实例，不修改正在服务的旧实例。
- Lifecycle 的 Start 在发布前完成启动或就绪确认，Stop 释放实例拥有的资源。
- 能力不得调用 `kernel.Register`、定义 `init` 注册逻辑、扫描目录或通过反射发现其他能力。

能力不提供原地 Reload。配置变化始终由 Kernel 创建候选实例、统一切换并停止旧实例。固定组合清单位于 `internal/kernel/composition`，只有调用方显式调用 `composition.Compose` 才会登记能力。

## Database Capability

`internal/kernel/capability/database` 是首个真实定义：

- 使用独立 `Config` 解码顶层 `database` 配置段，再转换为 `pkg/database.Config`。
- Builder 调用 `database.New`，Lifecycle Start 调用 `Ping`，Stop 调用 `Close`。
- composition 登记后返回稳定 Database Access；业务构造函数不接收 Kernel Handle。
- `database.Client`、事务、`Rows` 和 `Row` 都不得逃逸 Use 回调。查询结果必须在回调内消费并关闭，否则 Kernel 无法精确判定旧实例已经无人使用。

## 依赖方向

```text
caller -> internal/kernel/composition -> internal/kernel
                                      -> internal/kernel/capability/database -> pkg/database
pkg -----------------------------------------------------------------------X-> internal
```

新增能力时必须保留此方向，由 composition 逐项显式登记；不得为了接入 Kernel 修改 pkg 包的依赖边界或加入全局容器。
