# 024：生产就绪模板一次性竣工

## 当前状态

- 任务类型：研究与非文档实施计划。
- 代码基线：`e251b73518a457ec97c529d067ddfffe77be203a`。
- 研究门禁：C1-C3 已通过并完成；C4-C6 模块归属复核已通过，见 [R005](research/R005-security-module-ownership/report.md) 与 [R006](research/R006-remaining-module-ownership/report.md)。
- 计划状态：**已确认，实施中**。C1-C7 与 C8 Windows 隔离复制证据已完成；Linux 原生 runtime、容器、PostgreSQL/MySQL 和远端 CI 仍未执行。用户于 2026-08-15 确认 R005/R006 修订后的 module-owned 方案，并继续实施 C4-C8；R004 的 `jwx v3.2.0` 选择继续有效。
- 外部授权边界：不允许 push、tag、GitHub Release、GHCR 或外部 attestation。
- Authority：023 的研究与实现目标已由本变更吸收；024 是当前唯一施工 authority。C1-C3 已提交，错误的 C4 未提交骨架已撤回。

## 一句话结论

024 已把 reload、API authority、协议、边缘政策、认证授权、管理面、观测、版本化迁移、可移植性、CI、容器、release 与复制验收合并为一个施工级总计划。实现与 Windows 本地证据已经贯通到两个隔离副本；当前不能竣工的原因是缺少计划明确要求的 Linux、容器和服务器数据库真实环境，而不是继续缺少模块归属或本地实现。

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
- `internal/module/auth` 收口 Principal/Policy/Decision/Audit、JWT/JWKS Adapter、配置、middleware 与 generation contribution；
- `internal/module/ops` 收口 management、startup/live/ready、metrics、build info、脱敏 diagnostics、OpenTelemetry trace 与 Prometheus Adapter；
- `internal/module/migration` 收口命令用例，跨模块 engine 进入 `pkg/database/migrate`，Todo SQL/readiness 留在 Todo；
- Go 1.26.6、Windows/Linux 同义门禁、非 root OCI image、CI 安全门禁；
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
