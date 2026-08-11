# adapter

`internal/adapter` 封装可独立使用的 `pkg` 通用库，为 Kernel 提供 typed 配置、实例构造、就绪检查和资源释放实现。Adapter 不登记 Kernel，也不决定哪些能力会被启用。

## 职责边界

- Adapter 根据自身 typed 配置创建全新实例，不修改正在服务的旧实例。
- Adapter 的 Start 在发布前完成启动或就绪确认，Stop 释放实例拥有的资源。
- `Builder`、`Lifecycle` 和租约 `Access` 契约由 `internal/kernel` 定义，具体 Adapter 实现并收敛为能力专用 Access。
- Adapter 禁止调用 `kernel.Register`、定义 `init` 注册逻辑、扫描目录或通过反射发现能力。

Adapter 不提供原地 Reload。配置变化始终由 Kernel 创建候选实例、统一切换并停止旧实例。固定装配清单位于 `internal/kernel/assembly`，只有调用方显式调用 `assembly.Inject` 才会登记能力。

## Database Adapter

`internal/adapter/database` 是首个真实接入：

- 使用独立 DTO 解码顶层 `database` 配置段，再转换为 `pkg/database.Config`。
- `Adapter.Build` 调用 `database.New`，`Start` 调用 `Ping`，`Stop` 调用 `Close`。
- assembly 登记后返回稳定 Database Access；业务构造函数不接收 Kernel Handle。
- `database.Client`、事务、`Rows` 和 `Row` 都不得逃逸 Use 回调。查询结果必须在回调内消费并关闭，否则 kernel 无法精确判定旧实例已经无人使用。

## 依赖方向

```text
caller -> internal/kernel/assembly -> internal/kernel
                                   -> internal/adapter/database -> pkg/database
internal/adapter/database ---------> internal/kernel contracts
pkg ----------------------------------------------------------X-> internal
```

新增 Adapter 时必须保留此方向，由 assembly 逐项显式登记；不得为了接入 Kernel 修改 pkg 包的依赖边界或加入全局容器。
