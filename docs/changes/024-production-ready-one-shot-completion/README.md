# 024：生产就绪模板一次性竣工

## 当前状态

- 任务类型：研究与非文档实施计划。
- 代码基线：`e251b73518a457ec97c529d067ddfffe77be203a`。
- 研究门禁：已通过，见 [R001](research/R001-current-one-shot-baseline/report.md)、[R002](research/R002-api-security-stack-selection/report.md) 与 [R003](research/R003-delivery-release-stack-selection/report.md)。
- 计划状态：**已确认，实施中**。用户于 2026-08-15 明确确认连续实施 `ONE-001..025`、本地与 CI 验证和检查点提交，并允许临时本地容器与测试数据库。
- 外部授权边界：不允许 push、tag、GitHub Release、GHCR 或外部 attestation。
- Authority：023 的研究与实现目标已由本变更吸收；024 是当前唯一施工 authority。C1 工具链检查点已提交，C2 Application Generation/ListenerHub 正在收口。

## 一句话结论

剩余竣工不再拆成十二个独立 Program。024 将 reload、API authority、协议、边缘政策、认证授权、管理面、观测、版本化迁移、可移植性、CI、容器、release 与复制验收合并为一个施工级总计划；用户在本计划报告后的后续消息中只需确认一次，Agent 随后连续实施、验证和提交，除非命中明确的重新确认触发器。

“一次性”表示一次计划确认、连续施工和一次最终总验收，不表示把全部变更压成一个不可审查的 commit。计划采用按依赖排序的检查点 commit，任何检查点都不得降低最终完成定义，也不得提前宣称竣工。

## 竣工标签

只有下列三项同时通过，024 才允许完成：

1. `Foundation-closed(current production profile)`；
2. `Copy-ready(windows/amd64 + linux/amd64)`；
3. `Production HTTP API-ready`。

最终还必须通过两个隔离复制副本、两平台 runtime、协议与安全负向场景、版本化 migration、容器、SBOM/签名、rollback 和 release artifact 验收。

## 单轨范围

- 完整 watched runtime configuration 的不可变 Application Generation 与 listener handoff；
- spec-first OpenAPI authority、生成的 strict Chi server、统一 operation identity 与 breaking diff；
- RFC 9457 Problem Details、严格请求/响应协议和 edge policy；
- 项目自有 Principal/Policy/Decision/Audit 契约与 JWT/JWKS Adapter；
- 独立 management listener、startup/live/ready、metrics、build info 与脱敏 diagnostics；
- OpenTelemetry trace、Prometheus metrics 与日志关联；
- `golang-migrate` 版本化 SQL、独立 migration command 和启动 schema readiness；
- Go 1.26.5、Windows/Linux 同义门禁、非 root OCI image、CI 安全门禁；
- GoReleaser、Syft、Cosign、checksum、provenance、复制指南和最终验收。

## 非目标

- 不引入运行时 DI 容器、service locator、反射扫描、插件 Runtime 或 generator 产品形态。
- 不预装消息、调度、邮件、搜索、租户、特性开关或分布式锁。
- 不把 WebSocket、SSE、HTTP/3、hijacked connection 或多进程热升级伪装成已验收能力。
- 不替使用者选择云厂商、API gateway、OIDC 提供方、APM 后端或 Kubernetes。
- 不在本计划阶段 push、tag、发布、部署、拉取容器或写入外部系统。

## 阅读顺序

1. [研究索引](research/README.md)
2. [需求与竣工定义](requirements.md)
3. [ADR-003 单轨决策](decision.md)
4. [施工设计](design.md)
5. [任务与验证账本](tasks.md)
6. [022 当前 Foundation 结论](../022-http-api-template-readiness/README.md)

## 已确认授权

本轮施工已经获得上述一次性授权。没有外部发布授权，因此终点是经过验证的本地 `v1.0.0-rc.1` release candidate 与 clean 024 worktree；不得把远端 release、registry image 或外部 provenance 描述为已完成。
