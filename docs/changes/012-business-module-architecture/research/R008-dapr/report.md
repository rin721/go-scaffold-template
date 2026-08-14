# R008：Dapr

## 研究问题

区分进程内模块边界和 Dapr 解决的分布式运行问题，防止因为“模块化”一词过早引入 sidecar。

## 已验证机制

Dapr runtime 通常以应用旁的 sidecar 提供 service invocation、state、pub/sub、bindings、secret 等 building blocks。应用通过本地 HTTP/gRPC API 或语言 SDK 调用；具体中间件由外部 component 配置选择。

官方 Go SDK 同时包含 client 和服务端集成，帮助 Go 应用访问 sidecar。runtime 仓库则承担 sidecar、组件、placement/actors 等更广泛职责。这是部署时的跨进程能力，不是 Go constructor graph。

## 适用价值

- 多服务或跨语言系统需要统一调用、状态、pub/sub 和 Secret 接口。
- 团队愿意运营 sidecar/control plane，并接受其失败模式。
- 组件替换、平台治理和遥测收益大于额外网络跳数与部署复杂度。
- 远程边界已经有明确契约、身份、重试、幂等和一致性要求。

## 当前不适用

当前仓库没有真实分布式服务、消息交付或跨语言模块需求。用 sidecar 替代进程内调用会增加序列化、网络、超时、重试、部署、版本兼容和可观测性责任，而且无法解决 Go 包依赖方向、Handler/Service/Repository 边界或本地 composition。

Dapr component 的外部配置和生命周期还会与当前 Kernel Database/Cache/Config 能力形成重叠，需要完整迁移设计，不能作为可选 Adapter 顺手接入。

## 对 012 的结论

当前跨模块调用使用 caller-owned Go port 和构造注入。Dapr 只作为未来“模块已经成为独立部署服务”时的候选研究对象。那时需要独立 ADR，明确协议、身份、超时、重试、幂等、消息交付、观测和运维 owner。

不要以“以后可能拆微服务”为理由在当前 Service 接口中预置远程 DTO 或网络错误。

## 局限

本报告不做 Dapr 组件兼容矩阵、性能或安全评审；在出现真实分布式需求时必须重新研究当前版本和部署环境。
