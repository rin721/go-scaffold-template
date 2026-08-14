# 011 Cache、I18n 与 Storage 装配

## 状态

- 当前状态：**已完成**。
- 方案建立日期：2026-08-14。
- Git 事实基线：`main@d69233a`，本地分支相对当前 `origin/main` ahead 1。
- 工作树在本任务开始前已有未跟踪目录 `tmp/`；011 不读取其实现作为项目事实，不修改、不暂存、不提交该目录。
- 用户已在方案报告后的后续消息中明确要求开始 011，实施范围固定为 `CACHE-001` 至 `VER-001`。

## 实施结果

仓库已有的 `pkg/cache`、`pkg/i18n` 和 `pkg/storage` 已经按能力边界进入 Kernel 固定清单、`composition.Capabilities`、默认配置和应用启动链。

011 沿当前显式有序 Plan 单轨接入，没有建立第二套容器或运行期查找机制：

- Cache 把共享 Redis 后端与业务泛型 `cache.Client[T]` 分开；Kernel 只拥有后端连接，业务通过受控泛型构造函数取得自己的 typed Client。
- I18n 由 Kernel 根据配置加载资源，向调用方提供身份稳定的 `i18n.Translator` facade。
- Storage 只装配对象存储 Manager；带文件监听、Excel、图片处理和广泛文件系统操作的 `storage.New` 保持调用方局部拥有，不进入进程级 Capabilities。
- Cache 配置变化使用 `RestartRequired`；I18n 与对象 Storage 使用 `KernelInstanceSwap`。
- 默认 Cache 显式禁用，I18n 使用空资源清单，Storage 使用 `.data/storage` 本地后端，从而不把 Redis、S3 或 MinIO 变成本地默认启动前置条件。

## 阅读顺序

1. [requirements.md](requirements.md)：当前事实、目标、范围、约束和验收标准。
2. [design.md](design.md)：公开边界、配置、装配顺序、生命周期、失败语义和文件影响。
3. [tasks.md](tasks.md)：稳定任务 ID、依赖、完成条件、确认门禁与本轮证据。

## 交付边界

本任务已经完成本地实现、测试、文档同步与独立提交；没有启动真实 Redis、S3 或 MinIO，也没有 push。后续若改变公开接口、配置结构、组件边界或重载策略，应建立新的任务级变更记录。
