# 架构说明

本目录是当前架构阅读入口。项目采用显式 composition root，把底层能力、Application Generation、业务模块和进程生命周期连接成单轨运行模型；不依赖包扫描、运行期 Service Locator 或隐式全局状态。

## 架构主线

1. [Kernel 与 App 组件装配](../../internal/kernel/README.md)：底层组件如何通过 Plan、typed Input、Lease、Replacement、生命周期和重载治理进入进程。
2. [Kernel App 组件开发](../../internal/kernel/app/README.md)：跨业务复用且由进程统一选择的底层能力如何声明、构造、输出和验证。
3. [应用模块边界](../../internal/module/README.md)：业务模块如何收口 Model、Service、Adapter、Handler、binding 和 contribution。
4. [应用模块开发指南](../development/application-module-development.md)：新增模块时如何做能力评估、选择模块专属 Adapter 或底层 Capability，并接入 HTTP/config/cli/migration/i18n/schedule/message binding。
5. [pkg 封装规范与能力清单](../../pkg/README.md)：面向业务模块开放的项目自有能力、第三方封装边界和暂缓路线。

## 当前运行结构

- `cmd/app` 只负责进程 I/O、基线日志、参数分支和信号入口。
- `internal/composition` 是唯一应用 composition root，显式装配 Bootstrap CLI、migration one-shot 与长期 Service。
- 长期 Service 通过 Application Generation 管理配置快照、资源复用、listener、HTTP route、定时任务、消息 Consumer 准入、ready 状态和优雅停止。
- 业务模块位于 `internal/module/<name>`，通过 typed contribution 交给 composition 聚合，不扫描、不隐式注册。
- `pkg/<name>` 只暴露项目自有能力契约；第三方具体类型留在 Adapter 或 Kernel App 实现内。

## 当前权威与历史证据

当前架构说明以本页链接的主题文档为准。历史研究和设计过程保存在 [研究档案](../research/README.md) 与 [任务级变更记录](../changes/README.md)，用于解释决策来源和验证证据，不作为并列的当前规范。
