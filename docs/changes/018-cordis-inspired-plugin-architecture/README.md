# 018：Cordis 启发的插件架构

> **方案状态：已废除（2026-08-15）。** 用户已明确终止本方案，所有实施任务失效，不得再以本目录作为插件架构实施依据。研究材料仅保留为历史快照；新的当前评估见 [019 HTTP API 成熟度缺口评估](../019-http-api-maturity-gap-assessment/README.md)。

## 当前状态

- 研究门禁：历史上已通过，证据仍保存在 [R001 当前装配事实](research/R001-current-composition-baseline/report.md) 与 [R002 DeepSeek Harness/Cordis](research/R002-deepseek-harness-cordis/report.md)。
- 计划状态：已废除，不再接受确认。
- 当前授权：无；禁止执行本目录中的任何非文档任务。
- 目标：把 `go-scaffold2` 演进为“编译期可验证、运行期可治理”的积木式插件后端框架，而不是复制 TypeScript Proxy、字符串 Service Locator 或 Go 原生动态插件。

## 核心结论

以下内容是废除前的历史结论，不是当前目标设计。

项目已经拥有可复用的半个插件内核：typed `Definition/Binding/Input`、不可变 `FrozenPlan`、候选资源事务、稳定 Lease/facade、反向停止、模块 `Contribution` 和唯一 composition root。当前缺少的是统一插件身份、显式 Catalog、Profile/Bundle、顺序无关的能力依赖图、插件实例级可逆 Effect、图诊断，以及模块与底层组件共享的装配语义。

本方案采用分阶段目标：

1. 先实现编译进二进制的插件 Catalog 与启动期 Profile 编译；
2. 再把现有 Kernel App、Todo、HTTP/CLI surface 单轨迁移为插件；
3. 最后只在真实不停机需求出现后扩展运行期 mount/unmount 和 provider 换代；
4. 第三方不可信插件、进程外协议与 Wasm 不属于本轮。

## 阅读顺序

1. [需求](requirements.md)
2. [设计](design.md)
3. [任务与确认状态](tasks.md)
4. [研究索引](research/README.md)

## 与现行架构的关系

本记录提出目标设计，不代表插件 Runtime 已实现。当前有效行为仍以根 [README](../../../README.md)、[Kernel 文档](../../../internal/kernel/README.md)、[Kernel App 文档](../../../internal/kernel/app/README.md) 与 [应用模块文档](../../../internal/module/README.md) 为准。

012 中“拒绝 runtime DI、扫描、动态插件”的结论在当前实现上仍然有效。本方案若获确认，将通过新的 ADR 精确替代其中“所有装配必须长期固定为手工顺序”的部分；不会恢复扫描、`init` 自注册、公开 Resolver 或 Go `plugin`。
