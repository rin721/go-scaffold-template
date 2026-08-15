# 022 研究档案

本目录记录当前 `go-scaffold-template` 的底层能力闭环、成熟 Go 项目对照、业务模块设计解锁条件，以及完整 HTTP API 模板成熟度。

## 记录与阅读顺序

1. [R002 底层能力、装配与生命周期闭环审计](R002-foundation-closure-audit/report.md)：从配置输入追踪到 Stop/cleanup，给出十一门当前判断。
2. [R003 成熟 Go 项目的装配、运行与清理实践对照](R003-go-runtime-practices/report.md)：对照 Go 标准库、Fx、controller-runtime、dskit、Caddy 和 Wire 官方主源。
3. [R004 底层闭环综合结论与业务解锁条件](R004-foundation-closure-synthesis/report.md)：综合保留、补齐、优化、不重设计项和实施顺序。
4. [R005 当前资源终结、重试与强制关闭语义](R005-resource-finalization-policy/report.md)：逐项区分 no-finalization、terminal close、graceful、force、retry 和 release verification，并澄清 CLI 不是运行中资源 owner。
5. [R001 当前 HTTP API 脚手架成熟就绪度复核](R001-current-readiness-reassessment/report.md)：保留整体协议、安全、管理、遥测、迁移和交付差距判断，并由 R002/R004/R005 修正底层状态。

外部 HTTP、OpenAPI、安全、健康、遥测和 Go 安全语义继续复用 [019-R002](../../019-http-api-maturity-gap-assessment/research/R002-http-api-maturity-reference/report.md)。旧底层研究 012-R019/R021 与模块研究 017-R001 仅在刷新条件核验后复用；R002 明确记录哪些旧结论已失效或需当前复核。
