# 默认配置契约与可选 CLI 能力

## 状态

本设计已于 2026-08-11 完成实施。当前使用方式以根 [README.md](../../../README.md)、[Kernel 说明](../../../internal/kernel/README.md)、[App 组件说明](../../../internal/kernel/app/README.md)、[CLI 说明](../../../pkg/cli/README.md) 和实际 Go API 为准；本目录保留需求、开发设计、任务账本和当时实现证据，不作为第二套现行使用说明。

设计序号为 `001`，主题范围只包含：能力默认配置契约、默认配置文件生成、启动前可选 CLI 组合，以及 `config init` 命令。不包含运行时 CLI 命令、远程配置中心或多实例命名。

## 阅读顺序

1. [requirements.md](requirements.md)：面向产品与使用方，说明问题、用户场景和可验收行为。
2. [design.md](design.md)：面向开发者，冻结接口、组合顺序、失败语义和预期文件改动。
3. [tasks.md](tasks.md)：面向实施 Agent，记录任务依赖、工作量、完成条件和逐轮执行证据。

## 核心术语

- **Capability Definition**：由 `internal/kernel/capability` 提供、由 composition 显式登记的底层能力定义。
- **默认配置契约**：能力自行实现的抽象接口，用于产出该能力拥有的默认配置段。Kernel 不替能力决定字段和值。
- **Binding**：成功登记 Definition 后形成的不可变默认配置绑定，关联 Capability ID、配置路径和契约。
- **配置管理**：显式接收全部 Binding、聚合配置文档并安全写入文件的能力。
- **CLI Contract**：向启动前 CLI 贡献一个或多个项目自有 `cli.CommandSpec` 的可选契约。
- **Abort**：能力契约主动中止整次默认配置生成的控制结果；中止后不产生部分文件。
- **启动前 CLI**：在 `Kernel.Start` 或 `Host.Run` 之前执行的初始化入口，不是 Kernel 候选实例，也不是 Supervisor Task。

## 设计不变量

- `kernel.New` 继续只创建空运行时；没有扫描、反射、`init` 注册或默认能力清单。
- composition 继续按能力拆分文件并显式登记；默认配置和 CLI 绑定不能绕过 composition。
- 每个成功登记的 Definition 必须有默认配置契约；CLI Contract 则是可选的。
- Kernel 只检查契约结构和控制协议，不解析能力的业务默认值。
- 默认配置生成先在内存中完成全部调用、校验和编码，再接触目标文件。
- CLI 未启用时不构造 CLI App、不执行 CLI Contract，普通服务启动路径保持不变。
