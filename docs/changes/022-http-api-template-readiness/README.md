# 022：HTTP API 脚手架成熟就绪度

## 当前状态

- 任务类型：纯文档研究与实现规划。
- 研究门禁：已通过，证据为 [R001 当前就绪度复核](research/R001-current-readiness-reassessment/report.md)，并复用仍有效的 [019-R002 成熟度参考模型](../019-http-api-maturity-gap-assessment/research/R002-http-api-maturity-reference/report.md)。
- 当前状态：成熟度判断、实现目标和分阶段任务计划已完成；没有任何非文档实施授权。
- 代码快照：`main@fa349ab95a66ab641304dd9bfb31993d65cc04a6`。
- 与既有任务的关系：019 是旧身份下的首次差距快照；020 已确定 copy-owned 产品形态；021 已完成 canonical identity 迁移；022 是三者之后的当前判断入口。

## 一句话结论

当前项目**还不能称为成熟的 Go Server HTTP API 后端脚手架模板**。它已经是一份结构清晰、可运行、可复制且生命周期治理较强的后端基础源码基线，但成熟 HTTP API 所需的 API 单一权威、统一协议、安全策略、管理面与遥测、版本化迁移、跨平台交付和 release 验收仍未闭环。

更准确的当前定位是：

> 可用于学习、原型和受控团队内部项目起步的高级基础模板；若要用于公网、多人协作或多实例生产服务，必须先完成本计划的必需门禁，不能把调用方自行补齐的关键保证包装成模板已经成熟。

## 当前就绪度

| 平面 | 当前判断 | 说明 |
| --- | --- | --- |
| 进程、配置与资源生命周期 | 通过 | 显式 composition、严格配置、监督启动、ready、反向停止和资源切换已有实现与测试 |
| 真实业务垂直切片 | 通过 | Todo HTTP/CLI、Database migration、Repository、Service 与进程验收形成闭环 |
| copy-owned 产品形态与 identity | 部分通过 | Windows 隔离复制和 identity 迁移已验证；正式复制指南、Linux 和 release 尚缺 |
| HTTP API 契约与兼容 | 未通过 | Route 只有 method/path/handler/middleware，没有 operation schema、OpenAPI authority 或 breaking diff |
| 协议与边缘策略 | 未通过 | 错误、严格 JSON、404/405、分页、request budget、trusted proxy 与限流政策未统一 |
| 身份、安全与审计 | 未通过 | 无 Principal、认证 Adapter、授权 Policy、对象级授权和审计闭环 |
| 管理面与可观测性 | 未通过 | 内部 health snapshot 不可由编排系统消费；无 management listener、trace 或 metrics |
| 数据演进 | 未通过 | 启动期 additive migration 适合示例，不是多副本 production migration 协议 |
| 交付与供应链 | 未通过 | 无容器、部署样例、vulnerability gate、SBOM、签名、release/rollback；Windows tidy 门禁受行尾影响 |

## 成熟标签的停止线

只有 [requirements.md](requirements.md) 中全部 baseline gate 通过，并在独立复制副本、Windows/Linux CI 和可部署产物上获得验收证据后，项目才可以把自己描述为“成熟 HTTP API 后端脚手架”。认证产品、消息、调度、租户、搜索等场景能力不因常见就预装，但对应边界必须允许真实需求以单轨方式接入。

## 阅读顺序

1. [研究：当前就绪度复核](research/R001-current-readiness-reassessment/report.md)
2. [需求：成熟标签与验收门禁](requirements.md)
3. [设计：目标架构与分阶段路线](design.md)
4. [任务：实现 Program 与确认边界](tasks.md)

## 当前授权边界

022 只新增和更新研究、计划与文档导航。源码、配置、依赖、测试实现、CI、容器、生成物、服务启动、tag、release 和外部写入均未授权；每个实施阶段必须建立独立变更、重新研究并在计划报告后的后续消息中获得确认。
