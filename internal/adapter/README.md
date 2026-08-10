# adapter

`internal/adapter` 把可独立使用的 `pkg` 通用库转换为 kernel 可管理、业务可注入的稳定能力入口。

## 通用契约

- `Builder[C,T]`：只根据已校验配置创建全新实例，不修改旧实例。
- `Lifecycle[T]`：Start 在发布前完成启动或就绪确认，Stop 释放实例拥有的资源。
- `Access[T]`：在 Use 回调期间持有租约；回调返回后 kernel 才能确认本次使用结束。

Adapter 不提供原地 Reload。配置变化始终由 kernel 创建候选实例、统一切换并停止旧实例。

## Database Adapter

`internal/adapter/database` 是首个真实接入：

- 使用独立 DTO 解码顶层 `database` 配置段，再转换为 `pkg/database.Config`。
- Build 调用 `database.New`，Start 调用 `Ping`，Stop 调用 `Close`。
- `Register` 返回稳定 Database Access；业务构造函数不接收 kernel Handle。
- `database.Client`、事务、`Rows` 和 `Row` 都不得逃逸 Use 回调。查询结果必须在回调内消费并关闭，否则 kernel 无法精确判定旧实例已经无人使用。

## 依赖方向

```text
composition root -> internal/adapter -> pkg
                 -> internal/kernel -> internal/adapter contracts
pkg --------------------------------X-> internal
```

新增 Adapter 时必须保留此方向，不得为了接入 kernel 修改 pkg 包的依赖边界或加入全局容器。
