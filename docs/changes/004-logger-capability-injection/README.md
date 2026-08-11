# 004 Logger Capability 注入

## 状态

用户已于 2026-08-11 明确确认当前 004 方案；实现、测试、权威文档和验证已经完成。当前状态：**已完成**。

当前使用方式以根 [README.md](../../../README.md)、[Kernel 说明](../../../internal/kernel/README.md)、[Capability 说明](../../../internal/kernel/capability/README.md)、[`pkg/logger` 说明](../../../pkg/logger/README.md) 和实际 Go API 为准；本目录保留需求、设计、任务账本和验证证据，不作为第二套现行规范。

## 范围

本任务把现有 `pkg/logger` 纳入 Kernel 显式组合和配置事务，提供业务 Logger Access，并为 Kernel 保留启动前基线 logger。配置化 logger 只在整轮启动或重载事务成功后接管，停止前恢复基线。应用入口增加真实的启动、停止日志消费者，同时保持最终错误只由 stderr 边界输出一次。

本任务同步补齐 logger 文件 sink 的所有权和关闭契约；不新增 HTTP、业务服务、全局 logger、外部采集后端、配置文件 Watch 或第二套生命周期。

## 阅读顺序

1. [requirements.md](requirements.md)：目标、范围、约束和验收标准。
2. [design.md](design.md)：接口、数据流、事务语义、资源释放和文件影响。
3. [tasks.md](tasks.md)：稳定任务 ID、依赖、确认状态和后续实施证据。
