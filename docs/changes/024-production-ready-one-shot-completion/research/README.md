# 024 研究档案

## 阅读顺序

1. [R001 当前一次性竣工基线](R001-current-one-shot-baseline/report.md)：从当前 HEAD、022、并行 023 草案、代码、CI 和本机环境确认真实剩余范围。
2. [R002 API、安全与观测技术栈选型](R002-api-security-stack-selection/report.md)：冻结 spec-first、生成器、兼容门禁、JWT/JWKS、Problem Details 与观测 Adapter。
3. [R003 数据、交付与 release 技术栈选型](R003-delivery-release-stack-selection/report.md)：冻结 Go 版本、migration、容器、SBOM、签名和最终复制验收。
4. [R004 jwx v4 与 jsonv2 生产基线复核](R004-jwx-jsonv2-reassessment/report.md)：记录 v4 强制实验工具链的新事实，并确认 v3.2.0 稳定基线。
5. [R005 认证授权能力的应用模块归属复核](R005-security-module-ownership/report.md)：撤回分散顶层包方案，固定 `internal/module/auth` owner、generation lifecycle 与 Todo 跨模块窄 port。
6. [R006 C5-C6 新增能力归属复核](R006-remaining-module-ownership/report.md)：将 management/observability 合并为 Ops module，并固定 migration engine、command 与 Todo SQL 的不同 owner。
7. [R007 C7 安全基线刷新](R007-security-baseline-refresh/report.md)：记录固定漏洞门禁发现的可达风险，并将 Go/gRPC/x.image 单轨更新到修复版本。

## 复用记录

- [019-R002 成熟 HTTP API 参考模型](../../019-http-api-maturity-gap-assessment/research/R002-http-api-maturity-reference/report.md) 继续提供 HTTP/OpenAPI/Problem Details/OTel/健康与 Go 安全标准语义。
- [020-R002 Go module、模板与版本语义](../../020-scaffold-product-form/research/R002-go-distribution-versioning/report.md) 与 020 ADR-001 继续约束 copy-owned、tag 和人工升级说明。
- [022-R008 Foundation 闭环](../../022-http-api-template-readiness/research/R008-remaining-foundation-closure/report.md) 继续证明当前同步 HTTP/CLI profile 的底层完成状态。

023 当前是未跟踪并行草案，不作为已提交项目 authority；R001 只记录本轮工作区看到的事实及其对 024 单轨迁移的影响。
