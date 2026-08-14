# 019：HTTP API 成熟度缺口评估

## 当前状态

- 任务类型：纯文档研究与规划。
- 研究门禁：已通过，证据为 [R001 当前 HTTP API 基线](research/R001-current-http-api-baseline/report.md) 与 [R002 成熟度参考模型](research/R002-http-api-maturity-reference/report.md)。
- 当前状态：评估与路线设计已完成；没有任何非文档实施授权。
- 代码快照：`main@28fbc7a9cfe01e4e7c45505217c15f4d56e711b3`。
- 与 018 的关系：[018 Cordis 启发的插件架构](../018-cordis-inspired-plugin-architecture/README.md) 已按用户决定废除；019 不继承其插件化目标。

## 一句话结论

当前项目已经具备可靠的 Go 进程底座和一个真实 Todo HTTP/CLI 垂直切片，但还不是成熟的 HTTP API 后端脚手架：它缺少统一 API 契约、协议错误模型、身份与授权边界、生产入口策略、管理面、可观测性、版本化数据迁移、交付与升级模型，以及把这些保证自动带入新服务的脚手架产品形态。

主要矛盾不是再增加若干 `pkg` 工具，而是建立以下单轨治理链：

```text
API Contract
  -> generated/verified transport boundary
  -> route policy and application module
  -> runtime HTTP lifecycle
  -> management and observability
  -> migration, delivery and compatibility gates
```

## 缺口优先级

### P0：成熟 HTTP API 的前置架构

1. 脚手架产品形态与升级策略：模板仓库、生成器、库还是组合模式尚未决定。
2. API 单一权威：没有 OpenAPI/operation contract、兼容规则和漂移门禁。
3. 统一协议边界：错误、验证、404/405、分页、版本、幂等与条件请求没有项目级契约。
4. 身份与策略：没有 Principal、认证 Adapter、授权 Policy、审计和可选租户边界。
5. HTTP 入口策略：已有 CORS、BodyLimit、RateLimit 工具，但没有可信代理、请求预算、限流维度和生产配置模型。
6. 管理面与健康：Host 有内存健康快照，但没有 `/livez`、`/readyz`、`/startupz`、依赖检查贡献或受保护诊断入口。
7. 可观测性：只有日志和 request ID，没有 trace/metric、传播、低基数 route、SLO/告警契约。
8. 数据演进：当前 additive startup migration 不能替代版本化迁移、锁、回滚/前滚、backfill 和 expand-contract。

### P1：可复制、可运营与可发布

9. 出站调用治理：`httpx.Client` 和 `resilience` 尚未形成 production capability、策略配置、遥测和幂等重试契约。
10. 测试与质量：缺少 OpenAPI contract、fuzz、性能预算、泄漏、破坏性变更和安全扫描门禁。
11. 交付与运维：缺少镜像、非 root 运行、build metadata、部署样例、SBOM、签名、发布与回滚契约。
12. 开发者体验：没有创建项目/模块的生成流程、重命名规则、升级路径、版本标签和兼容政策。

### P2：必须由真实业务触发

消息、后台任务、调度、分布式锁、邮件、搜索、特性开关、多租户和第三方身份提供方仍需真实用例。019 只要求预留正确边界，不选择产品或制造空实现。

## 阅读顺序

1. [需求与成熟度标准](requirements.md)
2. [目标分层与路线设计](design.md)
3. [任务、优先级与确认边界](tasks.md)
4. [研究档案](research/README.md)

## 当前行为边界

019 是差距评估，不代表上述能力已经实现。当前运行方式仍以根 [README](../../../README.md)、[Kernel 文档](../../../internal/kernel/README.md)、[`httpx` 文档](../../../pkg/httpx/README.md) 与 [应用模块开发指南](../../../docs/development/application-module-development.md) 为准。
