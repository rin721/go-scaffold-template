# 020：脚手架产品形态与升级模型

## 当前状态

- 任务类型：研究、计划与待确认的隔离验证。
- 研究门禁：已通过，证据为 [R001 当前分发边界](research/R001-current-distribution-boundary/report.md) 与 [R002 Go 分发和版本语义](research/R002-go-distribution-versioning/report.md)。
- 计划状态：待确认；本轮只完成文档，没有源码、配置、依赖、生成器或临时消费者变更。
- 代码快照：`main@af7fdadc3ebe9f6c6895a60a9ee0794494c7037e`。
- 来源：[019 的 `FORM-001`](../019-http-api-maturity-gap-assessment/tasks.md)。

## 一句话结论

当前仓库不能继续把“可运行源码仓库”默认等同于“成熟脚手架产品”。完整运行时和装配位于 `internal/**`，外部 Go module 无法复用；直接复制又缺少确定的身份替换、生成文件所有权、版本发布和升级协议。

020 将“版本化的窄公共 Runtime + 生成后由应用拥有的 Composition/Module”作为优先验证假设，但不预先确认它。只有隔离消费者同时证明创建、定制、Runtime 升级和模板演进可控，才通过 ADR 采用组合模式；否则 v0 应单轨退回 generator/template-only，并明确不承诺自动上游合并。

```text
版本化输入                         新服务仓库
Runtime module ────────────────>  go.mod 中可升级的窄依赖
Template schema + generator ───>  应用拥有的 cmd/internal/config/docs
                                      │
                                      └─ 不反向导入源仓库 internal/**
```

## 本变更回答的问题

1. 谁是脚手架消费者，消费者实际获得和拥有哪些文件？
2. Runtime、模板、生成器和示例分别如何版本化？
3. 哪些契约值得成为公共 Go API，哪些必须留在生成后的应用内部？
4. 首次创建、业务定制、Runtime 升级和模板演进如何验证且不覆盖用户代码？
5. 当前仓库应选择 template、generator、library 还是组合形态？

## 阅读顺序

1. [需求与验收标准](requirements.md)
2. [比较、验证设计与决策规则](design.md)
3. [任务、确认范围与证据](tasks.md)
4. [研究档案](research/README.md)

## 当前行为边界

020 尚未改变项目的安装、启动、导入或升级方式。当前行为仍以根 [README](../../../README.md) 为准；任何 `tmp/` 消费者实验、生成命令、源码移动、公共 API、依赖或 ADR 写入，都必须在本计划报告之后获得用户明确确认。
