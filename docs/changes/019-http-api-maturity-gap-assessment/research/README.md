# 019 研究档案

## 研究问题

1. 当前项目作为 HTTP API 后端已经真正实现了什么？
2. 哪些能力只有局部工具或历史意向，没有形成可执行的项目设计？
3. 成熟脚手架必须提供哪些默认保证，哪些能力应等待真实业务？
4. 应按什么依赖顺序补齐，才能避免先造中间件、后补协议和运维语义？

## 档案

- [R001 当前 HTTP API 基线](R001-current-http-api-baseline/report.md)：沿 `cmd/app -> internal/composition -> module -> httpx/Host/Database -> CI` 核验真实能力和缺口。
- [R002 HTTP API 成熟度参考](R002-http-api-maturity-reference/report.md)：使用 HTTP RFC、OpenAPI、OpenTelemetry、Kubernetes、OWASP 和 Go 官方安全资料建立最小参考模型。

两份报告共同支撑 [需求](../requirements.md) 与 [设计](../design.md)。研究门禁通过只表示足以交付差距评估，不授权实现任何未来任务。
